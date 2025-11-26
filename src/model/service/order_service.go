package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/assimon/luuu/config"
	"github.com/assimon/luuu/model/dao"
	"github.com/assimon/luuu/model/data"
	"github.com/assimon/luuu/model/mdb"
	"github.com/assimon/luuu/model/request"
	"github.com/assimon/luuu/model/response"
	"github.com/assimon/luuu/mq/handle"
	"github.com/assimon/luuu/util/constant"
	"github.com/assimon/luuu/util/math"
	"github.com/golang-module/carbon/v2"
	"github.com/shopspring/decimal"
)

const (
	CnyMinimumPaymentAmount  = 0.01   // 人民币最低支付金额
	UsdtMinimumPaymentAmount = 0.0001 // USDT最低支付金额
	UsdtAmountPerIncrement   = 0.0001 // USDT每次递增金额
	IncrementalMaximumNumber = 1000   // 最大递增次数
)

var gCreateTransactionLock sync.Mutex

// CreateTransaction 创建订单
func CreateTransaction(req *request.CreateTransactionRequest) (*response.CreateTransactionResponse, error) {
	gCreateTransactionLock.Lock()
	defer gCreateTransactionLock.Unlock()
	payAmount := math.MustParsePrecFloat64(req.Amount, 2)
	// 按照汇率转化USDT
	decimalPayAmount := decimal.NewFromFloat(payAmount)
	decimalRate := decimal.NewFromFloat(config.GetUsdtRate())
	decimalUsdt := decimalPayAmount.Div(decimalRate)
	// 人民币是否可以满足最低支付金额
	if decimalPayAmount.Cmp(decimal.NewFromFloat(CnyMinimumPaymentAmount)) == -1 {
		return nil, constant.PayAmountErr
	}
	// USDT是否可以满足最低支付金额
	if decimalUsdt.Cmp(decimal.NewFromFloat(UsdtMinimumPaymentAmount)) == -1 {
		return nil, constant.PayAmountErr
	}
	// 已经存在了的交易
	exist, err := data.GetOrderInfoByOrderId(req.OrderId)
	if err != nil {
		return nil, err
	}
	if exist.ID > 0 {
		return nil, constant.OrderAlreadyExists
	}
	// 确定链类型，默认为TRC20
	chainType := req.ChainType
	if chainType == "" {
		chainType = mdb.ChainTypeTRC20
	}
	// 验证链类型是否有效
	validChainTypes := map[string]bool{
		mdb.ChainTypeTRC20:   true,
		mdb.ChainTypeERC20:   true,
		mdb.ChainTypeBEP20:   true,
		mdb.ChainTypeSOLANA:  true,
		mdb.ChainTypePOLYGON: true,
		mdb.ChainTypeARB:     true,
	}
	if !validChainTypes[chainType] {
		chainType = mdb.ChainTypeTRC20 // 无效时使用默认值
	}

	// 检查是否有可用钱包，根据链类型
	walletAddress, err := data.GetAvailableWalletAddressByChainType(chainType)
	if err != nil {
		return nil, err
	}
	if len(walletAddress) <= 0 {
		return nil, constant.NotAvailableWalletAddress
	}
	// 金额保留4位小数，与缓存key的规范化保持一致，避免12.31和12.3100不匹配
	amount := math.MustParsePrecFloat64(decimalUsdt.InexactFloat64(), 4)
	availableToken, availableAmount, err := CalculateAvailableWalletAndAmount(amount, walletAddress, chainType)
	if err != nil {
		return nil, err
	}
	if availableToken == "" {
		return nil, constant.NotAvailableAmountErr
	}
	tx := dao.Mdb.Begin()
	order := &mdb.Orders{
		TradeId:      GenerateCode(),
		OrderId:      req.OrderId,
		Amount:       req.Amount,
		ActualAmount: availableAmount,
		Token:        availableToken,
		ChainType:    chainType,
		Status:       mdb.StatusWaitPay,
		NotifyUrl:    req.NotifyUrl,
		RedirectUrl:  req.RedirectUrl,
	}
	err = data.CreateOrderWithTransaction(tx, order)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	// 先提交事务
	err = tx.Commit().Error
	if err != nil {
		return nil, err
	}

	// 提交事务后再锁定支付池，避免SQLite写锁冲突
	err = data.LockTransactionWithChainType(availableToken, order.TradeId, availableAmount, chainType, config.GetOrderExpirationTimeDuration())
	if err != nil {
		// 如果缓存失败，需要回滚订单，删除已创建的订单
		data.DeleteOrderById(order.ID)
		return nil, err
	}
	// 超时过期消息队列
	ctx := context.Background()
	dao.EnqueueTaskDelay(ctx, "default", handle.QueueOrderExpiration, order.TradeId, config.GetOrderExpirationTimeDuration(), 3)
	ExpirationTime := carbon.Now().AddMinutes(config.GetOrderExpirationTime()).Timestamp()
	resp := &response.CreateTransactionResponse{
		TradeId:        order.TradeId,
		OrderId:        order.OrderId,
		Amount:         order.Amount,
		ActualAmount:   order.ActualAmount,
		Token:          order.Token,
		ChainType:      order.ChainType,
		ExpirationTime: ExpirationTime,
		PaymentUrl:     fmt.Sprintf("%s/pay/checkout-counter/%s", config.GetAppUri(), order.TradeId),
	}
	return resp, nil
}

