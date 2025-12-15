package telegram

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/assimon/luuu/blockchain"
	"github.com/assimon/luuu/model/data"
	"github.com/assimon/luuu/model/mdb"
	"github.com/assimon/luuu/model/request"
	"github.com/assimon/luuu/model/service"
	"github.com/assimon/luuu/util/log"

	tb "gopkg.in/telebot.v3"
)

// 临时存储用户选择的链类型（使用sync.Map保证并发安全）
var userChainTypeCache sync.Map

// 钱包地址信息（等待输入备注时使用）
type walletAddressInfo struct {
	Address   string
	ChainType string
}

// 临时存储用户钱包地址（等待输入备注）
var userWalletAddressCache sync.Map

// 临时存储用户选择的钱包信息（用于创建支付链接）
type walletInfo struct {
	ID        uint64
	Address   string
	ChainType string
	Remark    string
}

var userWalletCache sync.Map

// OnCallbackHandle 统一的回调处理器
func OnCallbackHandle(c tb.Context) error {
	callback := c.Callback()
	if callback == nil {
		return nil
	}

	// 获取按钮数据
	btnData := callback.Data
	log.Sugar.Infof("[Telegram] 收到回调，数据=%s", btnData)

	// 先响应回调，避免超时
	c.Respond()

	// 根据数据前缀分发处理
	parts := strings.Split(btnData, ":")
	if len(parts) == 0 {
		log.Sugar.Errorf("[Telegram] 操作失败：无效的按钮数据，数据=%s", btnData)
		return c.Send("操作失败：无效的按钮数据")
	}

	action := parts[0]

	switch action {
	case "add_wallet":
		return ShowChainTypeMenu(c)

	case "select_chain":
		if len(parts) < 2 {
			return c.Send("操作失败：缺少链类型")
		}
		return RequestWalletAddress(c, parts[1])

	case "view_wallet":
		if len(parts) < 2 {
			return c.Send("操作失败：缺少钱包ID")
		}
		id, _ := strconv.ParseUint(parts[1], 10, 64)
		return ShowWalletDetail(c, id)

	case "enable_wallet":
		if len(parts) < 2 {
			return c.Send("操作失败：缺少钱包ID")
		}
		id, _ := strconv.ParseUint(parts[1], 10, 64)
		return EnableWallet(c, id)

	case "disable_wallet":
		if len(parts) < 2 {
			return c.Send("操作失败：缺少钱包ID")
		}
		id, _ := strconv.ParseUint(parts[1], 10, 64)
		return DisableWallet(c, id)

	case "delete_wallet":
		if len(parts) < 2 {
			return c.Send("操作失败：缺少钱包ID")
		}
		id, _ := strconv.ParseUint(parts[1], 10, 64)
		return DeleteWallet(c, id)

	case "create_payment_link":
		if len(parts) < 2 {
			return c.Send("操作失败：缺少钱包ID")
		}
		id, _ := strconv.ParseUint(parts[1], 10, 64)
		return RequestPaymentAmount(c, id)

	case "back_to_list":
		return ShowWalletList(c)

	case "query_balance":
		if len(parts) < 2 {
			return c.Send("操作失败：缺少钱包ID")
		}
		id, _ := strconv.ParseUint(parts[1], 10, 64)
		return QueryBalance(c, id)

	default:
		return c.Send(fmt.Sprintf("未知操作: %s", action))
	}
}

