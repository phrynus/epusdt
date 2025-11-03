package erc20

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
	EtherscanApiV2Uri        = "https://api.etherscan.io/v2/api"
	EthereumChainID          = "1"                                          // Ethereum Mainnet
	USDTContractAddressERC20 = "0xdac17f958d2ee523a2206206994597c13d831ec7" // USDT on Ethereum
)

type ERC20Service struct {
	rateLimiter *time.Ticker
	mu          sync.Mutex
}

var (
	erc20ServiceInstance *ERC20Service
	erc20ServiceOnce     sync.Once
)

type EtherscanResponse struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Result  []ERC20Transfer `json:"result"`
}

type ERC20Transfer struct {
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

func NewERC20Service() *ERC20Service {
	erc20ServiceOnce.Do(func() {
		erc20ServiceInstance = &ERC20Service{
			// 速率限制，每1秒最多1次请求，免费API限制
			rateLimiter: time.NewTicker(time.Second * 1),
		}
	})
	return erc20ServiceInstance
}

func (s *ERC20Service) GetChainType() string {
	return mdb.ChainTypeERC20
}

func (s *ERC20Service) GetUSDTContractAddress() string {
	return USDTContractAddressERC20
}

func (s *ERC20Service) ValidateAddress(address string) bool {
	// ERC20地址以0x开头，42个字符
	match, _ := regexp.MatchString(`^0x[a-fA-F0-9]{40}$`, address)
	return match
}

func (s *ERC20Service) GetTransactions(address string, startTime int64, endTime int64) ([]blockchain.Transaction, error) {
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
		"chainid":         EthereumChainID,
		"module":          "account",
		"action":          "tokentx",
		"contractaddress": USDTContractAddressERC20,
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
		return nil, fmt.Errorf("Etherscan API V2 返回状态码: %d", resp.StatusCode())
	}

	var etherscanResp EtherscanResponse
	err = json.Cjson.Unmarshal(resp.Body(), &etherscanResp)
	if err != nil {
		return nil, fmt.Errorf("解析 Etherscan API V2 响应失败: %w", err)
	}

	// 如果 API 返回错误，返回空数组而不是错误，降级处理
	if etherscanResp.Status != "1" {
		// 如果是速率限制错误，稍微延迟一下
		if etherscanResp.Message == "NOTOK" {
			time.Sleep(time.Second * 2)
		}
		return []blockchain.Transaction{}, nil
	}

	transactions := make([]blockchain.Transaction, 0)
	for _, transfer := range etherscanResp.Result {
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

		// 转换金额，ERC20 USDT是6位小数
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
			ContractAddress: USDTContractAddressERC20,
		}
		transactions = append(transactions, tx)
	}

	return transactions, nil
}

func init() {
	// 注册ERC20服务
	blockchain.RegisterChainService(NewERC20Service())
}
