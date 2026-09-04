package ioc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

const (
	queryTimeoutPluginName = "followingfeed:mysql:query_timeout"
	queryTimeoutCancelKey  = queryTimeoutPluginName + ":cancel"
)

// queryTimeoutPlugin 为业务 CRUD 语句设置独立截止时间。
// 更短的上游请求截止时间仍会优先生效。
// 迁移器直接使用 database/sql，由 migration.timeout 统一控制。
type queryTimeoutPlugin struct {
	timeout time.Duration
}

func (p queryTimeoutPlugin) Name() string {
	return queryTimeoutPluginName
}

func (p queryTimeoutPlugin) Initialize(db *gorm.DB) error {
	if p.timeout <= 0 {
		return errors.New("查询超时必须大于 0")
	}

	registrations := []struct {
		name     string
		register func(string, func(*gorm.DB)) error
	}{
		{"create", db.Callback().Create().Before("gorm:create").Register},
		{"query", db.Callback().Query().Before("gorm:query").Register},
		{"update", db.Callback().Update().Before("gorm:update").Register},
		{"delete", db.Callback().Delete().Before("gorm:delete").Register},
		{"raw", db.Callback().Raw().Before("gorm:raw").Register},
	}
	for _, registration := range registrations {
		if err := registration.register(p.beforeName(registration.name), p.apply); err != nil {
			return fmt.Errorf("注册 %s 超时回调失败：%w", registration.name, err)
		}
	}

	afterRegistrations := []struct {
		name     string
		register func(string, func(*gorm.DB)) error
	}{
		{"create", db.Callback().Create().After("gorm:create").Register},
		{"query", db.Callback().Query().After("gorm:query").Register},
		{"update", db.Callback().Update().After("gorm:update").Register},
		{"delete", db.Callback().Delete().After("gorm:delete").Register},
		{"raw", db.Callback().Raw().After("gorm:raw").Register},
	}
	for _, registration := range afterRegistrations {
		if err := registration.register(p.afterName(registration.name), cancelQueryTimeout); err != nil {
			return fmt.Errorf("注册 %s 超时清理回调失败：%w", registration.name, err)
		}
	}
	return nil
}

func (p queryTimeoutPlugin) beforeName(operation string) string {
	return p.Name() + ":before_" + operation
}

func (p queryTimeoutPlugin) afterName(operation string) string {
	return p.Name() + ":after_" + operation
}

func (p queryTimeoutPlugin) apply(tx *gorm.DB) {
	ctx, cancel := context.WithTimeout(tx.Statement.Context, p.timeout)
	tx.Statement.Context = ctx
	tx.InstanceSet(queryTimeoutCancelKey, cancel)
}

func cancelQueryTimeout(tx *gorm.DB) {
	value, ok := tx.InstanceGet(queryTimeoutCancelKey)
	if !ok {
		return
	}
	if cancel, ok := value.(context.CancelFunc); ok {
		cancel()
	}
}
