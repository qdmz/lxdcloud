package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"lxdapi/internal/db"
	"lxdapi/models"
)

const (
	TokenActivate = "activate"
	TokenReset    = "reset"
	tokenTTL      = 24 * time.Hour
)

// HashPassword 对密码做 bcrypt 哈希
func HashPassword(pwd string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// VerifyPassword 校验用户密码：新用户走 bcrypt；老用户（PasswordHash 为空）兼容明文 APIKey
func VerifyPassword(user *models.User, pwd string) bool {
	if user == nil {
		return false
	}
	if user.PasswordHash != "" {
		return bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(pwd)) == nil
	}
	return user.APIKey == pwd
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// RegisterUser 注册新用户（邮箱唯一；状态 pending，需邮件激活）
func RegisterUser(username, email, password string) (*models.User, error) {
	if username == "" || email == "" || password == "" {
		return nil, fmt.Errorf("用户名、邮箱和密码不能为空")
	}
	if len(password) < 6 {
		return nil, fmt.Errorf("密码长度至少 6 位")
	}

	var cnt int64
	db.DB.Model(&models.User{}).Where("username = ?", username).Count(&cnt)
	if cnt > 0 {
		return nil, fmt.Errorf("用户名已存在")
	}
	db.DB.Model(&models.User{}).Where("email = ?", email).Count(&cnt)
	if cnt > 0 {
		return nil, fmt.Errorf("邮箱已被注册")
	}

	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	apiKey, err := randomToken(16)
	if err != nil {
		return nil, err
	}

	user := &models.User{
		Username:     username,
		Email:        email,
		PasswordHash: hash,
		APIKey:       apiKey,
		Status:       "pending",
		Nickname:     username,
	}
	if err := db.DB.Create(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

// GetUserByEmail 按邮箱查找用户
func GetUserByEmail(email string) (*models.User, error) {
	var user models.User
	if err := db.DB.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, fmt.Errorf("该邮箱未注册")
	}
	return &user, nil
}

// CreateEmailToken 生成邮件令牌
func CreateEmailToken(userID uint, typ string) (*models.EmailToken, error) {
	token, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	et := &models.EmailToken{
		UserID:   userID,
		Token:    token,
		Type:     typ,
		ExpireAt: time.Now().Add(tokenTTL),
	}
	if err := db.DB.Create(et).Error; err != nil {
		return nil, err
	}
	return et, nil
}

// ValidateEmailToken 校验并消费令牌
func ValidateEmailToken(token, typ string) (*models.EmailToken, error) {
	var et models.EmailToken
	if err := db.DB.Where("token = ? AND type = ?", token, typ).First(&et).Error; err != nil {
		return nil, fmt.Errorf("链接无效或不存在")
	}
	if et.UsedAt != nil {
		return nil, fmt.Errorf("链接已被使用")
	}
	if time.Now().After(et.ExpireAt) {
		return nil, fmt.Errorf("链接已过期，请重新申请")
	}
	now := time.Now()
	et.UsedAt = &now
	db.DB.Save(&et)
	return &et, nil
}

// ActivateUser 激活用户
func ActivateUser(token string) error {
	et, err := ValidateEmailToken(token, TokenActivate)
	if err != nil {
		return err
	}
	var user models.User
	if err := db.DB.First(&user, et.UserID).Error; err != nil {
		return err
	}
	if user.Status == "active" {
		return fmt.Errorf("账户已是激活状态")
	}
	now := time.Now()
	user.Status = "active"
	user.EmailVerified = true
	user.ActivatedAt = &now
	return db.DB.Save(&user).Error
}

// ResetPasswordByToken 通过令牌重置密码
func ResetPasswordByToken(token, newPassword string) error {
	if len(newPassword) < 6 {
		return fmt.Errorf("密码长度至少 6 位")
	}
	et, err := ValidateEmailToken(token, TokenReset)
	if err != nil {
		return err
	}
	var user models.User
	if err := db.DB.First(&user, et.UserID).Error; err != nil {
		return err
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	user.PasswordHash = hash
	return db.DB.Save(&user).Error
}

// ActivateDirect 邮件不可用时直接激活（演示/内网部署）
func ActivateDirect(userID uint) error {
	var user models.User
	if err := db.DB.First(&user, userID).Error; err != nil {
		return err
	}
	now := time.Now()
	user.Status = "active"
	user.EmailVerified = true
	user.ActivatedAt = &now
	return db.DB.Save(&user).Error
}

// TouchLogin 记录最近登录时间
func TouchLogin(user *models.User) {
	now := time.Now()
	user.LastLoginAt = &now
	db.DB.Model(user).Update("last_login_at", now)
}

// ChangePassword 修改密码（需校验旧密码）
func ChangePassword(userID uint, oldPwd, newPwd string) error {
	if len(newPwd) < 6 {
		return fmt.Errorf("新密码长度至少 6 位")
	}
	var user models.User
	if err := db.DB.First(&user, userID).Error; err != nil {
		return fmt.Errorf("用户不存在")
	}
	if !VerifyPassword(&user, oldPwd) {
		return fmt.Errorf("原密码错误")
	}
	hash, err := HashPassword(newPwd)
	if err != nil {
		return err
	}
	user.PasswordHash = hash
	return db.DB.Save(&user).Error
}

// UpdateProfile 更新用户资料（昵称等）
func UpdateProfile(userID uint, nickname, email string) error {
	var user models.User
	if err := db.DB.First(&user, userID).Error; err != nil {
		return fmt.Errorf("用户不存在")
	}
	if email != "" && email != user.Email {
		var cnt int64
		db.DB.Model(&models.User{}).Where("email = ? AND id != ?", email, user.ID).Count(&cnt)
		if cnt > 0 {
			return fmt.Errorf("邮箱已被其他账户使用")
		}
		user.Email = email
	}
	if nickname != "" {
		user.Nickname = nickname
	}
	return db.DB.Save(&user).Error
}