// OnTextMessageHandle 处理文本消息
func OnTextMessageHandle(c tb.Context) error {
	if c.Message().ReplyTo == nil {
		return nil
	}

	// 处理添加钱包地址
	if strings.Contains(c.Message().ReplyTo.Text, "请发送") && strings.Contains(c.Message().ReplyTo.Text, "钱包地址") {
		walletAddress := strings.TrimSpace(c.Message().Text)
		chainTypeVal, ok := userChainTypeCache.Load(c.Sender().ID)
		if !ok {
			return c.Send("链类型信息丢失，请重新添加钱包")
		}
		chainType := chainTypeVal.(string)

		// 验证地址格式
		chainService := blockchain.GetChainService(chainType)
		if chainService == nil {
			return c.Send(fmt.Sprintf("不支持的链类型: %s", chainType))
		}

		if !chainService.ValidateAddress(walletAddress) {
			return c.Send(fmt.Sprintf("地址格式错误：无效的 %s 地址", chainType))
		}

		// 缓存钱包地址，等待输入备注
		userWalletAddressCache.Store(c.Sender().ID, &walletAddressInfo{
			Address:   walletAddress,
			ChainType: chainType,
		})

		// 请求输入备注
		return RequestWalletRemark(c, walletAddress, chainType)
	}

	// 处理添加钱包备注
	if strings.Contains(c.Message().ReplyTo.Text, "请输入备注名称") {
		remark := strings.TrimSpace(c.Message().Text)

		// 如果用户输入"无"，则不设置备注
		if remark == "无" {
			remark = ""
		}

		walletVal, ok := userWalletAddressCache.Load(c.Sender().ID)
		if !ok {
			return c.Send("钱包地址信息丢失，请重新添加钱包")
		}
		wallet := walletVal.(*walletAddressInfo)

		// 添加钱包（带备注）
		_, err := data.AddWalletAddress(wallet.Address, wallet.ChainType, remark)
		if err != nil {
			return c.Send(fmt.Sprintf("添加失败：%s", err.Error()))
		}

		// 清除缓存
		userChainTypeCache.Delete(c.Sender().ID)
		userWalletAddressCache.Delete(c.Sender().ID)

		displayInfo := fmt.Sprintf("%s", wallet.Address)
		if remark != "" {
			displayInfo = fmt.Sprintf("%s (%s)", wallet.Address, remark)
		}

		c.Send(fmt.Sprintf("【钱包添加成功】\n\n链类型：%s\n地址：%s", wallet.ChainType, displayInfo))
		return ShowWalletList(c)
	}

	// 处理支付金额输入
	if strings.Contains(c.Message().ReplyTo.Text, "请输入支付金额") {
		amountStr := strings.TrimSpace(c.Message().Text)
		amount, err := strconv.ParseFloat(amountStr, 64)
		if err != nil {
			return c.Send("金额格式不正确，请输入数字\n例如：100 或 100.5")
		}

		if amount <= 0 {
			return c.Send("金额必须大于0")
		}

		walletVal, ok := userWalletCache.Load(c.Sender().ID)
		if !ok {
			return c.Send("钱包信息丢失，请重新操作")
		}
		wallet := walletVal.(*walletInfo)

		// 创建支付链接
		return CreatePaymentLink(c, wallet, amount)
	}

	return nil
}

// ShowWalletList 显示钱包列表
func ShowWalletList(c tb.Context) error {
	wallets, err := data.GetAllWalletAddress()
	if err != nil {
		return c.Send(fmt.Sprintf("获取钱包列表失败：%s", err.Error()))
	}

	message := "【钱包管理】\n\n"

	var buttons [][]tb.InlineButton

	if len(wallets) == 0 {
		message += "暂无钱包地址\n"
	} else {
		// 按链类型分组
		chainGroups := make(map[string][]mdb.WalletAddress)
		for _, wallet := range wallets {
			chainType := wallet.ChainType
			if chainType == "" {
				chainType = mdb.ChainTypeTRC20
			}
			chainGroups[chainType] = append(chainGroups[chainType], wallet)
		}

		// 显示每个链的钱包
		chainOrder := []string{mdb.ChainTypeTRC20, mdb.ChainTypeERC20, mdb.ChainTypeBEP20, mdb.ChainTypePOLYGON, mdb.ChainTypeARB, mdb.ChainTypeSOLANA}
		for _, chainType := range chainOrder {
			if walletList, ok := chainGroups[chainType]; ok && len(walletList) > 0 {
				// message += fmt.Sprintf("[%s]\n", chainType)
				for _, wallet := range walletList {
					status := "启用"
					if wallet.Status == mdb.TokenStatusDisable {
						status = "禁用"
					}

					// 截取地址显示
					displayAddr := wallet.Token
					if len(wallet.Token) > 20 {
						displayAddr = wallet.Token[:6] + "..." + wallet.Token[len(wallet.Token)-6:]
					}

					// 显示格式：地址 + 备注
					buttonText := ""
					if wallet.Remark != "" {
						buttonText = fmt.Sprintf("[%s] %s (%s) - %s", wallet.ChainType, displayAddr, wallet.Remark, status)
					} else {
						buttonText = fmt.Sprintf("[%s] %s (%s)", wallet.ChainType, displayAddr, status)
					}

					// 创建钱包按钮
					btn := tb.InlineButton{
						Text: buttonText,
						Data: fmt.Sprintf("view_wallet:%d", wallet.ID),
					}
					buttons = append(buttons, []tb.InlineButton{btn})
				}
			}
		}
	}

	// message += "点击钱包查看详情\n点击下方按钮添加新钱包"

	// 添加钱包按钮
	addBtn := tb.InlineButton{
		Text: "添加钱包地址",
		Data: "add_wallet",
	}
	buttons = append(buttons, []tb.InlineButton{addBtn})

	return c.Send(message, &tb.SendOptions{
		ReplyMarkup: &tb.ReplyMarkup{
			InlineKeyboard: buttons,
		},
	})
}

