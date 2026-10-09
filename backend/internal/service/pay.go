package service

import (
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"lxdapi/internal/db"
	"lxdapi/models"

	"gorm.io/gorm"
)

// GetPayConfig 获取易支付配置（单例 ID=1）
func GetPayConfig() *models.PayConfig {
	var cfg models.PayConfig
	if err := db.DB.First(&cfg, 1).Error; err != nil {
		cfg = models.PayConfig{GatewayURL: "https://pay.example.com", PID: "", Key: ""}
		cfg.ID = 1
		if err := db.DB.Create(&cfg).Error; err != nil {
			cfg.Enabled = false
		}
	}
	return &cfg
}

// PayInput 易支付配置输入
type PayInput struct {
	GatewayURL string `json:"gateway_url"`
	PID        string `json:"pid"`
	Key        string `json:"key"`
	NotifyURL  string `json:"notify_url"`
	ReturnURL  string `json:"return_url"`
	Enabled    bool   `json:"enabled"`
}

// SavePayConfig 保存易支付配置
func SavePayConfig(in *PayInput) error {
	var existing models.PayConfig
	if err := db.DB.First(&existing, 1).Error; err != nil {
		cfg := models.PayConfig{Model: gorm.Model{ID: 1}, GatewayURL: in.GatewayURL, PID: in.PID, Key: in.Key,
			NotifyURL: in.NotifyURL, ReturnURL: in.ReturnURL, Enabled: in.Enabled}
		return db.DB.Create(&cfg).Error
	}
	return db.DB.Model(&existing).Updates(map[string]interface{}{
		"gateway_url": in.GatewayURL,
		"p_id":        in.PID,
		"key":         in.Key,
		"notify_url":  in.NotifyURL,
		"return_url":  in.ReturnURL,
		"enabled":     in.Enabled,
	}).Error
}

// epaySign 彩虹易支付标准签名：去除 sign/sign_type 与空值，参数按 ASCII 排序，
// 拼接 k=v&k=v 后直接追加商户密钥，MD5 小写。
func epaySign(params map[string]string, key string) string {
	filtered := make(map[string]string)
	for k, v := range params {
		if k == "sign" || k == "sign_type" || v == "" {
			continue
		}
		filtered[k] = v
	}
	keys := make([]string, 0, len(filtered))
	for k := range filtered {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteString("&")
		}
		sb.WriteString(k + "=" + filtered[k])
	}
	sb.WriteString(key)
	sum := md5.Sum([]byte(sb.String()))
	return hex.EncodeToString(sum[:])
}

// BuildPayParams 构造易支付下单参数与跳转地址
func BuildPayParams(orderNo, name, money, payType string) (string, map[string]string, error) {
	cfg := GetPayConfig()
	if !cfg.Enabled || cfg.GatewayURL == "" || cfg.PID == "" || cfg.Key == "" {
		return "", nil, fmt.Errorf("易支付未启用，请联系管理员配置")
	}
	notifyURL := cfg.NotifyURL
	if notifyURL == "" {
		notifyURL = "/api/public/pay/notify"
	}
	returnURL := cfg.ReturnURL
	if returnURL == "" {
		returnURL = "/api/public/pay/return"
	}
	if payType == "" {
		payType = "alipay"
	}
	params := map[string]string{
		"pid":         cfg.PID,
		"type":        payType,
		"out_trade_no": orderNo,
		"notify_url":  notifyURL,
		"return_url":  returnURL,
		"name":        name,
		"money":       money,
	}
	sign := epaySign(params, cfg.Key)
	params["sign"] = sign
	params["sign_type"] = "MD5"

	gateway := strings.TrimRight(cfg.GatewayURL, "/") + "/submit.php"
	// 参数值需 URL 编码（商品名含中文/空格/& 时原实现会生成错误的跳转地址）
	qs := url.Values{}
	for k, v := range params {
		qs.Set(k, v)
	}
	return gateway + "?" + qs.Encode(), params, nil
}

// VerifyPayCallback 验签易支付回调参数；成功返回订单号
func VerifyPayCallback(raw map[string]string) (string, error) {
	cfg := GetPayConfig()
	if cfg.Key == "" {
		return "", fmt.Errorf("支付配置未初始化")
	}
	orderNo := raw["out_trade_no"]
	if orderNo == "" {
		return "", fmt.Errorf("缺少订单号")
	}
	sign := raw["sign"]
	if sign == "" {
		return "", fmt.Errorf("缺少签名")
	}
	if raw["trade_status"] != "TRADE_SUCCESS" {
		return "", fmt.Errorf("支付状态非成功")
	}
	expected := epaySign(raw, cfg.Key)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(strings.ToLower(sign))) != 1 {
		return "", fmt.Errorf("签名校验失败")
	}
	// 商户号必须一致
	if cfg.PID != "" && raw["pid"] != "" && raw["pid"] != cfg.PID {
		return "", fmt.Errorf("商户号不匹配")
	}
	// 回调金额必须与订单金额一致，防止低价支付冒充高价订单
	o, err := GetOrderByNo(orderNo)
	if err != nil {
		return "", err
	}
	money, err := strconv.ParseFloat(strings.TrimSpace(raw["money"]), 64)
	if err != nil || math.Abs(money-o.Amount) > 0.005 {
		return "", fmt.Errorf("支付金额与订单金额不一致")
	}
	return orderNo, nil
}
