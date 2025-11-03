package handle

import (
	"context"
	"errors"

	"github.com/assimon/luuu/config"
	"github.com/assimon/luuu/model/dao"
	"github.com/assimon/luuu/model/data"
	"github.com/assimon/luuu/model/mdb"
	"github.com/assimon/luuu/model/response"
	"github.com/assimon/luuu/util/http_client"
	"github.com/assimon/luuu/util/json"
	"github.com/assimon/luuu/util/log"
	"github.com/assimon/luuu/util/sign"
)

const (
	QueueOrderExpiration         = "order:expiration"
	QueueOrderExpirationCallback = "order:expiration:callback"
)

// OrderExpirationHandle 设置订单过期
func OrderExpirationHandle(ctx context.Context, payload []byte) error {
	tradeId := string(payload)
	orderInfo, err := data.GetOrderInfoByTradeId(tradeId)
	if err != nil {
		return err
	}
	if orderInfo.ID <= 0 || orderInfo.Status != mdb.StatusWaitPay {
		return nil
	}
	err = data.UpdateOrderIsExpirationById(orderInfo.ID)
	if err != nil {
		return err
	}
	// 使用链类型解锁交易，确保不同链类型的订单能正确解锁
	err = data.UnLockTransactionWithChainType(orderInfo.Token, orderInfo.ActualAmount, orderInfo.ChainType)
	if err != nil {
		return err
	}

	// 如果订单设置了回调地址，发送过期通知
	if orderInfo.NotifyUrl != "" {
		orderInfo.Status = mdb.StatusExpired
		payloadBytes, err := json.Cjson.Marshal(orderInfo)
		if err == nil {
			// 将订单过期回调加入队列
			dao.EnqueueTaskNow(ctx, "default", QueueOrderExpirationCallback, string(payloadBytes), 3)
		}
	}

	return nil
}

// OrderExpirationCallbackHandle 订单过期回调通知
func OrderExpirationCallbackHandle(ctx context.Context, payload []byte) error {
	var order mdb.Orders
	err := json.Cjson.Unmarshal(payload, &order)
	if err != nil {
		return err
	}

	defer func() {
		if err := recover(); err != nil {
			log.Sugar.Error(err)
		}
	}()

	defer func() {
		data.SaveCallBackOrdersResp(&order)
	}()

	client := http_client.GetHttpClient()
	orderResp := response.OrderNotifyResponse{
		TradeId:            order.TradeId,
		OrderId:            order.OrderId,
		Amount:             order.Amount,
		ActualAmount:       order.ActualAmount,
		Token:              order.Token,
		ChainType:          order.ChainType,
		BlockTransactionId: order.BlockTransactionId,
		Status:             mdb.StatusExpired, // 订单过期状态
	}

	signature, err := sign.Get(orderResp, config.GetApiAuthToken())
	if err != nil {
		return err
	}
	orderResp.Signature = signature

	resp, err := client.R().
		SetHeader("powered-by", "Epusdt(https://github.com/assimon/epusdt)").
		SetBody(orderResp).
		Post(order.NotifyUrl)
	if err != nil {
		return err
	}

	body := string(resp.Body())
	if body != "ok" && body != "success" {
		order.CallBackConfirm = mdb.CallBackConfirmNo
		return errors.New("回调响应不正确")
	}

	order.CallBackConfirm = mdb.CallBackConfirmOk
	return nil
}
