package mdb

import "github.com/golang-module/carbon/v2"

const (
	TokenStatusEnable  = 1
	TokenStatusDisable = 2
)

// 链类型常量
const (
	ChainTypeTRC20   = "TRC20"   // 波场
	ChainTypeERC20   = "ERC20"   // 以太坊
	ChainTypeBEP20   = "BEP20"   // 币安智能链
	ChainTypeSOLANA  = "SOLANA"  // Solana
	ChainTypePOLYGON = "POLYGON" // Polygon
)

// WalletAddress  钱包表
type WalletAddress struct {
	Token            string       `gorm:"column:token" json:"token"`                           //  钱包地址
	ChainType        string       `gorm:"column:chain_type" json:"chain_type"`                 //  链类型: TRC20, ERC20, BEP20, SOLANA, POLYGON
	Remark           string       `gorm:"column:remark" json:"remark"`                         //  备注名称
	Balance          float64      `gorm:"column:balance" json:"balance"`                       //  USDT余额
	BalanceUpdatedAt *carbon.Time `gorm:"column:balance_updated_at" json:"balance_updated_at"` //  余额更新时间
	Status           int64        `gorm:"column:status" json:"status"`                         //  1:启用 2:禁用
	BaseModel
}

// TableName sets the insert table name for this struct type
func (w *WalletAddress) TableName() string {
	return "wallet_address"
}
