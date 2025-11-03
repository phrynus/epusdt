package dao

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/assimon/luuu/util/log"
)

const (
	QueueStatusPending    = 0
	QueueStatusProcessing = 1
	QueueStatusCompleted  = 2
	QueueStatusFailed     = 3
)

type QueueJob struct {
	ID          int64
	QueueName   string
	TaskType    string
	Payload     string
	MaxRetry    int
	RetryCount  int
	Status      int
	ScheduleAt  time.Time
	ProcessedAt sql.NullTime
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type TaskHandler func(ctx context.Context, payload []byte) error

var taskHandlers = make(map[string]TaskHandler)

// RegisterTaskHandler 注册任务处理器
func RegisterTaskHandler(taskType string, handler TaskHandler) {
	taskHandlers[taskType] = handler
}

// EnqueueTask 将任务加入队列
func EnqueueTask(ctx context.Context, queueName, taskType string, payload interface{}, scheduleAt time.Time, maxRetry int) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	if maxRetry <= 0 {
		maxRetry = 3
	}

	query := `INSERT INTO queue_jobs (queue_name, task_type, payload, max_retry, schedule_at)
			  VALUES (?, ?, ?, ?, ?)`

	return Mdb.WithContext(ctx).Exec(query, queueName, taskType, string(payloadBytes), maxRetry, scheduleAt).Error
}

// EnqueueTaskNow 立即将任务加入队列
func EnqueueTaskNow(ctx context.Context, queueName, taskType string, payload interface{}, maxRetry int) error {
	return EnqueueTask(ctx, queueName, taskType, payload, time.Now(), maxRetry)
}

// EnqueueTaskDelay 延迟将任务加入队列
func EnqueueTaskDelay(ctx context.Context, queueName, taskType string, payload interface{}, delay time.Duration, maxRetry int) error {
	return EnqueueTask(ctx, queueName, taskType, payload, time.Now().Add(delay), maxRetry)
}

// FetchPendingJob 获取待处理的任务
func FetchPendingJob(ctx context.Context, queueName string) (*QueueJob, error) {
	tx := Mdb.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 查找待处理的任务
	var job QueueJob
	query := `SELECT id, queue_name, task_type, payload, max_retry, retry_count, status, schedule_at, processed_at, created_at, updated_at
			  FROM queue_jobs 
			  WHERE queue_name = ? AND status = ? AND schedule_at <= ?
			  ORDER BY id ASC 
			  LIMIT 1`

	err := tx.Raw(query, queueName, QueueStatusPending, time.Now()).Scan(&job).Error
	if err != nil {
		tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if job.ID == 0 {
		tx.Rollback()
		return nil, nil
	}

	// 更新状态为处理中
	updateQuery := `UPDATE queue_jobs SET status = ? WHERE id = ?`
	err = tx.Exec(updateQuery, QueueStatusProcessing, job.ID).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return &job, nil
}

// MarkJobCompleted 标记任务完成
func MarkJobCompleted(ctx context.Context, jobID int64) error {
	query := `UPDATE queue_jobs 
			  SET status = ?, processed_at = NOW() 
			  WHERE id = ?`
	return Mdb.WithContext(ctx).Exec(query, QueueStatusCompleted, jobID).Error
}

// MarkJobFailed 标记任务失败并重试
func MarkJobFailed(ctx context.Context, jobID int64) error {
	tx := Mdb.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获取当前任务信息
	var job QueueJob
	query := `SELECT id, retry_count, max_retry FROM queue_jobs WHERE id = ?`
	err := tx.Raw(query, jobID).Scan(&job).Error
	if err != nil {
		tx.Rollback()
		return err
	}

	newRetryCount := job.RetryCount + 1
	var newStatus int
	var scheduleAt time.Time

	if newRetryCount >= job.MaxRetry {
		// 达到最大重试次数，标记为失败
		newStatus = QueueStatusFailed
	} else {
		// 重新加入队列，延迟重试
		newStatus = QueueStatusPending
		// 延迟时间：重试次数 * 10 秒
		scheduleAt = time.Now().Add(time.Duration(newRetryCount) * 10 * time.Second)
	}

	updateQuery := `UPDATE queue_jobs 
					SET status = ?, retry_count = ?, schedule_at = ? 
					WHERE id = ?`
	err = tx.Exec(updateQuery, newStatus, newRetryCount, scheduleAt, jobID).Error
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// ProcessQueue 处理队列任务
func ProcessQueue(ctx context.Context, queueName string) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	log.Sugar.Infof("[queue] Starting queue processor for: %s", queueName)

	for {
		select {
		case <-ctx.Done():
			log.Sugar.Infof("[queue] Stopping queue processor for: %s", queueName)
			return
		case <-ticker.C:
			job, err := FetchPendingJob(ctx, queueName)
			if err != nil {
				log.Sugar.Errorf("[queue] Error fetching job from queue %s: %v", queueName, err)
				continue
			}

			if job == nil {
				continue
			}

			// 处理任务
			handler, exists := taskHandlers[job.TaskType]
			if !exists {
				log.Sugar.Errorf("[queue] No handler found for task type: %s", job.TaskType)
				MarkJobFailed(ctx, job.ID)
				continue
			}

			err = handler(ctx, []byte(job.Payload))
			if err != nil {
				log.Sugar.Errorf("[queue] Error processing job %d: %v", job.ID, err)
				MarkJobFailed(ctx, job.ID)
			} else {
				log.Sugar.Infof("[queue] Successfully processed job %d", job.ID)
				MarkJobCompleted(ctx, job.ID)
			}
		}
	}
}

// CleanCompletedJobs 清理已完成的任务（保留最近N天的记录）
func CleanCompletedJobs(ctx context.Context, daysToKeep int) (int64, error) {
	if daysToKeep <= 0 {
		daysToKeep = 7
	}

	cutoffTime := time.Now().AddDate(0, 0, -daysToKeep)
	query := `DELETE FROM queue_jobs WHERE status IN (?, ?) AND updated_at < ?`
	result := Mdb.WithContext(ctx).Exec(query, QueueStatusCompleted, QueueStatusFailed, cutoffTime)
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}
