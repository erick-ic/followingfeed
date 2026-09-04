package ioc

import (
	"context"
	"fmt"
	"followingfeed/config"
	"followingfeed/internal/observability"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// InitMySQL 创建并校验 MySQL 连接，应用连接池配置后注册连接池与 GORM 查询指标。
// 初始化阶段主动探测连接，确保配置或网络错误在服务接收请求前暴露。
func InitMySQL(cfg config.Config, metrics *observability.Metrics) (*gorm.DB, func(), error) {
	dsn := cfg.MySQL.GetDSN()
	// 关闭 GORM 的隐式 Ping，统一在取得 sql.DB 并建立 cleanup 后使用可控超时探测。
	// 这样初始连接失败时也能确保连接池被关闭。
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		return nil, nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, fmt.Errorf("获取 MySQL 连接池失败：%w", err)
	}
	cleanup := func() { _ = sqlDB.Close() }
	sqlDB.SetMaxOpenConns(cfg.MySQL.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MySQL.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.MySQL.ConnMaxLifetime)

	// 启动探测使用独立短超时，避免数据库不可达时长期阻塞应用启动。
	pingCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err = sqlDB.PingContext(pingCtx); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("连接 MySQL 失败：%w", err)
	}
	if err = db.Use(queryTimeoutPlugin{timeout: cfg.MySQL.QueryTimeout}); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("启用 MySQL 查询超时失败：%w", err)
	}
	if err = metrics.RegisterDBStats(sqlDB); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("注册 MySQL 连接池指标失败：%w", err)
	}
	if err = metrics.InstrumentGORM(db); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("启用 MySQL 指标采集失败：%w", err)
	}

	return db, cleanup, nil
}
