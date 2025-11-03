package task

import (
	"context"

	"github.com/assimon/luuu/model/dao"
	"github.com/assimon/luuu/util/log"
)

// CleanCacheJob 清理过期缓存任务（高频）
type CleanCacheJob struct{}

// Run 执行缓存清理
func (j CleanCacheJob) Run() {
	ctx := context.Background()
	count, err := dao.CacheCleanExpired(ctx)
	if err != nil {
		log.Sugar.Errorf("[缓存清理] 清理过期缓存失败: %v", err)
	} else if count > 0 {
		log.Sugar.Infof("[缓存清理] 清理了 %d 条过期缓存记录", count)
	}
}

// CleanQueueJob 清理已完成的队列任务（低频）
type CleanQueueJob struct{}

// Run 执行队列清理
func (j CleanQueueJob) Run() {
	ctx := context.Background()
	// 清理已完成的队列任务（保留最近7天的记录）
	count, err := dao.CleanCompletedJobs(ctx, 7)
	if err != nil {
		log.Sugar.Errorf("[队列清理] 清理已完成队列任务失败: %v", err)
	} else if count > 0 {
		log.Sugar.Infof("[队列清理] 清理了 %d 条队列任务记录", count)
	}
}

// CleanJob 综合清理任务
type CleanJob struct{}

// Run 执行全面清理
func (j CleanJob) Run() {
	ctx := context.Background()

	// 清理过期缓存
	cacheCount, err := dao.CacheCleanExpired(ctx)
	if err != nil {
		log.Sugar.Errorf("[数据清理] 清理过期缓存失败: %v", err)
	} else {
		log.Sugar.Infof("[数据清理] 清理了 %d 条过期缓存记录", cacheCount)
	}

	// 清理已完成的队列任务（保留最近7天的记录）
	queueCount, err := dao.CleanCompletedJobs(ctx, 7)
	if err != nil {
		log.Sugar.Errorf("[数据清理] 清理已完成队列任务失败: %v", err)
	} else {
		log.Sugar.Infof("[数据清理] 清理了 %d 条队列任务记录", queueCount)
	}
}
