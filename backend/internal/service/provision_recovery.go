package service

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"lxdapi/internal/db"
	"lxdapi/internal/lxc"
	"lxdapi/models"
	"lxdapi/pkg/logger"

	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// 开通状态核对：主控在节点开通等待期间（最长 nodeProvisionWait）重启，实例会停在 creating。
// 启动时与每轮生命周期任务对「不在本进程开通中」的 creating 实例向节点/本机核对：
//   - 节点任务成功 / 容器已存在且超过等待期 → active
//   - 节点任务失败 / 超过宽限期仍查不到容器 → failed，归还库存，已付款订单退款到余额
//   - 节点不可达 → 保持 creating，下轮重试
// ---------------------------------------------------------------------------

// provisionRecoverGrace 超过「等待期 + 宽限」仍无结果才判定失败
var provisionRecoverGrace = 5 * time.Minute

var provisioningInFlight sync.Map // userProductID → struct{}

func markProvisioning(id uint, on bool) {
	if on {
		provisioningInFlight.Store(id, struct{}{})
	} else {
		provisioningInFlight.Delete(id)
	}
}

func isProvisioning(id uint) bool {
	_, ok := provisioningInFlight.Load(id)
	return ok
}

// activateProvision creating → active（条件更新，只有一方生效）
func activateProvision(up *models.UserProduct) bool {
	now := time.Now()
	res := db.DB.Model(&models.UserProduct{}).Where("id = ? AND status = ?", up.ID, "creating").
		Updates(map[string]interface{}{"status": "active", "provisioned_at": now})
	if res.Error != nil || res.RowsAffected == 0 {
		return false
	}
	up.Status = "active"
	up.ProvisionedAt = &now
	return true
}

// failProvision creating → failed，归还库存、已付款订单退款到余额并通知用户（条件更新保证只执行一次）
func failProvision(up *models.UserProduct, reason string) bool {
	res := db.DB.Model(&models.UserProduct{}).Where("id = ? AND status = ?", up.ID, "creating").Update("status", "failed")
	if res.Error != nil || res.RowsAffected == 0 {
		return false
	}
	up.Status = "failed"
	refundNote := ""
	var o models.Order
	if err := db.DB.Where("user_product_id = ? AND type <> ?", up.ID, "renew").Order("id DESC").First(&o).Error; err == nil {
		// 归还预占库存
		if o.StockReserved {
			if r := db.DB.Model(&models.Order{}).Where("id = ? AND stock_reserved = ?", o.ID, true).Update("stock_reserved", false); r.RowsAffected > 0 {
				releaseStock(o.ProductID)
			}
		}
		if amt, err := refundOrderToBalance(&o, fmt.Sprintf("实例 %s 开通失败退款", up.Name)); err != nil {
			logger.Error("订单 %s 开通失败退款失败: %v", o.OrderNo, err)
			refundNote = "，退款处理异常，请联系客服"
		} else if amt > 0 {
			refundNote = fmt.Sprintf("，已退款 ￥%.2f 到账户余额", amt)
		}
	}
	NotifyUser(up.UserID, "product", "实例开通失败", fmt.Sprintf("实例 %s 开通失败：%s%s", up.Name, reason, refundNote), up.ID)
	return true
}

