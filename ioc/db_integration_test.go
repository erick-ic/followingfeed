//go:build integration

package ioc

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"followingfeed/config"
	"followingfeed/internal/observability"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMySQLQueryTimeoutAgainstRealServer 验证 GORM 回调会真正取消服务端慢 SQL，
// 而不只是在配置中保存了一个未使用的超时数值。
func TestMySQLQueryTimeoutAgainstRealServer(t *testing.T) {
	dsn := os.Getenv("FOLLOWINGFEED_TEST_MYSQL_DSN")
	require.NotEmpty(t, dsn, "FOLLOWINGFEED_TEST_MYSQL_DSN must point to an isolated test database")
	cfg := config.Config{MySQL: config.MySQLConfig{
		DSN:             dsn,
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Minute,
		ConnectTimeout:  time.Second,
		ReadTimeout:     2 * time.Second,
		WriteTimeout:    2 * time.Second,
		QueryTimeout:    50 * time.Millisecond,
	}}

	db, cleanup, err := InitMySQL(cfg, observability.NewMetrics())
	require.NoError(t, err)
	t.Cleanup(cleanup)

	started := time.Now()
	var result []struct {
		ID int `gorm:"column:id"`
	}
	err = db.Table("(SELECT 1 AS id) AS timeout_probe").
		Where("SLEEP(?) = 0", 1).
		Find(&result).Error
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled), err)
	assert.Less(t, time.Since(started), 500*time.Millisecond)
}
