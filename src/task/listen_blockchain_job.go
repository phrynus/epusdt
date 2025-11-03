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

	var wg sync.WaitGroup
	for _, address := range walletAddressList {
		wg.Add(1)
		go service.ChainCallBack(address.Token, r.ChainType, &wg)
	}
	time.Sleep(1 * time.Second)
	wg.Wait()

	log.Sugar.Debugf("[%s] 监控周期完成", r.ChainType)
}
