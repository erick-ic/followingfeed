package ioc

import (
	"context"
	"fmt"
	"followingfeed/config"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func InitMySQL(cfg config.Config) (*gorm.DB, error) {
	dsn := cfg.MySQL.GetDSN()
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get mysql db: %w", err)
	}
	connMaxLifetime, err := time.ParseDuration(cfg.MySQL.ConnMaxLifetime)
	if err != nil {
		return nil, fmt.Errorf("parse mysql conn max lifetime: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MySQL.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MySQL.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)
	pingCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err = sqlDB.PingContext(pingCtx); err != nil {
		return nil, fmt.Errorf("ping mysql: %w", err)
	}

	return db, nil
}
