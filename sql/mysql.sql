-- MySQL 数据库结构
-- 从 SQLite 迁移到 MySQL

-- 订单表（已整合 chain_type 字段及索引）
CREATE TABLE IF NOT EXISTS `orders` (
  `id` BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `trade_id` VARCHAR(32) NOT NULL COMMENT 'epusdt订单号',
  `order_id` VARCHAR(32) NOT NULL COMMENT '客户交易id',
  `block_transaction_id` VARCHAR(128) DEFAULT NULL COMMENT '区块唯一编号',
  `actual_amount` DECIMAL(20,8) NOT NULL COMMENT '订单实际需要支付的金额（USDT）',
  `amount` DECIMAL(20,8) NOT NULL COMMENT '订单金额（人民币）',
  `token` VARCHAR(50) NOT NULL COMMENT '所属钱包地址',
  `chain_type` VARCHAR(20) NOT NULL DEFAULT 'TRC20' COMMENT '链类型（TRC20, ERC20, BEP20, SOLANA, POLYGON）',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '1=等待支付, 2=支付成功, 3=已过期',
  `notify_url` VARCHAR(128) NOT NULL COMMENT '异步回调地址',
  `redirect_url` VARCHAR(128) DEFAULT NULL COMMENT '同步回调地址',
  `callback_num` INT NOT NULL DEFAULT 0 COMMENT '回调次数',
  `callback_confirm` TINYINT NOT NULL DEFAULT 2 COMMENT '回调是否已确认（1=是, 2=否）',
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` TIMESTAMP NULL DEFAULT NULL,
  UNIQUE KEY `orders_order_id_uindex` (`order_id`),
  UNIQUE KEY `orders_trade_id_uindex` (`trade_id`),
  KEY `orders_block_transaction_id_index` (`block_transaction_id`),
  KEY `idx_chain_type` (`chain_type`),
  KEY `idx_orders_status` (`status`),
  KEY `idx_orders_created_at` (`created_at`),
  KEY `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='订单表';

-- 钱包地址表（已整合 chain_type 字段及联合索引）
CREATE TABLE IF NOT EXISTS `wallet_address` (
  `id` BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `token` VARCHAR(50) NOT NULL COMMENT '钱包地址',
  `chain_type` VARCHAR(20) NOT NULL DEFAULT 'TRC20' COMMENT '链类型（TRC20, ERC20, BEP20, SOLANA, POLYGON）',
  `remark` VARCHAR(100) DEFAULT NULL COMMENT '备注名称',
  `balance` DECIMAL(20,8) DEFAULT 0.00000000 COMMENT 'USDT余额',
  `balance_updated_at` TIMESTAMP NULL DEFAULT NULL COMMENT '余额更新时间',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '1=启用, 2=禁用',
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` TIMESTAMP NULL DEFAULT NULL,
  KEY `wallet_address_token_index` (`token`),
  KEY `idx_token_chain_type` (`token`, `chain_type`),
  KEY `idx_wallet_status` (`status`),
  KEY `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='钱包地址表';

-- 缓存表（替代 Redis 缓存）
CREATE TABLE IF NOT EXISTS `cache` (
  `id` BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `cache_key` VARCHAR(255) NOT NULL COMMENT '缓存键',
  `cache_value` TEXT NOT NULL COMMENT '缓存值',
  `expires_at` TIMESTAMP NULL DEFAULT NULL COMMENT '过期时间（NULL 表示永不过期）',
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY `idx_cache_key` (`cache_key`),
  KEY `idx_cache_expires` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='缓存表';

-- 队列表（替代 Redis 队列）
CREATE TABLE IF NOT EXISTS `queue_jobs` (
  `id` BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `queue_name` VARCHAR(50) NOT NULL COMMENT '队列名称（critical, default, low）',
  `task_type` VARCHAR(100) NOT NULL COMMENT '任务类型',
  `payload` TEXT NOT NULL COMMENT '任务数据（JSON 格式）',
  `max_retry` INT NOT NULL DEFAULT 3 COMMENT '最大重试次数',
  `retry_count` INT NOT NULL DEFAULT 0 COMMENT '当前重试次数',
  `status` TINYINT NOT NULL DEFAULT 0 COMMENT '0=待处理, 1=处理中, 2=已完成, 3=失败',
  `schedule_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '计划执行时间',
  `processed_at` TIMESTAMP NULL DEFAULT NULL COMMENT '处理时间',
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY `idx_queue_status_schedule` (`status`, `schedule_at`),
  KEY `idx_queue_name` (`queue_name`),
  KEY `idx_queue_task_type` (`task_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='队列任务表';

