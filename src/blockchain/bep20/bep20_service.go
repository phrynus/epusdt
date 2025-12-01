package bep20

import (
	"fmt"
	"math/big"
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
	USDTContractAddressBEP20 = "0x55d398326f99059fF775485246999027B3197955" // USDT on BSC
	USDCContractAddressBEP20 = "0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d" // USDC on BSC
)

type BEP20Service struct {
	rateLimiter *time.Ticker
	mu          sync.Mutex
}

var (
	bep20ServiceInstance *BEP20Service
	bep20ServiceOnce     sync.Once
)

type BscScanResponse struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Result  []BEP20Transfer `json:"result"`
}

type BEP20Transfer struct {
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

func NewBEP20Service() *BEP20Service {
	bep20ServiceOnce.Do(func() {
		bep20ServiceInstance = &BEP20Service{
			// 速率限制，每1秒最多1次请求，免费API限制
			rateLimiter: time.NewTicker(time.Second * 1),
		}
	})
	return bep20ServiceInstance
}

func (s *BEP20Service) GetChainType() string {
	return mdb.ChainTypeBEP20
}

func (s *BEP20Service) GetUSDTContractAddress() string {
	return USDTContractAddressBEP20
}

func (s *BEP20Service) ValidateAddress(address string) bool {
	// BEP20地址格式与ERC20相同，以0x开头，42个字符
	match, _ := regexp.MatchString(`^0x[a-fA-F0-9]{40}$`, address)
	return match
}

func (s *BEP20Service) GetTransactions(address string, startTime int64, endTime int64) ([]blockchain.Transaction, error) {
	// 同时查询 USDT 和 USDC 交易
	usdtTxs, err := s.getTransactionsByContract(address, startTime, endTime, USDTContractAddressBEP20)
	if err != nil {
		// USDT 查询失败，记录错误但继续查询 USDC
		usdtTxs = []blockchain.Transaction{}
	}

	usdcTxs, err := s.getTransactionsByContract(address, startTime, endTime, USDCContractAddressBEP20)
	if err != nil {
		// USDC 查询失败，记录错误但继续
		usdcTxs = []blockchain.Transaction{}
	}

	// 合并交易列表
	allTxs := append(usdtTxs, usdcTxs...)
	return allTxs, nil
}

// getTransactionsByContract 查询指定合约地址的交易（使用 OnFinality RPC）
func (s *BEP20Service) getTransactionsByContract(address string, startTime int64, endTime int64, contractAddress string) ([]blockchain.Transaction, error) {
	rpcUrl := config.GetBep20RpcUrl()
	if rpcUrl == "" {
		return nil, fmt.Errorf("未配置 BEP20 RPC URL")
	}

	// 速率限制，等待令牌
	s.mu.Lock()
	<-s.rateLimiter.C
	s.mu.Unlock()

	client := http_client.GetHttpClient()

	// BSC 平均 3 秒一个块，24 小时约 28800 个块
	blockCount := int64(28800)

	// 获取最新区块号
	latestBlockResp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "eth_blockNumber",
			"params":  []interface{}{},
			"id":      1,
		}).
		Post(rpcUrl)

	if err != nil {
		return nil, fmt.Errorf("获取最新区块失败: %w", err)
	}

	var blockNumResp struct {
		Result string `json:"result"`
	}
	if err := json.Cjson.Unmarshal(latestBlockResp.Body(), &blockNumResp); err != nil {
		return nil, fmt.Errorf("解析区块号失败: %w", err)
	}

	// 将十六进制区块号转换为十进制
	latestBlock, _ := strconv.ParseInt(strings.TrimPrefix(blockNumResp.Result, "0x"), 16, 64)
	startBlock := latestBlock - blockCount

	// Transfer 事件签名
	transferEventSignature := "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"

	// 构造接收地址 topic（补齐到 32 字节）
	toAddress := "0x" + strings.Repeat("0", 24) + strings.TrimPrefix(strings.ToLower(address), "0x")

	// 使用 eth_getLogs 查询事件
	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]interface{}{
			"id":      1,
			"jsonrpc": "2.0",
			"method":  "eth_getLogs",
			"params": []interface{}{
				map[string]interface{}{
					"fromBlock": fmt.Sprintf("0x%x", startBlock),
					"toBlock":   fmt.Sprintf("0x%x", latestBlock),
					"address":   contractAddress,
					"topics": []interface{}{
						transferEventSignature,
						nil,
						toAddress,
					},
				},
			},
		}).
		Post(rpcUrl)

	if err != nil {
		return nil, fmt.Errorf("eth_getLogs 请求失败: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("RPC 返回状态码: %d, 响应: %s", resp.StatusCode(), string(resp.Body()))
	}

	// 解析 JSON-RPC 响应
	var rpcResp struct {
		Result []struct {
			BlockNumber     string   `json:"blockNumber"`
			TransactionHash string   `json:"transactionHash"`
			Topics          []string `json:"topics"`
			Data            string   `json:"data"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Cjson.Unmarshal(resp.Body(), &rpcResp); err != nil {
		return nil, fmt.Errorf("解析 RPC 响应失败: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC 错误: %s", rpcResp.Error.Message)
	}

	// 如果没有结果，返回空数组
	if len(rpcResp.Result) == 0 {
		return []blockchain.Transaction{}, nil
	}

	transactions := make([]blockchain.Transaction, 0)
	for _, log := range rpcResp.Result {
		// 解析区块号
		blockNum, _ := strconv.ParseInt(strings.TrimPrefix(log.BlockNumber, "0x"), 16, 64)

		// 获取区块信息以获取时间戳
		blockResp, err := client.R().
			SetHeader("Content-Type", "application/json").
			SetBody(map[string]interface{}{
				"jsonrpc": "2.0",
				"method":  "eth_getBlockByNumber",
				"params":  []interface{}{fmt.Sprintf("0x%x", blockNum), false},
				"id":      1,
			}).
			Post(rpcUrl)

		if err != nil {
			continue
		}

		var blockInfo struct {
			Result struct {
				Timestamp string `json:"timestamp"`
			} `json:"result"`
		}
		if err := json.Cjson.Unmarshal(blockResp.Body(), &blockInfo); err != nil {
			continue
		}

		// 解析时间戳
		timestamp, _ := strconv.ParseInt(strings.TrimPrefix(blockInfo.Result.Timestamp, "0x"), 16, 64)
		timestampMs := timestamp * 1000

		// 检查时间范围
		if timestampMs < startTime || timestampMs > endTime {
			continue
		}

		// 解析金额（data 字段）
		valueHex := strings.TrimPrefix(log.Data, "0x")
		if len(valueHex) == 0 {
			continue
		}

		// 转换十六进制金额为十进制
		valueBigInt := new(big.Int)
		if _, ok := valueBigInt.SetString(valueHex, 16); !ok {
			continue
		}

		// BEP20 USDT/USDC 都是 18 位小数
		divisor := decimal.NewFromFloat(1)
		for i := 0; i < 18; i++ {
			divisor = divisor.Mul(decimal.NewFromInt(10))
		}

		// 转换金额
		valueDecimal := decimal.NewFromBigInt(valueBigInt, 0)
		amount, _ := valueDecimal.Div(divisor).Round(4).Float64()

		// 解析发送地址（topic[1]）
		fromAddr := ""
		if len(log.Topics) > 1 {
			fromAddr = "0x" + log.Topics[1][26:]
		}

		tx := blockchain.Transaction{
			Hash:            log.TransactionHash,
			From:            fromAddr,
			To:              address,
			Amount:          amount,
			BlockTimestamp:  timestampMs,
			Confirmations:   0,
			Status:          "SUCCESS",
			ContractAddress: contractAddress,
		}
		transactions = append(transactions, tx)
	}

	return transactions, nil
}

// GetTokenBalance 获取地址的代币余额（USDT + USDC）
func (s *BEP20Service) GetTokenBalance(address string) (*blockchain.TokenBalance, error) {
	apiKey := config.GetEtherscanApiKey()
	if apiKey == "" {
		return nil, fmt.Errorf("未配置 Etherscan API 密钥")
	}

	balance := &blockchain.TokenBalance{}

	// 查询 USDT 余额
	usdtBalance, err := s.getTokenBalanceByContract(address, USDTContractAddressBEP20, apiKey)
	if err != nil {
		fmt.Println(err)
		usdtBalance = 0
	}
	balance.USDT = usdtBalance

	// 查询 USDC 余额
	usdcBalance, err := s.getTokenBalanceByContract(address, USDCContractAddressBEP20, apiKey)
	if err != nil {
		fmt.Println(err)
		usdcBalance = 0
	}
	balance.USDC = usdcBalance

	return balance, nil
}

// getTokenBalanceByContract 查询指定合约的代币余额（使用 RPC）
func (s *BEP20Service) getTokenBalanceByContract(address, contractAddress, apiKey string) (float64, error) {
	rpcUrl := config.GetBep20RpcUrl()

	// 速率限制
	s.mu.Lock()
	<-s.rateLimiter.C
	s.mu.Unlock()

	client := http_client.GetHttpClient()

	// balanceOf 函数签名: balanceOf(address)
	balanceOfSignature := "0x70a08231"
	// 地址需要补齐到 32 字节
	paddedAddress := "0x" + strings.Repeat("0", 24) + strings.TrimPrefix(strings.ToLower(address), "0x")
	data := balanceOfSignature + strings.TrimPrefix(paddedAddress, "0x")

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "eth_call",
			"params": []interface{}{
				map[string]interface{}{
					"to":   contractAddress,
					"data": data,
				},
				"latest",
			},
			"id": 1,
		}).
		Post(rpcUrl)

	if err != nil {
		return 0, fmt.Errorf("RPC 请求失败: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return 0, fmt.Errorf("RPC 返回状态码: %d, 响应: %s", resp.StatusCode(), string(resp.Body()))
	}

	var apiResp struct {
		Result string `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	err = json.Cjson.Unmarshal(resp.Body(), &apiResp)
	if err != nil {
		return 0, fmt.Errorf("解析 RPC 响应失败: %w, 响应内容: %s", err, string(resp.Body()))
	}

	if apiResp.Error != nil {
		return 0, fmt.Errorf("RPC 错误: %s", apiResp.Error.Message)
	}

	// 解析余额（十六进制转十进制）
	balanceHex := strings.TrimPrefix(apiResp.Result, "0x")
	if balanceHex == "" {
		return 0, nil
	}

	balanceBigInt := new(big.Int)
	if _, ok := balanceBigInt.SetString(balanceHex, 16); !ok {
		return 0, fmt.Errorf("无法解析余额: %s", balanceHex)
	}

	// BSC 上的 USDT 和 USDC 都是 18 位小数
	divisor := decimal.NewFromInt(1000000000000000000)
	balanceDecimal := decimal.NewFromBigInt(balanceBigInt, 0)
	balance, _ := balanceDecimal.Div(divisor).Round(4).Float64()

	return balance, nil
}

func init() {
	// 注册BEP20服务
	blockchain.RegisterChainService(NewBEP20Service())
}
