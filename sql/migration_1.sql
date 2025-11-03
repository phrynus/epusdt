-- 数据库迁移脚本：为 wallet_address 表添加 remark 字段
-- 执行日期：2025-11-03
-- 说明：为钱包地址添加备注功能

-- 添加 remark 字段
ALTER TABLE `wallet_address` 
ADD COLUMN `remark` VARCHAR(100) DEFAULT NULL COMMENT '备注名称' 
AFTER `chain_type`;

-- 验证字段是否添加成功
-- SELECT * FROM wallet_address LIMIT 1;

-- 如果需要回滚，执行以下语句：
-- ALTER TABLE `wallet_address` DROP COLUMN `remark`;