// ShowChainTypeMenu 显示链类型选择菜单
func ShowChainTypeMenu(c tb.Context) error {
	buttons := [][]tb.InlineButton{
		{{Text: "TRC20 (波场)", Data: "select_chain:TRC20"}},
		{{Text: "ERC20 (以太坊)", Data: "select_chain:ERC20"}},
		{{Text: "BEP20 (币安链)", Data: "select_chain:BEP20"}},
		{{Text: "POLYGON (Polygon)", Data: "select_chain:POLYGON"}},
		{{Text: "ARB (Arbitrum)", Data: "select_chain:ARBITRUM"}},
		{{Text: "SOLANA", Data: "select_chain:SOLANA"}},
		{{Text: "返回", Data: "back_to_list"}},
	}

	return c.Send("请选择要添加的链类型:", &tb.SendOptions{
		ReplyMarkup: &tb.ReplyMarkup{
			InlineKeyboard: buttons,
		},
	})
}

// RequestWalletAddress 请求用户输入钱包地址
func RequestWalletAddress(c tb.Context, chainType string) error {
	userChainTypeCache.Store(c.Sender().ID, chainType)

	message := fmt.Sprintf("【添加 %s 钱包 - 步骤 1/2】\n\n", chainType)

	// 添加格式提示
	switch chainType {
	case mdb.ChainTypeTRC20:
		message += "格式：以T开头，34位字符\n示例：TQWh7yxxvJkxPVrXkhaQDqvVsrw4uG1FVJ"
	case mdb.ChainTypeERC20:
		message += "格式：以0x开头，42位字符\n示例：0x9f8620f01a98Ca608db53842e3989f6C89Cc7519"
	case mdb.ChainTypeBEP20:
		message += "格式：以0x开头，42位字符\n示例：0x9f8620f01a98Ca608db53842e3989f6C89Cc7519"
	case mdb.ChainTypePOLYGON:
		message += "格式：以0x开头，42位字符\n示例：0x9f8620f01a98Ca608db53842e3989f6C89Cc7519"
	case mdb.ChainTypeARB:
		message += "格式：以0x开头，42位字符\n示例：0x9f8620f01a98Ca608db53842e3989f6C89Cc7519"
	case mdb.ChainTypeSOLANA:
		message += "格式：Base58编码，32-44位字符\n示例：2rJqjpvAuLjdVerBacXGraHxVcVkQcuKGvQUi785FUfa"
	}

	message += "\n\n请发送钱包地址："

	return c.Send(message, &tb.SendOptions{
		ReplyMarkup: &tb.ReplyMarkup{
			ForceReply: true,
		},
	})
}

// RequestWalletRemark 请求用户输入钱包备注
func RequestWalletRemark(c tb.Context, address string, chainType string) error {
	message := fmt.Sprintf("【添加 %s 钱包 - 步骤 2/2】\n\n", chainType)
	message += fmt.Sprintf("地址：%s\n\n", address)
	message += "请输入备注名称（例如：主钱包、备用钱包1）：\n"
	message += "如不需要备注，请发送：无"

	return c.Send(message, &tb.SendOptions{
		ReplyMarkup: &tb.ReplyMarkup{
			ForceReply: true,
		},
	})
}

