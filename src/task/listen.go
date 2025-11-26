package task

import (
	"fmt"
	"time"

	"github.com/assimon/luuu/config"
	"github.com/assimon/luuu/model/mdb"
	"github.com/assimon/luuu/util/log"
	"github.com/robfig/cron/v3"
)

func Start() {
	c := cron.New()

	log.Sugar.Info("启动区块链监控任务...")

	// 获取配置的监听间隔
	listenInterval := config.GetBlockchainListenInterval()
	cronExpr := fmt.Sprintf("@every %ds", listenInterval)

	// 汇率监听
	c.AddJob("@every 60s", UsdtRateJob{})
	log.Sugar.Info("USDT汇率监控已启动，每60秒执行")

	// TRC20，波场钱包监听
	c.AddJob(cronExpr, NewListenBlockchainJob(mdb.ChainTypeTRC20))
	log.Sugar.Infof("TRC20监控已启动，每%d秒执行", listenInterval)
	time.Sleep(1 * time.Second)

	// ERC20，以太坊钱包监听
	c.AddJob(cronExpr, NewListenBlockchainJob(mdb.ChainTypeERC20))
	log.Sugar.Infof("ERC20监控已启动，每%d秒执行", listenInterval)
	time.Sleep(1 * time.Second)

	// BEP20，币安智能链钱包监听
	c.AddJob(cronExpr, NewListenBlockchainJob(mdb.ChainTypeBEP20))
	log.Sugar.Infof("BEP20监控已启动，每%d秒执行", listenInterval)
	time.Sleep(1 * time.Second)

	// Polygon钱包监听
	c.AddJob(cronExpr, NewListenBlockchainJob(mdb.ChainTypePOLYGON))
	log.Sugar.Infof("Polygon监控已启动，每%d秒执行", listenInterval)
	time.Sleep(1 * time.Second)

	// ARB钱包监听
	c.AddJob(cronExpr, NewListenBlockchainJob(mdb.ChainTypeARB))
	log.Sugar.Infof("ARB监控已启动，每%d秒执行", listenInterval)
	time.Sleep(1 * time.Second)

	// Solana钱包监听
	c.AddJob(cronExpr, NewListenBlockchainJob(mdb.ChainTypeSOLANA))
	log.Sugar.Infof("Solana监控已启动，每%d秒执行", listenInterval)

	// 定时清理过期缓存（每5分钟执行一次）
	c.AddJob("@every 5m", CleanCacheJob{})
	log.Sugar.Info("缓存清理任务已启动，每5分钟执行")

	// 定时清理已完成队列任务（每6小时执行一次，保留7天数据）
	c.AddJob("@every 6h", CleanQueueJob{})
	log.Sugar.Info("队列清理任务已启动，每6小时执行")

	// 启动时立即执行一次全面清理
	go func() {
		log.Sugar.Info("执行启动时数据清理...")
		cleanJob := CleanJob{}
		cleanJob.Run()
	}()

	c.Start()
	log.Sugar.Info("所有定时任务运行中")
}
