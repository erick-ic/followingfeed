//go:build integration

package dao

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"followingfeed/config"
	"followingfeed/internal/domain"
	"followingfeed/migrations"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestGORMUserDAOIntegration(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../.."))
	oldDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(root))
	defer os.Chdir(oldDir)

	cfg, err := config.Load()
	if err != nil {
		t.Skipf("未找到集成测试配置，跳过: %v", err)
	}
	db, err := gorm.Open(mysql.Open(cfg.MySQL.GetDSN()), &gorm.Config{})
	if err != nil {
		t.Skipf("MySQL 未启动，跳过集成测试: %v", err)
	}
	dao := NewGORMUserDAO(db)
	email := fmt.Sprintf("integration-%s@example.com", uuid.NewString())
	ctx := context.Background()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, migrations.ApplyAll(ctx, sqlDB))
	err = dao.Insert(ctx, domain.User{Nickname: "云端旅人", Email: email, Password: "hash"})
	require.NoError(t, err)

	got, err := dao.FindByEmail(ctx, email)
	require.NoError(t, err)
	assert.Equal(t, email, got.Email.String)
	assert.Equal(t, "云端旅人", got.Nickname)
	assert.Equal(t, "hash", got.Password)
	assert.NotZero(t, got.CreatedAt)
	assert.WithinDuration(t, time.Now(), time.UnixMilli(got.CreatedAt), time.Minute)

	byID, err := dao.FindById(ctx, int64(got.Id))
	require.NoError(t, err)
	assert.Equal(t, got.Id, byID.Id)

	_, err = dao.FindByEmail(ctx, "missing-"+email)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	duplicateErr := dao.Insert(ctx, domain.User{Nickname: "像素松鼠", Email: email, Password: "hash"})
	assert.ErrorIs(t, duplicateErr, ErrUserDuplicated)

}
