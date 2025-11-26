package bootstrap

import (
	"github.com/assimon/luuu/command"
	"github.com/assimon/luuu/config"
	"github.com/assimon/luuu/model/dao"
	"github.com/assimon/luuu/mq"
	"github.com/assimon/luuu/task"
	"github.com/assimon/luuu/telegram"
	"github.com/assimon/luuu/util/log"

	// 初始化区块链服务
	_ "github.com/assimon/luuu/blockchain/arb"
	_ "github.com/assimon/luuu/blockchain/bep20"
	_ "github.com/assimon/luuu/blockchain/erc20"
	_ "github.com/assimon/luuu/blockchain/polygon"
	_ "github.com/assimon/luuu/blockchain/solana"
	_ "github.com/assimon/luuu/blockchain/trc20"
)

// Start 服务启动
func Start() {
	// 配置加载
	config.Init()
	// 日志加载
	log.Init()
	// MySQL启动
	dao.MysqlInit()
	// 队列启动
	mq.Start()
	// telegram机器人启动
	go telegram.BotStart()
	// 定时任务
	go task.Start()
	err := command.Execute()
	if err != nil {
		panic(err)
	}
}
