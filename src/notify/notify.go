package notify

import (
	"github.com/assimon/luuu/config"
	"github.com/assimon/luuu/util/log"
	tb "gopkg.in/telebot.v3"
)

var bot *tb.Bot

// SetBot 设置机器人实例
func SetBot(b *tb.Bot) {
	bot = b
}

// SendToBot 主动发送消息到机器人
func SendToBot(msg string) {
	if bot == nil {
		return
	}

	go func() {
		user := tb.User{
			ID: config.TgManage,
		}
		_, err := bot.Send(&user, msg, &tb.SendOptions{
			ParseMode: tb.ModeHTML,
		})
		if err != nil {
			log.Sugar.Error(err)
		}
	}()
}

