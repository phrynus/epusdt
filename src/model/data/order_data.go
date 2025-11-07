package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/assimon/luuu/model/dao"
	"github.com/assimon/luuu/model/mdb"
	"github.com/assimon/luuu/model/request"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

var (
	CacheWalletAddressWithAmountToTradeIdKey = "wallet:%s_%s_%s" // 钱包_待支付金额_链类型 : 交易号
)

// normalizeAmount 规范化金额，统一保留4位小数，避免12.31和12.3100不匹配的问题
func normalizeAmount(amount float64) string {
	return decimal.NewFromFloat(amount).StringFixed(4)
}

// GetOrderInfoByOrderId 通过客户订单号查询订单
func GetOrderInfoByOrderId(orderId string) (*mdb.Orders, error) {
	order := new(mdb.Orders)
	err := dao.Mdb.Model(order).Limit(1).Find(order, "order_id = ?", orderId).Error
	return order, err
}

// GetOrderInfoByTradeId 通过交易号查询订单
func GetOrderInfoByTradeId(tradeId string) (*mdb.Orders, error) {
	order := new(mdb.Orders)
	err := dao.Mdb.Model(order).Limit(1).Find(order, "trade_id = ?", tradeId).Error
	return order, err
}

// CreateOrderWithTransaction 事务创建订单
func CreateOrderWithTransaction(tx *gorm.DB, order *mdb.Orders) error {
	err := tx.Model(order).Create(order).Error
	return err
}

// GetOrderByBlockIdWithTransaction 通过区块获取订单
func GetOrderByBlockIdWithTransaction(tx *gorm.DB, blockId string) (*mdb.Orders, error) {
	order := new(mdb.Orders)
	err := tx.Model(order).Limit(1).Find(order, "block_transaction_id = ?", blockId).Error
	return order, err
}

// OrderSuccessWithTransaction 事务支付成功
func OrderSuccessWithTransaction(tx *gorm.DB, req *request.OrderProcessingRequest) error {
	err := tx.Model(&mdb.Orders{}).Where("trade_id = ?", req.TradeId).Updates(map[string]interface{}{
		"block_transaction_id": req.BlockTransactionId,
		"status":               mdb.StatusPaySuccess,
		"callback_confirm":     mdb.CallBackConfirmNo,
	}).Error
	return err
}

// GetPendingCallbackOrders 查询出等待回调的订单
func GetPendingCallbackOrders() ([]mdb.Orders, error) {
	var orders []mdb.Orders
	err := dao.Mdb.Model(orders).
		Where("callback_num < ?", 5).
		Where("callback_confirm = ?", mdb.CallBackConfirmNo).
		Where("status = ?", mdb.StatusPaySuccess).
		Find(&orders).Error
	return orders, err
}

// SaveCallBackOrdersResp 保存订单回调结果
func SaveCallBackOrdersResp(order *mdb.Orders) error {
	err := dao.Mdb.Model(order).Where("id = ?", order.ID).Updates(map[string]interface{}{
		"callback_num":     gorm.Expr("callback_num + ?", 1),
		"callback_confirm": order.CallBackConfirm,
	}).Error
	return err
}

// UpdateOrderIsExpirationById 通过id设置订单过期
func UpdateOrderIsExpirationById(id uint64) error {
	err := dao.Mdb.Model(mdb.Orders{}).Where("id = ?", id).Update("status", mdb.StatusExpired).Error
	return err
}

// DeleteOrderById 通过id删除订单
func DeleteOrderById(id uint64) error {
	err := dao.Mdb.Where("id = ?", id).Delete(&mdb.Orders{}).Error
	return err
}

// GetTradeIdByWalletAddressAndAmountAndChainType 通过钱包地址、支付金额、链类型获取交易号
func GetTradeIdByWalletAddressAndAmountAndChainType(token string, amount float64, chainType string) (string, error) {
	ctx := context.Background()
	normalizedAmount := normalizeAmount(amount)
	cacheKey := fmt.Sprintf(CacheWalletAddressWithAmountToTradeIdKey, token, normalizedAmount, chainType)
	result, err := dao.CacheGet(ctx, cacheKey)
	if errors.Is(err, dao.ErrCacheNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return result, nil
}

// LockTransactionWithChainType 锁定交易（支持链类型）
func LockTransactionWithChainType(token, tradeId string, amount float64, chainType string, expirationTime time.Duration) error {
	ctx := context.Background()
	normalizedAmount := normalizeAmount(amount)
	cacheKey := fmt.Sprintf(CacheWalletAddressWithAmountToTradeIdKey, token, normalizedAmount, chainType)
	err := dao.CacheSet(ctx, cacheKey, tradeId, expirationTime)
	return err
}

// UnLockTransactionWithChainType 解锁交易（支持链类型）
func UnLockTransactionWithChainType(token string, amount float64, chainType string) error {
	ctx := context.Background()
	normalizedAmount := normalizeAmount(amount)
	cacheKey := fmt.Sprintf(CacheWalletAddressWithAmountToTradeIdKey, token, normalizedAmount, chainType)
	err := dao.CacheDel(ctx, cacheKey)
	return err
}

// HasPendingOrderByAddress 检查指定地址是否有待支付订单（通过缓存检查）
func HasPendingOrderByAddress(token string, chainType string) (bool, error) {
	ctx := context.Background()
	// 缓存 key 格式: wallet:地址_金额_链类型
	// 我们需要查询缓存表中是否有该地址的待支付订单
	cacheKeyPrefix := fmt.Sprintf("wallet:%s_", token)

	// 使用数据库查询缓存表，检查是否存在以该前缀开头且包含链类型的key
	var count int64
	query := `SELECT COUNT(*) FROM cache 
			  WHERE cache_key LIKE ? 
			  AND cache_key LIKE ?
			  AND (expires_at IS NULL OR expires_at > NOW())`
	err := dao.Mdb.WithContext(ctx).Raw(query, cacheKeyPrefix+"%", "%_"+chainType).Row().Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}
