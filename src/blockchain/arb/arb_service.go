package arb

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/assimon/luuu/blockchain"
	"github.com/assimon/luuu/config"
	"github.com/assimon/luuu/model/mdb"
	"github.com/assimon/luuu/util/http_client"
	"github.com/assimon/luuu/util/json"
	"github.com/shopspring/decimal"
)

const (
	EtherscanApiV2Uri      = "https://api.etherscan.io/v2/api"            // Etherscan API V2
	ArbitrumChainID        = "42161"                                     // Arbitrum One Mainnet
	USDTContractAddressARB = "0xFd086bC7CD5C481DCC9C85ebE478A1C0b69FCbb9" // USDT on Arbitrum
	USDCContractAddressARB = "0xaf88d065e77c8cC2239327C5EDb3A432268e5831" // USDC on Arbitrum
)

type ARBService struct {
	rateLimiter *time.Ticker
	mu          sync.Mutex
}

var (
	arbServiceInstance *ARBService
	arbServiceOnce     sync.Once
)

type ARBScanResponse struct {
	Status  string        `json:"status"`
	Message string        `json:"message"`
	Result  []ARBTransfer `json:"result"`
}

type ARBTransfer struct {
	BlockNumber       string `json:"blockNumber"`
	TimeStamp         string `json:"timeStamp"`
	Hash              string `json:"hash"`
	Nonce             string `json:"nonce"`
	BlockHash         string `json:"blockHash"`
	From              string `json:"from"`
	ContractAddress   string `json:"contractAddress"`
	To                string `json:"to"`
	Value             string `json:"value"`
	TokenName         string `json:"tokenName"`
	TokenSymbol       string `json:"tokenSymbol"`
	TokenDecimal      string `json:"tokenDecimal"`
	TransactionIndex  string `json:"transactionIndex"`
	Gas               string `json:"gas"`
	GasPrice          string `json:"gasPrice"`
	GasUsed           string `json:"gasUsed"`
	CumulativeGasUsed string `json:"cumulativeGasUsed"`
	Input             string `json:"input"`
	Confirmations     string `json:"confirmations"`
}

func NewARBService() *ARBService {
	arbServiceOnce.Do(func() {
		arbServiceInstance = &ARBService{
			// 速率限制，每1秒最多1次请求，免费API限制
			rateLimiter: time.NewTicker(time.Second * 1),
		}
	})
	return arbServiceInstance
}

func (s *ARBService) GetChainType() string {
	return mdb.ChainTypeARB
}

func (s *ARBService) GetUSDTContractAddress() string {
	return USDTContractAddressARB
}

func (s *ARBService) ValidateAddress(address string) bool {
	// Arbitrum地址格式与ERC20相同，以0x开头，42个字符
	match, _ := regexp.MatchString(`^0x[a-fA-F0-9]{40}$`, address)
	return match
}

func (s *ARBService) GetTransactions(address string, startTime int64, endTime int64) ([]blockchain.Transaction, error) {
	// 同时查询 USDT 和 USDC 交易
	usdtTxs, err := s.getTransactionsByContract(address, startTime, endTime, USDTContractAddressARB)
	if err != nil {
		// USDT 查询失败，记录错误但继续查询 USDC
		usdtTxs = []blockchain.Transaction{}
	}

	usdcTxs, err := s.getTransactionsByContract(address, startTime, endTime, USDCContractAddressARB)
	if err != nil {
		// USDC 查询失败，记录错误但继续
		usdcTxs = []blockchain.Transaction{}
	}

	// 合并交易列表
	allTxs := append(usdtTxs, usdcTxs...)
	return allTxs, nil
}

