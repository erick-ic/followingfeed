package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"followingfeed/internal/domain"
	"followingfeed/internal/repository/cache/redismocks"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

//go:generate mockgen -package=redismocks -destination=redismocks/cmdable.mock.go github.com/redis/go-redis/v9 Cmdable
func TestRedisUserCacheSet(t *testing.T) {
	testCases := []struct {
		name    string
		mock    func(*gomock.Controller) redis.Cmdable
		wantErr error
	}{
		{
			name: "缓存成功",
			mock: func(ctrl *gomock.Controller) redis.Cmdable {
				cmd := redismocks.NewMockCmdable(ctrl)
				result := redis.NewStatusCmd(context.Background())
				cmd.EXPECT().Set(gomock.Any(), "user:info:v2:1", []byte(`{"Id":1,"Nickname":"云端旅人","Email":"alice@example.com","Password":"hash","CreatedAt":0,"UpdatedAt":0}`), 15*time.Minute).Return(result)
				return cmd
			},
		},
		{
			name: "Redis 异常",
			mock: func(ctrl *gomock.Controller) redis.Cmdable {
				cmd := redismocks.NewMockCmdable(ctrl)
				result := redis.NewStatusCmd(context.Background())
				result.SetErr(errors.New("redis unavailable"))
				cmd.EXPECT().Set(gomock.Any(), "user:info:v2:1", []byte(`{"Id":1,"Nickname":"云端旅人","Email":"alice@example.com","Password":"hash","CreatedAt":0,"UpdatedAt":0}`), 15*time.Minute).Return(result)
				return cmd
			},
			wantErr: errors.New("redis unavailable"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := NewUserCache(tc.mock(gomock.NewController(t))).Set(context.Background(), domain.User{
				Id: 1, Nickname: "云端旅人", Email: "alice@example.com", Password: "hash",
			})
			if tc.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.wantErr.Error())
			}
		})
	}
}

func TestRedisUserCacheGet(t *testing.T) {
	testCases := []struct {
		name    string
		mock    func(*gomock.Controller) redis.Cmdable
		want    domain.User
		wantErr error
	}{
		{
			name: "缓存命中",
			mock: func(ctrl *gomock.Controller) redis.Cmdable {
				cmd := redismocks.NewMockCmdable(ctrl)
				result := redis.NewStringCmd(context.Background(), "user:info:v2:1")
				result.SetVal(`{"Id":1,"Nickname":"云端旅人","Email":"alice@example.com","Password":"hash"}`)
				cmd.EXPECT().Get(gomock.Any(), "user:info:v2:1").Return(result)
				return cmd
			},
			want: domain.User{Id: 1, Nickname: "云端旅人", Email: "alice@example.com", Password: "hash"},
		},
		{
			name: "缓存不存在",
			mock: func(ctrl *gomock.Controller) redis.Cmdable {
				cmd := redismocks.NewMockCmdable(ctrl)
				result := redis.NewStringCmd(context.Background(), "user:info:v2:1")
				result.SetErr(redis.Nil)
				cmd.EXPECT().Get(gomock.Any(), "user:info:v2:1").Return(result)
				return cmd
			},
			wantErr: redis.Nil,
		},
		{
			name: "缓存数据格式错误",
			mock: func(ctrl *gomock.Controller) redis.Cmdable {
				cmd := redismocks.NewMockCmdable(ctrl)
				result := redis.NewStringCmd(context.Background(), "user:info:v2:1")
				result.SetVal("invalid-json")
				cmd.EXPECT().Get(gomock.Any(), "user:info:v2:1").Return(result)
				return cmd
			},
			wantErr: errors.New("invalid character"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewUserCache(tc.mock(gomock.NewController(t))).Get(context.Background(), 1)
			if tc.wantErr != nil {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
