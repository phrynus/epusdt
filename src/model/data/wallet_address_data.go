package data

import (
	"github.com/assimon/luuu/model/dao"
	"github.com/assimon/luuu/model/mdb"
	"github.com/assimon/luuu/util/constant"
	"github.com/golang-module/carbon/v2"
)

// AddWalletAddress 创建钱包
func AddWalletAddress(token string, chainType string, remark string) (*mdb.WalletAddress, error) {
	exist, err := GetWalletAddressByTokenAndChainType(token, chainType)
	if err != nil {
		return nil, err
	}
	if exist.ID > 0 {
		return nil, constant.WalletAddressAlreadyExists
	}
	walletAddress := &mdb.WalletAddress{
		Token:     token,
		ChainType: chainType,
		Remark:    remark,
		Status:    mdb.TokenStatusEnable,
	}
	err = dao.Mdb.Create(walletAddress).Error
	return walletAddress, err
}

// GetWalletAddressByToken 通过钱包地址获取token
func GetWalletAddressByToken(token string) (*mdb.WalletAddress, error) {
	walletAddress := new(mdb.WalletAddress)
	err := dao.Mdb.Model(walletAddress).Limit(1).Find(walletAddress, "token = ?", token).Error
	return walletAddress, err
}

// GetWalletAddressByTokenAndChainType 通过钱包地址和链类型获取
func GetWalletAddressByTokenAndChainType(token string, chainType string) (*mdb.WalletAddress, error) {
	walletAddress := new(mdb.WalletAddress)
	err := dao.Mdb.Model(walletAddress).Limit(1).Find(walletAddress, "token = ? AND chain_type = ?", token, chainType).Error
	return walletAddress, err
}

// GetAvailableWalletAddressByChainType 获得指定链类型的所有可用钱包地址
func GetAvailableWalletAddressByChainType(chainType string) ([]mdb.WalletAddress, error) {
	var WalletAddressList []mdb.WalletAddress
	err := dao.Mdb.Model(WalletAddressList).Where("status = ? AND chain_type = ?", mdb.TokenStatusEnable, chainType).Find(&WalletAddressList).Error
	return WalletAddressList, err
}

// GetWalletAddressById 通过id获取钱包
func GetWalletAddressById(id uint64) (*mdb.WalletAddress, error) {
	walletAddress := new(mdb.WalletAddress)
	err := dao.Mdb.Model(walletAddress).Limit(1).Find(walletAddress, id).Error
	return walletAddress, err
}

// DeleteWalletAddressById 通过id删除钱包
func DeleteWalletAddressById(id uint64) error {
	err := dao.Mdb.Where("id = ?", id).Delete(&mdb.WalletAddress{}).Error
	return err
}

// GetAvailableWalletAddress 获得所有可用的钱包地址
func GetAvailableWalletAddress() ([]mdb.WalletAddress, error) {
	var WalletAddressList []mdb.WalletAddress
	err := dao.Mdb.Model(WalletAddressList).Where("status = ?", mdb.TokenStatusEnable).Find(&WalletAddressList).Error
	return WalletAddressList, err
}

// GetAllWalletAddress 获得所有钱包地址
func GetAllWalletAddress() ([]mdb.WalletAddress, error) {
	var WalletAddressList []mdb.WalletAddress
	err := dao.Mdb.Model(WalletAddressList).Find(&WalletAddressList).Error
	return WalletAddressList, err
}

// ChangeWalletAddressStatus 启用禁用钱包
func ChangeWalletAddressStatus(id uint64, status int) error {
	err := dao.Mdb.Model(&mdb.WalletAddress{}).Where("id = ?", id).Update("status", status).Error
	return err
}

// UpdateWalletBalance 更新钱包余额
func UpdateWalletBalance(id uint64, balance float64) error {
	updates := map[string]interface{}{
		"balance":            balance,
		"balance_updated_at": carbon.Now().ToDateTimeString(),
	}
	err := dao.Mdb.Model(&mdb.WalletAddress{}).Where("id = ?", id).Updates(updates).Error
	return err
}

// UpdateWalletBalanceByTokenAndChain 通过地址和链类型更新余额
func UpdateWalletBalanceByTokenAndChain(token string, chainType string, balance float64) error {
	updates := map[string]interface{}{
		"balance":            balance,
		"balance_updated_at": carbon.Now().ToDateTimeString(),
	}
	err := dao.Mdb.Model(&mdb.WalletAddress{}).
		Where("token = ? AND chain_type = ?", token, chainType).
		Updates(updates).Error
	return err
}