// getTransactionsByContract 查询指定合约地址的交易
func (s *ARBService) getTransactionsByContract(address string, startTime int64, endTime int64, contractAddress string) ([]blockchain.Transaction, error) {
	apiKey := config.GetEtherscanApiKey()
	if apiKey == "" {
		return nil, fmt.Errorf("未配置 Etherscan API 密钥")
	}

	// 速率限制，等待令牌
	s.mu.Lock()
	<-s.rateLimiter.C
	s.mu.Unlock()

	client := http_client.GetHttpClient()

	// 转换时间戳（毫秒转秒）
	startBlock := "0"
	endBlock := "99999999"

	resp, err := client.R().SetQueryParams(map[string]string{
		"chainid":         ArbitrumChainID,
		"module":          "account",
		"action":          "tokentx",
		"contractaddress": contractAddress,
		"address":         address,
		"page":            "1",
		"offset":          "100",
		"startblock":      startBlock,
		"endblock":        endBlock,
		"sort":            "desc",
		"apikey":          apiKey,
	}).Get(EtherscanApiV2Uri)

	if err != nil {
		return nil, fmt.Errorf("Etherscan API V2 请求失败: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("Etherscan API V2 返回状态码: %d, 响应: %s", resp.StatusCode(), string(resp.Body()))
	}

	var arbScanResp ARBScanResponse
	err = json.Cjson.Unmarshal(resp.Body(), &arbScanResp)
	if err != nil {
		return nil, fmt.Errorf("解析 Etherscan API V2 响应失败: %w, 响应内容: %s", err, string(resp.Body()))
	}

	// 如果API返回错误，返回空数组而不是错误，降级处理
	if arbScanResp.Status != "1" {
		// 如果是速率限制错误，稍微延迟一下
		if arbScanResp.Message == "NOTOK" {
			time.Sleep(time.Second * 2)
		}
		return []blockchain.Transaction{}, nil
	}

	transactions := make([]blockchain.Transaction, 0)
	for _, transfer := range arbScanResp.Result {
		// 只处理接收到的交易，0x开头的地址需要忽略大小写比对
		if !strings.EqualFold(transfer.To, address) {
			continue
		}

		// 解析时间戳
		timestamp, err := strconv.ParseInt(transfer.TimeStamp, 10, 64)
		if err != nil {
			continue
		}

		// 转换为毫秒
		timestampMs := timestamp * 1000
		// 检查时间范围
		if timestampMs < startTime || timestampMs > endTime {
			continue
		}

		// 转换金额，Arbitrum USDT/USDC是6位小数
		decimalQuant, err := decimal.NewFromString(transfer.Value)
		if err != nil {
			continue
		}

		// 获取小数位
		tokenDecimal, err := strconv.Atoi(transfer.TokenDecimal)
		if err != nil {
			tokenDecimal = 6 // 默认为6位
		}

		divisor := decimal.NewFromFloat(1)
		for i := 0; i < tokenDecimal; i++ {
			divisor = divisor.Mul(decimal.NewFromInt(10))
		}

		// 金额统一保留4位小数，避免精度不匹配问题，比如12.31和12.3100
		amount, _ := decimalQuant.Div(divisor).Round(4).Float64()

		// 解析确认数
		confirmations, _ := strconv.Atoi(transfer.Confirmations)

		tx := blockchain.Transaction{
			Hash:            transfer.Hash,
			From:            transfer.From,
			To:              transfer.To,
			Amount:          amount,
			BlockTimestamp:  timestampMs,
			Confirmations:   confirmations,
			Status:          "SUCCESS",
			ContractAddress: contractAddress, // 使用实际的合约地址
		}
		transactions = append(transactions, tx)
	}

	return transactions, nil
}

// GetTokenBalance 获取地址的代币余额（USDT + USDC）
func (s *ARBService) GetTokenBalance(address string) (*blockchain.TokenBalance, error) {
	apiKey := config.GetEtherscanApiKey()
	if apiKey == "" {
		return nil, fmt.Errorf("未配置 Etherscan API 密钥")
	}

	balance := &blockchain.TokenBalance{}

	// 查询 USDT 余额
	usdtBalance, err := s.getTokenBalanceByContract(address, USDTContractAddressARB, apiKey)
	if err == nil {
		balance.USDT = usdtBalance
	}

	// 查询 USDC 余额
	usdcBalance, err := s.getTokenBalanceByContract(address, USDCContractAddressARB, apiKey)
	if err == nil {
		balance.USDC = usdcBalance
	}

	return balance, nil
}

// getTokenBalanceByContract 查询指定合约的代币余额
func (s *ARBService) getTokenBalanceByContract(address, contractAddress, apiKey string) (float64, error) {
	// 速率限制
	s.mu.Lock()
	<-s.rateLimiter.C
	s.mu.Unlock()

	client := http_client.GetHttpClient()

	resp, err := client.R().SetQueryParams(map[string]string{
		"chainid":         ArbitrumChainID,
		"module":          "account",
		"action":          "tokenbalance",
		"contractaddress": contractAddress,
		"address":         address,
		"tag":             "latest",
		"apikey":          apiKey,
	}).Get(EtherscanApiV2Uri)

	if err != nil {
		return 0, fmt.Errorf("Etherscan API V2 请求失败: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return 0, fmt.Errorf("Etherscan API V2 返回状态码: %d, 响应: %s", resp.StatusCode(), string(resp.Body()))
	}

	var apiResp struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Result  string `json:"result"`
	}

	err = json.Cjson.Unmarshal(resp.Body(), &apiResp)
	if err != nil {
		return 0, fmt.Errorf("解析 Etherscan API V2 响应失败: %w, 响应内容: %s", err, string(resp.Body()))
	}

	if apiResp.Status != "1" {
		return 0, fmt.Errorf("Etherscan API V2 返回错误: %s", apiResp.Message)
	}

	// 解析余额，Arbitrum USDT/USDC 是 6 位小数
	balanceDecimal, err := decimal.NewFromString(apiResp.Result)
	if err != nil {
		return 0, fmt.Errorf("解析余额失败: %w", err)
	}

	// Arbitrum 上的 USDT 和 USDC 都是 6 位小数
	divisor := decimal.NewFromInt(1000000)
	balance, _ := balanceDecimal.Div(divisor).Round(4).Float64()

	return balance, nil
}

func init() {
	// 注册ARB服务
	blockchain.RegisterChainService(NewARBService())
}

