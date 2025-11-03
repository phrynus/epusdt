package solana

import (
	"context"
	"fmt"
	"regexp"

	"github.com/assimon/luuu/blockchain"
	"github.com/assimon/luuu/config"
	"github.com/assimon/luuu/model/mdb"
	"github.com/assimon/luuu/util/log"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

const (
	// USDT on Solana (SPL Token)
	USDTMintAddressSolana = "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB"
)

type SolanaService struct {
	rpcClient *rpc.Client
}

func NewSolanaService() *SolanaService {
	rpcEndpoint := config.GetSolanaRpcEndpoint()
	if rpcEndpoint == "" {
		rpcEndpoint = rpc.MainNetBeta_RPC // 默认使用主网
	}

	return &SolanaService{
		rpcClient: rpc.New(rpcEndpoint),
	}
}

func (s *SolanaService) GetChainType() string {
	return mdb.ChainTypeSOLANA
}

func (s *SolanaService) GetUSDTContractAddress() string {
	return USDTMintAddressSolana
}

func (s *SolanaService) ValidateAddress(address string) bool {
	// Solana地址是Base58编码，通常32-44个字符
	match, _ := regexp.MatchString(`^[1-9A-HJ-NP-Za-km-z]{32,44}$`, address)
	return match
}

func (s *SolanaService) GetTransactions(address string, startTime int64, endTime int64) ([]blockchain.Transaction, error) {
	ctx := context.Background()

	// 解析地址
	pubKey, err := solana.PublicKeyFromBase58(address)
	if err != nil {
		return nil, fmt.Errorf("无效的 Solana 地址: %w", err)
	}

	usdtMint := solana.MustPublicKeyFromBase58("Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB")

	// 获取 ATA 地址
	ata, _, err := solana.FindAssociatedTokenAddress(
		pubKey,
		usdtMint,
	)
	if err != nil {
		return nil, fmt.Errorf("获取关联Token地址失败: %w", err)
	}
	// log.Sugar.Debugf("[%s] 查找订单: 地址=%s, 金额=%.4f", chainType, address, tx.Amount)
	log.Sugar.Debugf("[SOLANA] %s 的关联Token地址: %s", address, ata)

	// 获取签名列表，最近的交易
	sigs, err := s.rpcClient.GetSignaturesForAddress(ctx, ata)
	if err != nil {
		return nil, fmt.Errorf("获取签名失败: %w", err)
	}

	transactions := make([]blockchain.Transaction, 0)

	// 遍历签名
	for _, sig := range sigs {
		// 检查时间范围
		if sig.BlockTime == nil {
			continue
		}

		blockTimeMs := int64(*sig.BlockTime) * 1000
		if blockTimeMs < startTime || blockTimeMs > endTime {
			continue
		}

		// 检查交易状态
		if sig.Err != nil {
			continue // 跳过失败的交易
		}

		// 获取交易详情
		tx, err := s.rpcClient.GetTransaction(ctx, sig.Signature, &rpc.GetTransactionOpts{
			Encoding:                       solana.EncodingBase64,
			MaxSupportedTransactionVersion: nil, // 使用默认版本
		})
		if err != nil {
			continue
		}

		if tx == nil || tx.Meta == nil {
			continue
		}

		// 简化的Token转账检测
		transaction := s.parseTokenTransfer(tx, pubKey, sig.Signature.String(), blockTimeMs)
		if transaction != nil {
			transactions = append(transactions, *transaction)
		}
	}

	return transactions, nil
}

// parseTokenTransfer 解析SPL Token转账，简化版本
func (s *SolanaService) parseTokenTransfer(tx *rpc.GetTransactionResult, targetAddr solana.PublicKey, txHash string, blockTime int64) *blockchain.Transaction {
	if tx.Meta == nil {
		return nil
	}

	// 检查是否有Token余额变化
	if tx.Meta.PostTokenBalances == nil || tx.Meta.PreTokenBalances == nil {
		return nil
	}

	// 查找USDT相关的余额变化
	for _, postBalance := range tx.Meta.PostTokenBalances {
		// 检查是否是USDT Mint地址
		if postBalance.Mint.String() != USDTMintAddressSolana {
			continue
		}

		// 查找对应的前置余额
		var preAmount float64 = 0
		for _, preBalance := range tx.Meta.PreTokenBalances {
			if preBalance.AccountIndex == postBalance.AccountIndex &&
				preBalance.Mint.String() == USDTMintAddressSolana {
				if preBalance.UiTokenAmount.UiAmount != nil {
					preAmount = *preBalance.UiTokenAmount.UiAmount
				}
				break
			}
		}

		var postAmount float64 = 0
		if postBalance.UiTokenAmount.UiAmount != nil {
			postAmount = *postBalance.UiTokenAmount.UiAmount
		}

		// 如果余额增加，说明是接收到的转账
		if postAmount > preAmount {
			diff := postAmount - preAmount

			return &blockchain.Transaction{
				Hash:            txHash,
				From:            "solana_sender", // Solana发送者地址解析复杂，暂时用占位符
				To:              targetAddr.String(),
				Amount:          diff,
				BlockTimestamp:  blockTime,
				Confirmations:   1, // Solana确认很快
				Status:          "SUCCESS",
				ContractAddress: USDTMintAddressSolana,
			}
		}
	}

	return nil
}

func init() {
	// 注册Solana服务，RPC端点在实际使用时从配置读取
	blockchain.RegisterChainService(NewSolanaService())
}