// refundOrderToBalance 已支付订单退款到余额并把订单置为 refunded；0 元订单不处理。返回退款金额。
// 订单状态翻转、余额到账、退款流水写入在同一个 DB 事务里完成：任何一步失败整体回滚，
// 不存在"订单已标退款但余额没到账"的 crash 中间态；以订单状态条件更新 + 退款流水去重保证幂等。
func refundOrderToBalance(o *models.Order, remark string) (float64, error) {
	amt := round2(o.Amount)
	if amt <= 0 {
		return 0, nil // 0 元订单无需退款，保持 paid
	}
	var after float64
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		// 1. 订单 paid → refunded（条件更新：并发下只有一方能推进）
		res := tx.Model(&models.Order{}).Where("id = ? AND status = ?", o.ID, "paid").Update("status", "refunded")
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil // 未支付或已退款：幂等直接返回
		}
		// 2. 退款流水去重（同一订单绝不重复入账）
		var exists int64
		if err := tx.Model(&models.BalanceLog{}).Where("order_no = ? AND type = ?", o.OrderNo, "refund").Count(&exists).Error; err != nil {
			return err
		}
		if exists > 0 {
			return nil
		}
		// 3. 余额到账 + 写流水（同一事务）
		res = tx.Model(&models.User{}).Where("id = ?", o.UserID).UpdateColumn("balance", gorm.Expr("balance + ?", amt))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return fmt.Errorf("用户不存在")
		}
		var u models.User
		if err := tx.Select("id", "balance").First(&u, o.UserID).Error; err != nil {
			return err
		}
		after = round2(u.Balance)
		return tx.Create(&models.BalanceLog{UserID: o.UserID, Amount: amt, Balance: after, Type: "refund", Remark: remark, OrderNo: o.OrderNo}).Error
	})
	if err != nil {
		return 0, err
	}
	if after <= 0 {
		return 0, nil // 幂等未执行（已退款/未支付/已有流水）
	}
	o.Status = "refunded"
	NotifyUser(o.UserID, "order", "余额到账", fmt.Sprintf("账户余额增加 ￥%.2f，当前余额 ￥%.2f。%s", amt, after, remark), 0)
	return amt, nil
}

var recoverMu sync.Mutex

// RecoverStuckProvisioning 核对停在 creating 的实例（启动时与每分钟执行）
func RecoverStuckProvisioning() {
	if !recoverMu.TryLock() {
		return
	}
	defer recoverMu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			logger.Error("开通状态核对异常: %v", r)
		}
	}()
	var ups []models.UserProduct
	db.DB.Where("status = ?", "creating").Find(&ups)
	if len(ups) == 0 {
		return
	}
	round := newLifecycleRound()
	for i := range ups {
		up := &ups[i]
		if isProvisioning(up.ID) {
			continue
		}
		recoverOne(round, up)
	}
}

