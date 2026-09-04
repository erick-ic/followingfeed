package main

import (
	"context"
	"database/sql"
	"log"
	"time"

	"followingfeed/config"
	"followingfeed/migrations"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	// 迁移命令只加载 MySQL、迁移超时和数据库锁配置，避免发布任务依赖
	// API 运行时才需要的 Redis、JWT 和 CORS 配置。
	cfg, err := config.LoadMigration()
	if err != nil {
		log.Fatalf("加载迁移配置失败：%v", err)
	}

	// sql.Open 只创建连接池，不会立即访问数据库，因此下面还需要 PingContext。
	db, err := sql.Open("mysql", cfg.MigrationDSN())
	if err != nil {
		log.Fatalf("创建 MySQL 连接池失败：%v", err)
	}
	defer db.Close()

	// 启动阶段使用较短的连接超时，让账号、网络或 DSN 错误尽快暴露。
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err = db.PingContext(pingCtx); err != nil {
		pingCancel()
		log.Fatalf("连接 MySQL 失败：%v", err)
	}
	pingCancel()

	// 整个迁移过程共用一个总超时；迁移器内部还会持有数据库建议锁，
	// 防止多个发布任务同时修改表结构。
	migrationCtx, migrationCancel := context.WithTimeout(context.Background(), cfg.Migration.Timeout)
	defer migrationCancel()

	startedAt := time.Now()
	log.Print("正在执行数据库迁移")
	err = migrations.ApplyAllWithLock(migrationCtx, db, cfg.Migration.LockTimeout)
	if err != nil {
		log.Fatalf("数据库迁移失败：%v", err)
	}
	log.Printf("数据库迁移完成，耗时=%s", time.Since(startedAt).Round(time.Millisecond))
}
