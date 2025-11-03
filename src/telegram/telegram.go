package telegram

import (
	"time"

	"github.com/assimon/luuu/config"
	"github.com/assimon/luuu/notify"
	"github.com/assimon/luuu/util/log"
	tb "gopkg.in/telebot.v3"
	"gopkg.in/telebot.v3/middleware"
)

var bots *tb.Bot

// BotStart 机器人启动
func BotStart() {
	var err error
	botSetting := tb.Settings{
		Token:  config.TgBotToken,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	}
	if config.TgProxy != "" {
		botSetting.URL = config.TgProxy
	}
	bots, err = tb.NewBot(botSetting)
	if err != nil {
		log.Sugar.Error(err.Error())
		return
	}
	err = bots.SetCommands(Cmds)
	if err != nil {
		log.Sugar.Error(err.Error())
		return
	}
	// 设置通知bot实例
	notify.SetBot(bots)
	RegisterHandle()
	log.Sugar.Info("[Telegram] 机器人启动成功")
	bots.Start()
}

// RegisterHandle 注册处理器
func RegisterHandle() {
	adminOnly := bots.Group()
	adminOnly.Use(middleware.Whitelist(config.TgManage))

	// 注册命令处理器
	adminOnly.Handle(START_CMD, ShowWalletList)

	// 注册文本消息处理器
	adminOnly.Handle(tb.OnText, OnTextMessageHandle)

	// 注册回调处理器
	adminOnly.Handle(tb.OnCallback, OnCallbackHandle)
}