func recoverOne(round *lifecycleRound, up *models.UserProduct) {
	age := time.Since(up.CreatedAt)
	expired := age > nodeProvisionWait+provisionRecoverGrace

	if up.NodeID == 0 {
		if !lxc.BinaryAvailable() {
			// 本机根本没装 lxc：永久性缺失，直接判失败
			if failProvision(up, "主控本机未安装 LXD") {
				logger.Warn("开通核对：本机实例 %s 无法开通（无 LXD），已标记失败", up.Name)
			}
			return
		}
		if !LocalLXDAvailable() {
			// lxc 命令在但连不上守护进程：可能是短暂抖动，与节点路径一致，等宽限期后再判失败
			if expired {
				if failProvision(up, "主控本机 LXD 不可用") {
					logger.Warn("开通核对：本机实例 %s 无法开通（LXD 不可用），已标记失败", up.Name)
				}
			}
			return
		}
		// lxc info 加超时：socket 异常时进程可能 hang 住，不能让它卡住核对任务（持有 recoverMu 全局锁）
		infoCtx, infoCancel := context.WithTimeout(context.Background(), 30*time.Second)
		infoErr := exec.CommandContext(infoCtx, "lxc", "info", up.Name).Run()
		infoCancel()
		if infoErr == nil {
			if expired || age > nodeProvisionWait {
				if activateProvision(up) {
					logger.OK("开通核对：本机实例 %s 已存在，补齐为 active", up.Name)
				}
			}
			return
		}
		if expired && failProvision(up, "主控在开通过程中重启，实例未创建") {
			logger.Warn("开通核对：本机实例 %s 不存在，已标记失败并退款", up.Name)
		}
		return
	}

	// 节点实例：优先按任务 ID 查询
	var result string // success / failed / pending / unknown
	var failMsg string
	if up.ProvisionTaskID > 0 {
		skipped, err := round.nodeCall(up.NodeID, func(nc *NodeClient) error {
			t, e := nc.GetTask(up.ProvisionTaskID)
			if e != nil {
				return e
			}
			st, _ := t["status"].(string)
			switch st {
			case models.TaskSuccess:
				result = "success"
			case models.TaskFailed:
				result = "failed"
				failMsg, _ = t["error_msg"].(string)
			default:
				result = "pending"
			}
			return nil
		})
		if skipped {
			return
		}
		if err != nil {
			if strings.Contains(err.Error(), "节点连接失败") {
				logger.Warn("开通核对：节点#%d 不可达，实例 %s 下轮重试", up.NodeID, up.Name)
				return
			}
			result = "unknown" // 任务不存在等：退回按容器核对
		}
	}
	switch result {
	case "success":
		if activateProvision(up) {
			logger.OK("开通核对：节点#%d 实例 %s 任务已成功，补齐为 active", up.NodeID, up.Name)
		}
		return
	case "failed":
		if failMsg == "" {
			failMsg = "节点任务失败"
		}
		if failProvision(up, failMsg) {
			logger.Warn("开通核对：节点#%d 实例 %s 任务失败，已标记失败并退款: %s", up.NodeID, up.Name, failMsg)
		}
		return
	case "pending":
		// 节点任务仍在执行：超过 3 倍等待期视为卡死，按容器是否存在判断
		if age < 3*nodeProvisionWait {
			return
		}
	}

	// 无任务 ID / 任务查不到 / 任务长时间未结束：按容器是否存在判断
	var exists, gone bool
	skipped, err := round.nodeCall(up.NodeID, func(nc *NodeClient) error {
		_, e := nc.GetContainer(up.Name)
		return e
	})
	if skipped {
		return
	}
	switch {
	case err == nil:
		exists = true
	case nodeGone(err):
		gone = true
	default:
		logger.Warn("开通核对：查询节点#%d 实例 %s 失败，下轮重试: %v", up.NodeID, up.Name, err)
		return
	}
	if exists && (age > nodeProvisionWait || result == "pending") {
		if activateProvision(up) {
			logger.OK("开通核对：节点#%d 实例 %s 已存在，补齐为 active", up.NodeID, up.Name)
		}
		return
	}
	if gone && expired {
		if failProvision(up, "主控在开通过程中重启，节点上未找到实例") {
			logger.Warn("开通核对：节点#%d 实例 %s 不存在，已标记失败并退款", up.NodeID, up.Name)
		}
	}
}

// RetryPendingActions 重试待补做的节点操作（续费恢复时节点离线未能启动的实例）
func RetryPendingActions() {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("待重试操作异常: %v", r)
		}
	}()
	var ups []models.UserProduct
	db.DB.Where("status = ? AND pending_action = ?", "active", "start").Find(&ups)
	if len(ups) == 0 {
		return
	}
	round := newLifecycleRound()
	for i := range ups {
		up := &ups[i]
		var err error
		if up.NodeID == 0 {
			err = resumeInstance(up)
		} else {
			skipped, e := round.nodeCall(up.NodeID, func(nc *NodeClient) error {
				ae := nc.ContainerAction(up.Name, "start")
				if ae != nil && alreadyRunning(ae) {
					return nil
				}
				return ae
			})
			if skipped {
				continue
			}
			err = e
		}
		if err != nil && !nodeGone(err) {
			logger.Warn("续费恢复重试失败（节点#%d %s），下轮继续: %v", up.NodeID, up.Name, err)
			continue
		}
		if err != nil {
			logger.Warn("续费恢复重试：节点#%d 上实例 %s 不存在，停止重试", up.NodeID, up.Name)
		} else {
			logger.OK("续费恢复重试成功：节点#%d 实例 %s 已启动", up.NodeID, up.Name)
		}
		db.DB.Model(&models.UserProduct{}).Where("id = ? AND pending_action = ?", up.ID, "start").Update("pending_action", "")
	}
}