// ShowWalletDetail 显示钱包详情
func ShowWalletDetail(c tb.Context, id uint64) error {
	tokenInfo, err := data.GetWalletAddressById(id)
	if err != nil {
		return c.Send(fmt.Sprintf("获取钱包信息失败：%s", err.Error()))
	}

	status := "已启用"
	if tokenInfo.Status == mdb.TokenStatusDisable {
		status = "已禁用"
	}

	message := "【钱包详情】\n\n"
	message += fmt.Sprintf("链类型：%s\n", tokenInfo.ChainType)

	// 显示备注（如果有）
	if tokenInfo.Remark != "" {
		message += fmt.Sprintf("备注：%s\n", tokenInfo.Remark)
	}

	message += fmt.Sprintf("状态：%s\n", status)

	// 显示余额（如果有更新过）
	if tokenInfo.Balance > 0 || tokenInfo.BalanceUpdatedAt != nil {
		// TRC20 只显示 USDT，其他链显示 USDT/USDC
		balanceLabel := "USDT/USDC"
		if tokenInfo.ChainType == mdb.ChainTypeTRC20 {
			balanceLabel = "USDT"
		}
		message += fmt.Sprintf("余额：%.4f %s\n", tokenInfo.Balance, balanceLabel)
		if tokenInfo.BalanceUpdatedAt != nil {
			message += fmt.Sprintf("更新时间：%s\n", tokenInfo.BalanceUpdatedAt.ToDateTimeString())
		}
	}

	message += fmt.Sprintf("\n地址：\n%s", tokenInfo.Token)

	buttons := [][]tb.InlineButton{
		{
			{Text: "启用", Data: fmt.Sprintf("enable_wallet:%d", id)},
			{Text: "禁用", Data: fmt.Sprintf("disable_wallet:%d", id)},
			{Text: "删除", Data: fmt.Sprintf("delete_wallet:%d", id)},
		},
		{
			{Text: "查询余额", Data: fmt.Sprintf("query_balance:%d", id)},
			{Text: "创建支付", Data: fmt.Sprintf("create_payment_link:%d", id)},
		},
		{{Text: "返回", Data: "back_to_list"}},
	}

	return c.Send(message, &tb.SendOptions{
		ReplyMarkup: &tb.ReplyMarkup{
			InlineKeyboard: buttons,
		},
	})
}

// EnableWallet 启用钱包
func EnableWallet(c tb.Context, id uint64) error {
	if id <= 0 {
		return c.Send("请求不合法")
	}

	err := data.ChangeWalletAddressStatus(id, mdb.TokenStatusEnable)
	if err != nil {
		return c.Send(fmt.Sprintf("操作失败：%s", err.Error()))
	}

	c.Send("操作成功：钱包已启用")
	return ShowWalletDetail(c, id)
}

// DisableWallet 禁用钱包
func DisableWallet(c tb.Context, id uint64) error {
	if id <= 0 {
		return c.Send("请求不合法")
	}

	err := data.ChangeWalletAddressStatus(id, mdb.TokenStatusDisable)
	if err != nil {
		return c.Send(fmt.Sprintf("操作失败：%s", err.Error()))
	}

	c.Send("操作成功：钱包已禁用")
	return ShowWalletDetail(c, id)
}

// DeleteWallet 删除钱包
func DeleteWallet(c tb.Context, id uint64) error {
	if id <= 0 {
		return c.Send("请求不合法")
	}

	err := data.DeleteWalletAddressById(id)
	if err != nil {
		return c.Send(fmt.Sprintf("删除失败：%s", err.Error()))
	}

	c.Send("操作成功：钱包已删除")
	return ShowWalletList(c)
}

