package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/assimon/luuu/blockchain"
	"github.com/assimon/luuu/model/dao"
	"github.com/assimon/luuu/model/data"
	"github.com/assimon/luuu/model/mdb"
	"github.com/assimon/luuu/model/request"
	"github.com/assimon/luuu/mq/handle"
	"github.com/assimon/luuu/notify"
	"github.com/assimon/luuu/util/log"
	"github.com/golang-module/carbon/v2"
)

// GetBlockchainExplorerURL 获取区块链浏览器URL
func GetBlockchainExplorerURL(chainType string, txHash string) string {
	switch chainType {
	case mdb.ChainTypeTRC20:
		return fmt.Sprintf("https://tronscan.org/#/transaction/%s", txHash)
	case mdb.ChainTypeERC20:
		return fmt.Sprintf("https://etherscan.io/tx/%s", txHash)
	case mdb.ChainTypeBEP20:
		return fmt.Sprintf("https://bscscan.com/tx/%s", txHash)
	case mdb.ChainTypePOLYGON:
		return fmt.Sprintf("https://polygonscan.com/tx/%s", txHash)
	case mdb.ChainTypeSOLANA:
		return fmt.Sprintf("https://solscan.io/tx/%s", txHash)
	default:
		return ""
	}
}

// GetTokenSymbol 根据合约地址获取代币符号
func GetTokenSymbol(contractAddress string, chainType string) string {
	// USDT合约地址
	usdtContracts := map[string]string{
		"0xdac17f958d2ee523a2206206994597c13d831ec7":   "USDT", // ERC20
		"TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t":           "USDT", // TRC20
		"0x55d398326f99059fF775485246999027B3197955":   "USDT", // BEP20
		"0xc2132D05D31c914a87C6611C10748AEb04B58e8F":   "USDT", // Polygon
		"Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB": "USDT", // Solana
	}

	// USDC合约地址（TRC20除外）
	usdcContracts := map[string]string{
		"0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48":   "USDC", // ERC20
		"0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d":   "USDC", // BEP20
		"0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174":   "USDC", // Polygon
		"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v": "USDC", // Solana
	}

	// 检查是否是USDT
	if symbol, ok := usdtContracts[contractAddress]; ok {
		return symbol
	}

	// 检查是否是USDC
	if symbol, ok := usdcContracts[contractAddress]; ok {
		return symbol
	}

	// 默认返回USDT（向后兼容）
	return "USDT"
}

