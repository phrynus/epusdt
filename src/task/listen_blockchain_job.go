package task

import (
	"sync"
	"time"

	"github.com/assimon/luuu/model/data"
	"github.com/assimon/luuu/model/service"
	"github.com/assimon/luuu/util/log"
)

// ListenBlockchainJob 通用区块链监听任务
type ListenBlockchainJob struct {
	ChainType string
}

func NewListenBlockchainJob(chainType string) *ListenBlockchainJob {
	return &ListenBlockchainJob{
		ChainType: chainType,
	}
}

func (r *ListenBlockchainJob) Run() {
	// 获取该链类型的可用钱包地址
	walletAddressList, err := data.GetAvailableWalletAddressByChainType(r.ChainType)
	if err != nil {
		log.Sugar.Errorf("获取%s钱包地址失败: %v", r.ChainType, err)
		return
	}

	if len(walletAddressList) <= 0 {
		log.Sugar.Debugf("[%s] 未配置监控钱包地址", r.ChainType)
		return
	}

	log.Sugar.Debugf("[%s] 开始监控%d个钱包地址", r.ChainType, len(walletAddressList))

	// 筛选出有待支付订单的地址
	var activeAddresses []string
	for _, address := range walletAddressList {
		hasPendingOrder, err := data.HasPendingOrderByAddress(address.Token, r.ChainType)
		if err != nil {
			log.Sugar.Warnf("[%s] 检查地址订单状态失败: %s, err=%v", r.ChainType, address.Token, err)
			// 出错时也加入监控列表，保证不遗漏
			activeAddresses = append(activeAddresses, address.Token)
			continue
		}

		if hasPendingOrder {
			activeAddresses = append(activeAddresses, address.Token)
		} else {
			log.Sugar.Debugf("[%s] 跳过无待支付订单的地址: %s", r.ChainType, address.Token)
		}
	}

	if len(activeAddresses) == 0 {
		log.Sugar.Debugf("[%s] 当前无待支付订单需要监控", r.ChainType)
		return
	}

	log.Sugar.Infof("[%s] 筛选后需要监控%d个有订单的地址（共%d个地址）", r.ChainType, len(activeAddresses), len(walletAddressList))

	var wg sync.WaitGroup
	for _, address := range activeAddresses {
		wg.Add(1)
		go service.ChainCallBack(address, r.ChainType, &wg)
	}
	time.Sleep(1 * time.Second)
	wg.Wait()

	log.Sugar.Debugf("[%s] 监控周期完成", r.ChainType)
}