// RequestPaymentAmount 请求用户输入支付金额
func RequestPaymentAmount(c tb.Context, walletID uint64) error {
	tokenInfo, err := data.GetWalletAddressById(walletID)
	if err != nil {
		return c.Send(fmt.Sprintf("获取钱包信息失败：%s", err.Error()))
	}

	// 缓存钱包信息
	userWalletCache.Store(c.Sender().ID, &walletInfo{
		ID:        tokenInfo.ID,
		Address:   tokenInfo.Token,
		ChainType: tokenInfo.ChainType,
		Remark:    tokenInfo.Remark,
	})

	message := "【创建支付链接】\n\n"
	message += fmt.Sprintf("链类型：%s\n", tokenInfo.ChainType)

	// 显示备注（如果有）
	if tokenInfo.Remark != "" {
		message += fmt.Sprintf("备注：%s\n", tokenInfo.Remark)
	}

	message += fmt.Sprintf("收款地址：\n%s\n\n", tokenInfo.Token)
	message += "请输入支付金额（单位：人民币）\n例如：100"

	return c.Send(message, &tb.SendOptions{
		ReplyMarkup: &tb.ReplyMarkup{
			ForceReply: true,
		},
	})
}

// CreatePaymentLink 创建支付链接
func CreatePaymentLink(c tb.Context, wallet *walletInfo, amount float64) error {
	// 生成唯一订单号
	orderID := fmt.Sprintf("TG%d%d", c.Sender().ID, time.Now().Unix())

	// 创建订单请求
	createReq := &request.CreateTransactionRequest{
		OrderId:     orderID,
		Amount:      amount,
		NotifyUrl:   "https://example.com/notify", // 手动创建的订单使用占位URL
		RedirectUrl: "",
		ChainType:   wallet.ChainType,
	}

	// 调用订单服务创建订单
	resp, err := service.CreateTransaction(createReq)
	if err != nil {
		userWalletCache.Delete(c.Sender().ID)
		return c.Send(fmt.Sprintf("创建失败：%s", err.Error()))
	}

	// 清除缓存
	userWalletCache.Delete(c.Sender().ID)

	// 返回支付信息
	message := "【支付链接创建成功】\n\n"
	message += fmt.Sprintf("订单号：%s\n", resp.TradeId)
	message += fmt.Sprintf("链类型：%s\n", resp.ChainType)

	// 显示备注（如果有）
	if wallet.Remark != "" {
		message += fmt.Sprintf("备注：%s\n", wallet.Remark)
	}

	// TRC20 只显示 USDT，其他链显示 USDT/USDC
	paymentLabel := "USDT/USDC"
	if resp.ChainType == mdb.ChainTypeTRC20 {
		paymentLabel = "USDT"
	}
	message += fmt.Sprintf("支付金额：%.4f %s\n\n", resp.ActualAmount, paymentLabel)
	message += fmt.Sprintf("收款地址：\n`%s`\n\n", resp.Token)
	message += fmt.Sprintf("收银台：\n%s\n\n", resp.PaymentUrl)
	message += fmt.Sprintf("过期时间：%s", time.Unix(resp.ExpirationTime, 0).Format("2006-01-02 15:04:05"))

	return c.Send(message, &tb.SendOptions{
		ParseMode: "Markdown",
	})
}

// QueryBalance 查询钱包余额
func QueryBalance(c tb.Context, id uint64) error {
	if id <= 0 {
		return c.Send("请求不合法")
	}

	// 发送查询中提示
	c.Send("正在查询余额，请稍候...")

	// 查询余额
	balance, err := service.QueryWalletBalance(id)
	if err != nil {
		return c.Send(fmt.Sprintf("查询失败：%s", err.Error()))
	}

	// 获取钱包信息
	tokenInfo, err := data.GetWalletAddressById(id)
	if err != nil {
		return c.Send(fmt.Sprintf("获取钱包信息失败：%s", err.Error()))
	}

	message := "【余额查询成功】\n\n"
	message += fmt.Sprintf("链类型：%s\n", tokenInfo.ChainType)
	if tokenInfo.Remark != "" {
		message += fmt.Sprintf("备注：%s\n", tokenInfo.Remark)
	}
	// TRC20 只显示 USDT，其他链显示 USDT/USDC
	balanceLabel := "USDT/USDC"
	if tokenInfo.ChainType == mdb.ChainTypeTRC20 {
		balanceLabel = "USDT"
	}
	message += fmt.Sprintf("余额：%.4f %s\n", balance, balanceLabel)
	message += fmt.Sprintf("查询时间：%s", time.Now().Format("2006-01-02 15:04:05"))

	// c.Send(message)
	return ShowWalletDetail(c, id)
}
