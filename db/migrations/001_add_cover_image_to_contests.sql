-- 为 contests 表添加封面图字段
-- 执行时间: 2026-02-12

ALTER TABLE contests ADD COLUMN cover_image VARCHAR(1024) COMMENT '比赛封面图路径' AFTER description;
