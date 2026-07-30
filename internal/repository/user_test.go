package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"followingfeed/internal/domain"
	cachemocks "followingfeed/internal/repository/cache/mocks"
	"followingfeed/internal/repository/dao"
	daomocks "followingfeed/internal/repository/dao/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

//go:generate mockgen -source=user.go -package=repomocks -destination=mocks/user.mock.go
//go:generate mockgen -source=cache/user.go -package=cachemocks -destination=cache/mocks/user.mock.go
func TestUserRepositoryFindById(t *testing.T) {
	testCases := []struct {
		name      string
		cacheUser domain.User
		cacheErr  error
		daoUser   dao.User
		daoErr    error
		want      domain.User
		wantErr   error
	}{
		{
			name: "缓存命中", cacheUser: domain.User{Id: 1, Nickname: "缓存旅人", Email: "cached@example.com"},
			want: domain.User{Id: 1, Nickname: "缓存旅人", Email: "cached@example.com"},
		},
		{
			name: "缓存未命中并查询数据库", cacheErr: errors.New("cache miss"),
			daoUser: dao.User{
				Id: 1, Nickname: "云端旅人",
				Email:    sql.NullString{String: "alice@example.com", Valid: true},
				Password: "hash", CreatedAt: 100, UpdatedAt: 200,
			},
			want: domain.User{
				Id: 1, Nickname: "云端旅人", Email: "alice@example.com",
				Password: "hash", CreatedAt: 100, UpdatedAt: 200,
			},
		},
		{
			name: "数据库查询失败", cacheErr: errors.New("cache miss"), daoErr: dao.ErrUserNotFound,
			wantErr: dao.ErrUserNotFound,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			cache := cachemocks.NewMockUserCache(ctrl)
			daoMock := daomocks.NewMockUserDAO(ctrl)
			cache.EXPECT().Get(gomock.Any(), int64(1)).Return(tc.cacheUser, tc.cacheErr)
			if tc.cacheErr != nil {
				daoMock.EXPECT().FindById(gomock.Any(), int64(1)).Return(tc.daoUser, tc.daoErr)
				if tc.daoErr == nil {
					cache.EXPECT().Set(gomock.Any(), tc.want).Return(nil).AnyTimes()
				}
			}

			got, err := NewUserRepositoryImpl(daoMock, cache).FindById(context.Background(), 1)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestUserRepositoryFindByEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	daoMock := daomocks.NewMockUserDAO(ctrl)
	daoMock.EXPECT().FindByEmail(gomock.Any(), "alice@example.com").Return(
		dao.User{
			Id: 1, Nickname: "云端旅人",
			Email:    sql.NullString{String: "alice@example.com", Valid: true},
			Password: "hash", CreatedAt: 100, UpdatedAt: 200,
		}, nil,
	)

	got, err := NewUserRepositoryImpl(daoMock, nil).FindByEmail(context.Background(), "alice@example.com")
	require.NoError(t, err)
	assert.Equal(t, domain.User{
		Id: 1, Nickname: "云端旅人", Email: "alice@example.com",
		Password: "hash", CreatedAt: 100, UpdatedAt: 200,
	}, got)
}

func TestUserRepositoryCreate(t *testing.T) {
	ctrl := gomock.NewController(t)
	daoMock := daomocks.NewMockUserDAO(ctrl)
	wantErr := errors.New("insert failed")
	daoMock.EXPECT().Insert(gomock.Any(), domain.User{
		Nickname: "云端旅人", Email: "alice@example.com", Password: "hash",
	}).Return(wantErr)

	err := NewUserRepositoryImpl(daoMock, nil).Create(context.Background(), domain.User{
		Nickname: "云端旅人", Email: "alice@example.com", Password: "hash",
	})
	assert.ErrorIs(t, err, wantErr)
}