// OrderProcessing 成功处理订单
func OrderProcessing(req *request.OrderProcessingRequest) error {
	tx := dao.Mdb.Begin()
	exist, err := data.GetOrderByBlockIdWithTransaction(tx, req.BlockTransactionId)
	if err != nil {
		return err
	}
	if exist.ID > 0 {
		tx.Rollback()
		return constant.OrderBlockAlreadyProcess
	}

	// 获取订单信息以获得链类型
	order, err := data.GetOrderInfoByTradeId(req.TradeId)
	if err != nil {
		tx.Rollback()
		return err
	}

	// 标记订单成功
	err = data.OrderSuccessWithTransaction(tx, req)
	if err != nil {
		tx.Rollback()
		return err
	}

	// 先提交事务
	err = tx.Commit().Error
	if err != nil {
		return err
	}

	// 提交事务后再解锁交易，避免SQLite写锁冲突
	err = data.UnLockTransactionWithChainType(req.Token, req.Amount, order.ChainType)
	if err != nil {
		// 缓存解锁失败不影响订单处理结果，只记录错误
		// 缓存会自动过期
		return nil
	}

	return nil
}

// CalculateAvailableWalletAndAmount 计算可用钱包地址和金额
func CalculateAvailableWalletAndAmount(amount float64, walletAddress []mdb.WalletAddress, chainType string) (string, float64, error) {
	availableToken := ""
	availableAmount := amount
	calculateAvailableWalletFunc := func(amount float64) (string, error) {
		availableWallet := ""
		for _, address := range walletAddress {
			token := address.Token
			result, err := data.GetTradeIdByWalletAddressAndAmountAndChainType(token, amount, chainType)
			if err != nil {
				return "", err
			}
			if result == "" {
				availableWallet = token
				break
			}
		}
		return availableWallet, nil
	}
	for i := 0; i < IncrementalMaximumNumber; i++ {
		token, err := calculateAvailableWalletFunc(availableAmount)
		if err != nil {
			return "", 0, err
		}
		// 拿不到可用钱包就累加金额
		if token == "" {
			decimalOldAmount := decimal.NewFromFloat(availableAmount)
			decimalIncr := decimal.NewFromFloat(UsdtAmountPerIncrement)
			// 确保累加后的金额也保持4位小数精度
			availableAmount = math.MustParsePrecFloat64(decimalOldAmount.Add(decimalIncr).InexactFloat64(), 4)
			continue
		}
		availableToken = token
		break
	}
	// 确保最终返回的金额保持4位小数精度
	availableAmount = math.MustParsePrecFloat64(availableAmount, 4)
	return availableToken, availableAmount, nil
}

// GenerateCode 订单号生成
func GenerateCode() string {
	date := time.Now().Format("20060102")
	r := rand.Intn(1000)
	code := fmt.Sprintf("%s%d%03d", date, time.Now().UnixNano()/1e6, r)
	return code
}

// GetOrderInfoByTradeId 通过交易号获取订单
func GetOrderInfoByTradeId(tradeId string) (*mdb.Orders, error) {
	order, err := data.GetOrderInfoByTradeId(tradeId)
	if err != nil {
		return nil, err
	}
	if order.ID <= 0 {
		return nil, constant.OrderNotExists
	}
	// 实时检查订单是否已过期
	err = CheckAndUpdateOrderExpiration(order)
	if err != nil {
		return nil, err
	}
	return order, nil
}

// CheckAndUpdateOrderExpiration 检查并更新订单过期状态
func CheckAndUpdateOrderExpiration(order *mdb.Orders) error {
	// 只处理等待支付的订单
	if order.Status != mdb.StatusWaitPay {
		return nil
	}

	// 计算订单过期时间
	expirationTime := order.CreatedAt.AddMinutes(config.GetOrderExpirationTime())
	currentTime := carbon.Now()

	// 如果当前时间已超过过期时间，立即更新订单状态
	if currentTime.Gt(expirationTime) {
		// 更新数据库状态为已过期
		err := data.UpdateOrderIsExpirationById(order.ID)
		if err != nil {
			return err
		}

		// 解锁交易缓存
		err = data.UnLockTransactionWithChainType(order.Token, order.ActualAmount, order.ChainType)
		if err != nil {
			// 缓存解锁失败不影响订单过期状态，缓存会自动过期
		}

		// 更新内存中的订单状态
		order.Status = mdb.StatusExpired

		// 如果订单设置了回调地址，发送过期通知
		if order.NotifyUrl != "" {
			ctx := context.Background()
			payloadBytes, err := json.Marshal(order)
			if err == nil {
				// 将订单过期回调加入队列
				dao.EnqueueTaskNow(ctx, "default", handle.QueueOrderExpirationCallback, string(payloadBytes), 3)
			}
		}
	}

	return nil
}
