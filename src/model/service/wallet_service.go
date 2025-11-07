package service

import (
	"fmt"

	"github.com/assimon/luuu/blockchain"
	"github.com/assimon/luuu/model/data"
	"github.com/assimon/luuu/util/log"
)

// QueryWalletBalance 查询钱包余额
func QueryWalletBalance(walletID uint64) (float64, error) {
	// 获取钱包信息
	walletInfo, err := data.GetWalletAddressById(walletID)
	if err != nil {
		return 0, fmt.Errorf("获取钱包信息失败: %v", err)
	}

	if walletInfo.ID == 0 {
		return 0, fmt.Errorf("钱包不存在")
	}

	// 获取链服务
	chainService := blockchain.GetChainService(walletInfo.ChainType)
	if chainService == nil {
		return 0, fmt.Errorf("不支持的链类型: %s", walletInfo.ChainType)
	}

	// 查询链上余额
	balance, err := GetAddressUSDTBalance(walletInfo.Token, walletInfo.ChainType)
	if err != nil {
		log.Sugar.Errorf("[QueryWalletBalance] 查询余额失败: address=%s, chain=%s, err=%v",
			walletInfo.Token, walletInfo.ChainType, err)
		return 0, fmt.Errorf("查询余额失败: %v", err)
	}

	// 更新数据库中的余额
	err = data.UpdateWalletBalance(walletID, balance)
	if err != nil {
		log.Sugar.Errorf("[QueryWalletBalance] 更新余额失败: walletID=%d, balance=%.8f, err=%v",
			walletID, balance, err)
		// 即使更新失败，也返回查询到的余额
	}

	return balance, nil
}

// GetAddressUSDTBalance 获取地址的USDT+USDC余额（TRC20只查询USDT）
func GetAddressUSDTBalance(address string, chainType string) (float64, error) {
	chainService := blockchain.GetChainService(chainType)
	if chainService == nil {
		return 0, fmt.Errorf("不支持的链类型: %s", chainType)
	}

	// 使用真实的余额查询API
	tokenBalance, err := chainService.GetTokenBalance(address)
	if err != nil {
		return 0, fmt.Errorf("查询余额失败: %v", err)
	}

	// 返回 USDT + USDC 的总余额
	totalBalance := tokenBalance.USDT + tokenBalance.USDC
	return totalBalance, nil
}

// UpdateWalletBalanceAfterPayment 支付成功后更新钱包余额
func UpdateWalletBalanceAfterPayment(token string, chainType string) {
	balance, err := GetAddressUSDTBalance(token, chainType)
	if err != nil {
		log.Sugar.Errorf("[UpdateWalletBalanceAfterPayment] 查询余额失败: token=%s, chain=%s, err=%v",
			token, chainType, err)
		return
	}

	err = data.UpdateWalletBalanceByTokenAndChain(token, chainType, balance)
	if err != nil {
		log.Sugar.Errorf("[UpdateWalletBalanceAfterPayment] 更新余额失败: token=%s, chain=%s, balance=%.8f, err=%v",
			token, chainType, balance, err)
	} else {
		log.Sugar.Infof("[UpdateWalletBalanceAfterPayment] 余额更新成功: token=%s, chain=%s, balance=%.8f",
			token, chainType, balance)
	}
}
