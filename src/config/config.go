package config

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
)

var (
	AppDebug                 bool
	LogDebug                 bool // 日志是否输出到控制台
	BlockchainListenInterval int  // 区块链监听间隔（秒）
	MysqlHost                string
	MysqlPort                string
	MysqlUser                string
	MysqlPassword            string
	MysqlDatabase            string
	RuntimePath              string
	LogSavePath              string
	StaticPath               string
	TgBotToken               string
	TgProxy                  string
	TgManage                 int64
	UsdtRate                 float64
	EtherscanApiKey          string
	BscScanApiKey            string // 已弃用，请使用 EtherscanApiKey，Etherscan API V2 支持多链
	SolanaRpcEndpoint        string
)

func Init() {
	viper.AddConfigPath("./")
	viper.SetConfigFile(".env")
	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}
	gwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	AppDebug = viper.GetBool("app_debug")
	LogDebug = viper.GetBool("log_debug")
	BlockchainListenInterval = viper.GetInt("blockchain_listen_interval")
	if BlockchainListenInterval <= 0 {
		BlockchainListenInterval = 10 // 默认10秒
	}
	StaticPath = viper.GetString("static_path")
	RuntimePath = fmt.Sprintf(
		"%s%s",
		gwd,
		viper.GetString("runtime_root_path"))
	LogSavePath = fmt.Sprintf(
		"%s%s",
		RuntimePath,
		viper.GetString("log_save_path"))
	// MySQL 数据库配置
	MysqlHost = viper.GetString("mysql_host")
	if MysqlHost == "" {
		MysqlHost = "127.0.0.1"
	}
	MysqlPort = viper.GetString("mysql_port")
	if MysqlPort == "" {
		MysqlPort = "3306"
	}
	MysqlUser = viper.GetString("mysql_user")
	if MysqlUser == "" {
		MysqlUser = "root"
	}
	MysqlPassword = viper.GetString("mysql_password")
	MysqlDatabase = viper.GetString("mysql_database")
	if MysqlDatabase == "" {
		MysqlDatabase = "epusdt"
	}
	TgBotToken = viper.GetString("tg_bot_token")
	TgProxy = viper.GetString("tg_proxy")
	TgManage = viper.GetInt64("tg_manage")
	EtherscanApiKey = viper.GetString("etherscan_api_key")
	BscScanApiKey = viper.GetString("bscscan_api_key")
	SolanaRpcEndpoint = viper.GetString("solana_rpc_endpoint")
	fmt.Println(SolanaRpcEndpoint)
}

func GetAppVersion() string {
	return "0.0.2"
}

func GetAppName() string {
	appName := viper.GetString("app_name")
	if appName == "" {
		return "epusdt"
	}
	return appName
}

func GetAppUri() string {
	return viper.GetString("app_uri")
}

func GetApiAuthToken() string {
	return viper.GetString("api_auth_token")
}

func GetUsdtRate() float64 {
	forcedUsdtRate := viper.GetFloat64("forced_usdt_rate")
	if forcedUsdtRate > 0 {
		return forcedUsdtRate
	}
	if UsdtRate <= 0 {
		return 6.4
	}
	return UsdtRate
}

func GetOrderExpirationTime() int {
	timer := viper.GetInt("order_expiration_time")
	if timer <= 0 {
		return 10
	}
	return timer
}

func GetOrderExpirationTimeDuration() time.Duration {
	timer := GetOrderExpirationTime()
	return time.Minute * time.Duration(timer)
}

func GetEtherscanApiKey() string {
	return EtherscanApiKey
}

func GetBscScanApiKey() string {
	// 优先返回 BscScanApiKey（向后兼容），如果未配置则返回 EtherscanApiKey
	// 因为 Etherscan API V2 支持多链，一个密钥可用于 BSC 和 Ethereum
	if BscScanApiKey != "" {
		return BscScanApiKey
	}
	return EtherscanApiKey
}

func GetSolanaRpcEndpoint() string {
	if SolanaRpcEndpoint == "" {
		return "https://api.mainnet-beta.solana.com" // 默认 Solana 主网
	}
	return SolanaRpcEndpoint
}

// GetBlockchainListenInterval 获取区块链监听间隔
func GetBlockchainListenInterval() int {
	if BlockchainListenInterval <= 0 {
		return 10 // 默认10秒
	}
	return BlockchainListenInterval
}
