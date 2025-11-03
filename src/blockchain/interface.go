package blockchain

import "sync"

// Transaction 通用交易结构
type Transaction struct {
	Hash            string  // 交易哈希
	From            string  // 发送地址
	To              string  // 接收地址
	Amount          float64 // 金额，已转换为USDT
	BlockTimestamp  int64   // 区块时间戳，毫秒
	Confirmations   int     // 确认数
	Status          string  // 交易状态
	ContractAddress string  // 合约地址，代币
}

// ChainService 区块链服务接口
type ChainService interface {
	// GetChainType 获取链类型
	GetChainType() string

	// GetTransactions 获取地址的交易记录
	GetTransactions(address string, startTime int64, endTime int64) ([]Transaction, error)

	// GetUSDTContractAddress 获取USDT合约地址
	GetUSDTContractAddress() string

	// ValidateAddress 验证地址格式
	ValidateAddress(address string) bool
}

// Factory 链服务工厂
type Factory struct {
	services map[string]ChainService
	mu       sync.RWMutex
}

var defaultFactory *Factory

func init() {
	defaultFactory = &Factory{
		services: make(map[string]ChainService),
	}
}

// RegisterChainService 注册链服务
func RegisterChainService(service ChainService) {
	defaultFactory.mu.Lock()
	defer defaultFactory.mu.Unlock()
	defaultFactory.services[service.GetChainType()] = service
}

// GetChainService 获取链服务
func GetChainService(chainType string) ChainService {
	defaultFactory.mu.RLock()
	defer defaultFactory.mu.RUnlock()
	return defaultFactory.services[chainType]
}

// GetAllChainTypes 获取所有注册的链类型
func GetAllChainTypes() []string {
	defaultFactory.mu.RLock()
	defer defaultFactory.mu.RUnlock()

	chainTypes := make([]string, 0, len(defaultFactory.services))
	for chainType := range defaultFactory.services {
		chainTypes = append(chainTypes, chainType)
	}
	return chainTypes
}
