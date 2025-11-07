package trc20

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/assimon/luuu/blockchain"
	"github.com/assimon/luuu/model/mdb"
	"github.com/assimon/luuu/util/http_client"
	"github.com/assimon/luuu/util/json"
	"github.com/gookit/goutil/stdutil"
	"github.com/shopspring/decimal"
)

const (
	TRC20ApiUri              = "https://apilist.tronscanapi.com/api/transfer/trc20"
	USDTContractAddressTRC20 = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
)

type TRC20Service struct{}

type UsdtTrc20Resp struct {
	PageSize int             `json:"page_size"`
	Code     int             `json:"code"`
	Data     []TRC20Transfer `json:"data"`
}

type TRC20Transfer struct {
	Amount         string `json:"amount"`
	ApprovalAmount string `json:"approval_amount"`
	BlockTimestamp int64  `json:"block_timestamp"`
	Block          int    `json:"block"`
	From           string `json:"from"`
	To             string `json:"to"`
	Hash           string `json:"hash"`
	Confirmed      int    `json:"confirmed"`
	ContractType   string `json:"contract_type"`
	ContracTType   int    `json:"contractType"`
	Revert         int    `json:"revert"`
	ContractRet    string `json:"contract_ret"`
	EventType      string `json:"event_type"`
	IssueAddress   string `json:"issue_address"`
	Decimals       int    `json:"decimals"`
	TokenName      string `json:"token_name"`
	ID             string `json:"id"`
	Direction      int    `json:"direction"`
}

func NewTRC20Service() *TRC20Service {
	return &TRC20Service{}
}

func (s *TRC20Service) GetChainType() string {
	return mdb.ChainTypeTRC20
}

func (s *TRC20Service) GetUSDTContractAddress() string {
	return USDTContractAddressTRC20
}

func (s *TRC20Service) ValidateAddress(address string) bool {
	// TRC20地址以T开头，34个字符
	match, _ := regexp.MatchString(`^T[a-zA-Z0-9]{33}$`, address)
	return match
}

func (s *TRC20Service) GetTransactions(address string, startTime int64, endTime int64) ([]blockchain.Transaction, error) {
	client := http_client.GetHttpClient()

	resp, err := client.R().SetQueryParams(map[string]string{
		"sort":            "-timestamp",
		"limit":           "50",
		"start":           "0",
		"direction":       "2", // 2表示接收
		"db_version":      "1",
		"trc20Id":         USDTContractAddressTRC20,
		"address":         address,
		"start_timestamp": stdutil.ToString(startTime),
		"end_timestamp":   stdutil.ToString(endTime),
	}).Get(TRC20ApiUri)

	if err != nil {
		return nil, fmt.Errorf("TRC20 API 请求失败: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("TRC20 API 返回状态码: %d", resp.StatusCode())
	}

	var trc20Resp UsdtTrc20Resp
	err = json.Cjson.Unmarshal(resp.Body(), &trc20Resp)
	if err != nil {
		return nil, fmt.Errorf("解析 TRC20 响应失败: %w", err)
	}

	if trc20Resp.PageSize <= 0 {
		return []blockchain.Transaction{}, nil
	}

	transactions := make([]blockchain.Transaction, 0)
	for _, transfer := range trc20Resp.Data {
		// 只处理成功的交易
		if transfer.To != address || transfer.ContractRet != "SUCCESS" {
			continue
		}

		// 转换金额，TRC20 USDT是6位小数
		decimalQuant, err := decimal.NewFromString(transfer.Amount)
		if err != nil {
			continue
		}
		decimalDivisor := decimal.NewFromFloat(1000000)
		// 金额统一保留4位小数，避免精度不匹配问题，比如12.31和12.3100
		amount, _ := decimalQuant.Div(decimalDivisor).Round(4).Float64()

		tx := blockchain.Transaction{
			Hash:            transfer.Hash,
			From:            transfer.From,
			To:              transfer.To,
			Amount:          amount,
			BlockTimestamp:  transfer.BlockTimestamp,
			Confirmations:   transfer.Confirmed,
			Status:          transfer.ContractRet,
			ContractAddress: USDTContractAddressTRC20,
		}
		transactions = append(transactions, tx)
	}

	return transactions, nil
}

// GetTokenBalance 获取地址的代币余额（TRC20只支持USDT）
func (s *TRC20Service) GetTokenBalance(address string) (*blockchain.TokenBalance, error) {
	client := http_client.GetHttpClient()

	// TronScan API获取账户的TRC20代币余额
	resp, err := client.R().SetQueryParams(map[string]string{
		"address": address,
	}).Get("https://apilist.tronscanapi.com/api/account/tokens")

	if err != nil {
		return nil, fmt.Errorf("TronScan API 请求失败: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("TronScan API 返回状态码: %d", resp.StatusCode())
	}

	var apiResp struct {
		Data []struct {
			TokenId          string `json:"tokenId"`
			TokenAbbr        string `json:"tokenAbbr"`
			TokenName        string `json:"tokenName"`
			Balance          string `json:"balance"`
			TokenDecimal     int    `json:"tokenDecimal"`
			TokenCanShow     int    `json:"tokenCanShow"`
			TokenType        string `json:"tokenType"`
			TokenLogo        string `json:"tokenLogo"`
			Vip              bool   `json:"vip"`
			TokenPriceInTrx  string `json:"tokenPriceInTrx"`
			Amount           string `json:"amount"`
			NrOfTokenHolders int    `json:"nrOfTokenHolders"`
			TransferCount    int    `json:"transferCount"`
		} `json:"data"`
		Success bool `json:"success"`
	}

	err = json.Cjson.Unmarshal(resp.Body(), &apiResp)
	if err != nil {
		return nil, fmt.Errorf("解析 TronScan API 响应失败: %w", err)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("TronScan API 返回失败")
	}

	balance := &blockchain.TokenBalance{}

	// 查找USDT代币
	for _, token := range apiResp.Data {
		if token.TokenId == USDTContractAddressTRC20 && token.TokenType == "trc20" {
			// 解析余额
			balanceDecimal, err := decimal.NewFromString(token.Balance)
			if err != nil {
				continue
			}
			// TRC20 USDT 是 6 位小数
			divisor := decimal.NewFromInt(1000000)
			balance.USDT, _ = balanceDecimal.Div(divisor).Round(4).Float64()
			break
		}
	}

	// TRC20 不支持 USDC
	balance.USDC = 0

	return balance, nil
}

func init() {
	// 注册TRC20服务
	blockchain.RegisterChainService(NewTRC20Service())
}
