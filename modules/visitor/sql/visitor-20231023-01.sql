-- +migrate Up

-- visitor 游客信息
CREATE TABLE IF NOT EXISTS `visitor` (
  `id` int NOT NULL AUTO_INCREMENT COMMENT '表主键',
  `uuid` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT '' COMMENT 'UUID',
  `comment` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT '' COMMENT '备注',
  `status` tinyint NOT NULL DEFAULT '0' COMMENT '状态:0离线，1在线，2拉黑',
  `name` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT '' COMMENT '游客名',
  `ip` varchar(50) NOT NULL DEFAULT '' COMMENT 'IP',
  `referer` varchar(500) NOT NULL DEFAULT '' COMMENT '来源，从哪个网址发起的聊天',
  `region` varchar(100) NOT NULL DEFAULT '' COMMENT '地区',
  `city` varchar(100) NOT NULL DEFAULT '' COMMENT '城市',
  `platform` varchar(100) NOT NULL DEFAULT '' COMMENT '平台:移动设备,电脑,平板,Xbox',
  `device` varchar(100) NOT NULL DEFAULT '' COMMENT '设备:移动设备,电脑,平板,Xbox',
  `os` varchar(100) NOT NULL DEFAULT '' COMMENT '系统: andriod,ios,macOS,windows',
  `browser` varchar(100) NOT NULL DEFAULT '' COMMENT '浏览器: Chrome,Safari,InternalExplorer,Firefox,Edge',
  `language` varchar(100) NOT NULL DEFAULT '' COMMENT '语言',
  `last_login_at` datetime NOT NULL DEFAULT '2005-01-02 15:04:05' COMMENT '最后一次登录时间',
  `create_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `update_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_uuid` (`uuid`),
  KEY `idx_status` (`status`),
  KEY `idx_ip` (`ip`),
  KEY `idx_last_login_at` (`last_login_at`)
);
