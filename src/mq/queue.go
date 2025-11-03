package mq

import (
	"context"

	"github.com/assimon/luuu/model/dao"
	"github.com/assimon/luuu/mq/handle"
	"github.com/assimon/luuu/util/log"
)

var queueCtx context.Context
var queueCancel context.CancelFunc

func Start() {
	// 注册任务处理器
	dao.RegisterTaskHandler(handle.QueueOrderExpiration, handle.OrderExpirationHandle)
	dao.RegisterTaskHandler(handle.QueueOrderExpirationCallback, handle.OrderExpirationCallbackHandle)
	dao.RegisterTaskHandler(handle.QueueOrderCallback, handle.OrderCallbackHandle)

	// 启动队列处理器
	queueCtx, queueCancel = context.WithCancel(context.Background())
	go dao.ProcessQueue(queueCtx, "critical")
	go dao.ProcessQueue(queueCtx, "default")
	go dao.ProcessQueue(queueCtx, "low")

	log.Sugar.Info("[队列] 队列处理器已启动")
}

func Stop() {
	if queueCancel != nil {
		queueCancel()
		log.Sugar.Info("[队列] 队列处理器已停止")
	}
}
