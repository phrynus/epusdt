-- 数据库迁移脚本：为 wallet_address 表添加余额字段
-- 执行日期：2025-11-03
-- 说明：为钱包地址添加余额查询和记录功能

-- 添加 balance 字段
ALTER TABLE `wallet_address` 
ADD COLUMN `balance` DECIMAL(20,8) DEFAULT 0.00000000 COMMENT 'USDT余额' 
AFTER `remark`;

-- 添加 balance_updated_at 字段
ALTER TABLE `wallet_address` 
ADD COLUMN `balance_updated_at` TIMESTAMP NULL DEFAULT NULL COMMENT '余额更新时间' 
AFTER `balance`;

-- 验证字段是否添加成功
SELECT id, token, chain_type, remark, balance, balance_updated_at FROM wallet_address LIMIT 5;

-- 如果需要回滚，执行以下语句：
-- ALTER TABLE `wallet_address` DROP COLUMN `balance`;
-- ALTER TABLE `wallet_address` DROP COLUMN `balance_updated_at`;

