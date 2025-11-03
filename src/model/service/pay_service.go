package service

import (
	"errors"

	"github.com/assimon/luuu/config"
	"github.com/assimon/luuu/model/data"
	"github.com/assimon/luuu/model/mdb"
	"github.com/assimon/luuu/model/response"
)

// GetCheckoutCounterByTradeId 获取收银台详情，通过订单
func GetCheckoutCounterByTradeId(tradeId string) (*response.CheckoutCounterResponse, error) {
	orderInfo, err := data.GetOrderInfoByTradeId(tradeId)
	if err != nil {
		return nil, err
	}
	if orderInfo.ID <= 0 {
		return nil, errors.New("订单不存在！")
	}

	// 实时检查订单是否已过期
	err = CheckAndUpdateOrderExpiration(orderInfo)
	if err != nil {
		return nil, err
	}

	// 检查订单状态
	if orderInfo.Status != mdb.StatusWaitPay {
		return nil, errors.New("不存在待支付订单或已过期！")
	}

	// 获取钱包备注信息
	walletInfo, _ := data.GetWalletAddressByTokenAndChainType(orderInfo.Token, orderInfo.ChainType)
	tokenRemark := ""
	if walletInfo != nil && walletInfo.ID > 0 {
		tokenRemark = walletInfo.Remark
	}

	resp := &response.CheckoutCounterResponse{
		TradeId:        orderInfo.TradeId,
		ActualAmount:   orderInfo.ActualAmount,
		Token:          orderInfo.Token,
		TokenRemark:    tokenRemark,
		ChainType:      orderInfo.ChainType,
		ExpirationTime: orderInfo.CreatedAt.AddMinutes(config.GetOrderExpirationTime()).TimestampWithMillisecond(),
		RedirectUrl:    orderInfo.RedirectUrl,
	}
	return resp, nil
}