// ChainCallBack 通用区块链回调处理
func ChainCallBack(address string, chainType string, wg *sync.WaitGroup) {
	defer wg.Done()
	defer func() {
		if err := recover(); err != nil {
			log.Sugar.Errorf("[%s] 区块链回调异常: %v", chainType, err)
		}
	}()

	log.Sugar.Debugf("[%s] 开始处理地址回调: %s", chainType, address)

	// 获取对应链的服务
	chainService := blockchain.GetChainService(chainType)
	if chainService == nil {
		log.Sugar.Errorf("[%s] 未找到区块链服务", chainType)
		return
	}

	log.Sugar.Debugf("[%s] 已找到区块链服务，正在查询交易...", chainType)

	// 查询最近24小时的交易
	startTime := carbon.Now().AddHours(-24).TimestampWithMillisecond()
	endTime := carbon.Now().TimestampWithMillisecond()

	transactions, err := chainService.GetTransactions(address, startTime, endTime)
	if err != nil {
		// API 失败时只记录警告，不中断服务
		log.Sugar.Warnf("[%s] API调用失败 %s: %v (将在下次周期重试)", chainType, address, err)
		return
	}

	log.Sugar.Debugf("[%s] API返回 %d 笔交易", chainType, len(transactions))

	if len(transactions) == 0 {
		log.Sugar.Debugf("[%s] 未找到交易记录 %s", chainType, address)
		return
	}

	log.Sugar.Infof("[%s] 找到 %d 笔交易，地址 %s", chainType, len(transactions), address)

	// 处理每笔交易
	for i, tx := range transactions {
		log.Sugar.Infof("[%s] 处理交易 %d/%d: 哈希=%s, 金额=%.4f, 发送方=%s, 接收方=%s",
			chainType, i+1, len(transactions), tx.Hash, tx.Amount, tx.From, tx.To)

		// 根据钱包地址和金额查询订单
		log.Sugar.Debugf("[%s] 查找订单: 地址=%s, 金额=%.4f", chainType, address, tx.Amount)

		tradeId, err := data.GetTradeIdByWalletAddressAndAmountAndChainType(address, tx.Amount, chainType)
		if err != nil {
			log.Sugar.Errorf("[%s] 获取交易号失败: %v", chainType, err)
			continue
		}

		if tradeId == "" {
			log.Sugar.Debugf("[%s] 未找到匹配订单，金额=%.4f", chainType, tx.Amount)
			continue
		}

		log.Sugar.Infof("[%s] 找到匹配订单！交易号=%s, 金额=%.4f", chainType, tradeId, tx.Amount)

		// 获取订单信息
		order, err := data.GetOrderInfoByTradeId(tradeId)
		if err != nil {
			log.Sugar.Errorf("[%s] 获取订单信息失败: %v", chainType, err)
			continue
		}

		log.Sugar.Infof("[%s] 订单信息: 交易号=%s, 订单号=%s, 状态=%d, 金额=%.2f, 实际金额=%.4f",
			chainType, order.TradeId, order.OrderId, order.Status, order.Amount, order.ActualAmount)

		// 验证链类型匹配
		if order.ChainType != chainType {
			log.Sugar.Warnf("[%s] 链类型不匹配: 订单=%s, 交易=%s",
				chainType, order.ChainType, chainType)
			continue
		}

		// 区块的确认时间必须在订单创建时间之后
		createTime := order.CreatedAt.TimestampWithMillisecond()
		log.Sugar.Debugf("[%s] 时间检查: 交易时间=%d, 订单时间=%d", chainType, tx.BlockTimestamp, createTime)

		if tx.BlockTimestamp < createTime {
			log.Sugar.Warnf("[%s] 交易时间(%d) 早于订单创建时间(%d)",
				chainType, tx.BlockTimestamp, createTime)
			continue
		}

		log.Sugar.Infof("[%s] 所有验证通过，正在处理支付...", chainType)

		// 到这一步就完全算是支付成功了
		req := &request.OrderProcessingRequest{
			Token:              address,
			TradeId:            tradeId,
			Amount:             tx.Amount,
			BlockTransactionId: tx.Hash,
		}

		log.Sugar.Infof("处理支付: 交易号=%s, 金额=%f, 交易哈希=%s", tradeId, tx.Amount, tx.Hash)

		err = OrderProcessing(req)
		if err != nil {
			log.Sugar.Errorf("处理订单失败 %s: %v", tradeId, err)
			continue
		}

		log.Sugar.Infof("支付处理成功，交易号=%s", tradeId)

		// 更新钱包余额
		go UpdateWalletBalanceAfterPayment(address, chainType)

		// 回调队列
		ctx := context.Background()
		dao.EnqueueTaskNow(ctx, "default", handle.QueueOrderCallback, order, 5)

		// 发送机器人消息
		explorerURL := GetBlockchainExplorerURL(chainType, tx.Hash)
		tokenSymbol := GetTokenSymbol(tx.ContractAddress, chainType)
		msgTpl := `【支付成功通知】

区块链：%s
交易号：%s
订单号：%s
请求金额：%.2f 元
支付币种：%s
支付金额：%.4f
收款地址：%s

交易哈希：
%s

区块链浏览器：
%s

订单创建时间：%s
支付成功时间：%s`
		msg := fmt.Sprintf(msgTpl,
			chainType,
			order.TradeId,
			order.OrderId,
			order.Amount,
			tokenSymbol,
			order.ActualAmount,
			order.Token,
			tx.Hash,
			explorerURL,
			order.CreatedAt.ToDateTimeString(),
			carbon.Now().ToDateTimeString())
		notify.SendToBot(msg)
	}
}
