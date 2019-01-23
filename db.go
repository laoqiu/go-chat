package gochat

import (
	"github.com/jmoiron/sqlx"
	"github.com/laoqiu/sqlxt"
)

var schemas = []string{
	// 用户表
	`CREATE TABLE IF NOT EXISTS users (
		id VARCHAR(45) NOT NULL COMMENT '用户唯一标识',
		name VARCHAR(45) NOT NULL COMMENT '昵称',
		created DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '注册时间',
		updated DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
		PRIMARY KEY (id)
	);`,
	// 用户登录记录
	`CREATE TABLE IF NOT EXISTS user_login_logs (
		id INT(11) NOT NULL AUTO_INCREMENT,
		user_id VARCHAR(45) NOT NULL COMMENT '用户名',
		platform VARCHAR(20) NOT NULL COMMENT '平台',
		ip VARCHAR(15) NOT NULL COMMENT 'ip地址',
		browser_info VARCHAR(200) DEFAULT '' COMMENT '浏览器信息',
		os_info VARCHAR(200) DEFAULT '' COMMENT '操作系统信息',
		created DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '登录时间',
		PRIMARY KEY (id)
	);`,
	// 用户状态
	`CREATE TABLE IF NOT EXISTS user_status (
		id INT(11) NOT NULL AUTO_INCREMENT,
		user_id VARCHAR(45) NOT NULL COMMENT '用户名',
		platform VARCHAR(20) NOT NULL COMMENT '平台',
		is_online TINYINT(1) DEFAULT 0 COMMENT '当前在线',
		created DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '登录/登出时间',
		PRIMARY KEY (id),
		UNIQUE KEY user_UNIQUE (user_id, platform)
	);`,
	// 组
	`CREATE TABLE IF NOT EXISTS chatgroup (
		id INT(11) NOT NULL AUTO_INCREMENT,
		name VARCHAR(45) NOT NULL COMMENT '组名',
		description VARCHAR(200) DEFAULT '' COMMENT '群组描述',
		owner VARCHAR(45) NOT NULL COMMENT '创建者/群主',
		public TINYINT(1) DEFAULT 1 COMMENT '是否公开，默认是',
		maxmembers INT(11) DEFAULT 50 COMMENT '群成员上限',
		created DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		PRIMARY KEY (id)
	);`,
	// 组成员表
	`CREATE TABLE IF NOT EXISTS chatgroup_members (
		id INT(11) NOT NULL AUTO_INCREMENT,
		group_id INT(11) NOT NULL COMMENT '组id',
		member VARCHAR(45) NOT NULL COMMENT '成员名称',
		is_manager TINYINT(1) DEFAULT 0 COMMENT '是否管理员，默认否',
		created DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		PRIMARY KEY (id),
		INDEX group_id_IDX (group_id ASC)
	);`,
}

func Init(opts ...sqlxt.Option) (*sqlx.DB, error) {
	o := sqlxt.NewOptions(opts...)
	db, err := sqlxt.Connect(o.Driver, o.URI, o.Charset, o.ParseTime, o.MaxClient, o.MaxClient)
	if err != nil {
		return db, err
	}
	// 加载mapper
	sqlxt.LoadMapper(db, sqlxt.DefaultMapper)
	// 初始化表结构
	for _, s := range schemas {
		_, err = db.Exec(s)
		if err != nil {
			return db, err
		}
	}
	return db, nil
}
