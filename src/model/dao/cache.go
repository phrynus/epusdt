package dao

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var (
	ErrCacheNotFound = errors.New("cache: key not found")
)

// CacheGet 获取缓存
func CacheGet(ctx context.Context, key string) (string, error) {
	var value string
	var expiresAt sql.NullTime

	query := `SELECT cache_value, expires_at FROM cache WHERE cache_key = ? LIMIT 1`
	err := Mdb.WithContext(ctx).Raw(query, key).Row().Scan(&value, &expiresAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrCacheNotFound
		}
		return "", err
	}

	// 检查是否过期
	if expiresAt.Valid && expiresAt.Time.Before(time.Now()) {
		// 删除过期缓存
		CacheDel(ctx, key)
		return "", ErrCacheNotFound
	}

	return value, nil
}

// CacheSet 设置缓存
func CacheSet(ctx context.Context, key, value string, expiration time.Duration) error {
	// 先尝试更新，如果不存在则插入 (MySQL 语法)
	// 使用 MySQL 服务器端计算过期时间，避免时区问题
	var query string
	var args []interface{}

	if expiration > 0 {
		// 将 expiration 转换为秒数，让 MySQL 服务器端计算过期时间
		seconds := int(expiration.Seconds())
		query = `INSERT INTO cache (cache_key, cache_value, expires_at, updated_at) 
				 VALUES (?, ?, DATE_ADD(NOW(), INTERVAL ? SECOND), CURRENT_TIMESTAMP)
				 ON DUPLICATE KEY UPDATE 
				 cache_value = VALUES(cache_value),
				 expires_at = VALUES(expires_at),
				 updated_at = CURRENT_TIMESTAMP`
		args = []interface{}{key, value, seconds}
	} else {
		// 永不过期
		query = `INSERT INTO cache (cache_key, cache_value, expires_at, updated_at) 
				 VALUES (?, ?, NULL, CURRENT_TIMESTAMP)
				 ON DUPLICATE KEY UPDATE 
				 cache_value = VALUES(cache_value),
				 expires_at = VALUES(expires_at),
				 updated_at = CURRENT_TIMESTAMP`
		args = []interface{}{key, value}
	}

	return Mdb.WithContext(ctx).Exec(query, args...).Error
}

// CacheDel 删除缓存
func CacheDel(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	// 单个key直接删除
	if len(keys) == 1 {
		query := `DELETE FROM cache WHERE cache_key = ?`
		return Mdb.WithContext(ctx).Exec(query, keys[0]).Error
	}

	// 多个key使用IN语句（使用GORM的Where方法）
	return Mdb.WithContext(ctx).Where("cache_key IN ?", keys).Delete(&struct{ CacheKey string }{}).Error
}

// CacheCleanExpired 清理过期缓存
func CacheCleanExpired(ctx context.Context) (int64, error) {
	query := `DELETE FROM cache WHERE expires_at IS NOT NULL AND expires_at < NOW()`
	result := Mdb.WithContext(ctx).Exec(query)
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// CacheExists 检查缓存是否存在
func CacheExists(ctx context.Context, key string) (bool, error) {
	var count int64
	query := `SELECT COUNT(*) FROM cache WHERE cache_key = ? AND (expires_at IS NULL OR expires_at > NOW())`
	err := Mdb.WithContext(ctx).Raw(query, key).Row().Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
