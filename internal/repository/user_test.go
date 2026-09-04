package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"followingfeed/internal/domain"
	"followingfeed/internal/repository/cache"
	cachemocks "followingfeed/internal/repository/cache/mocks"
	"followingfeed/internal/repository/dao"
	daomocks "followingfeed/internal/repository/dao/mocks"
	"followingfeed/pkg/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

//go:generate mockgen -source=user.go -package=repomocks -destination=mocks/user.mock.go
//go:generate mockgen -source=cache/user.go -package=cachemocks -destination=cache/mocks/user.mock.go
//go:generate mockgen -source=dao/user.go -package=daomocks -destination=dao/mocks/user.mock.go

func TestUserRepositoryFindById(t *testing.T) {
	testCases := []struct {
		name    string
		ctx     context.Context
		uid     int64
		mock    func(*gomock.Controller) (dao.UserDAO, cache.UserCache)
		want    domain.User
		wantErr error
	}{
		{
			name: "缓存未命中后查询数据库并回写缓存",
			ctx:  context.Background(),
			uid:  1,
			mock: func(ctrl *gomock.Controller) (dao.UserDAO, cache.UserCache) {
				cacheMock := cachemocks.NewMockUserCache(ctrl)
				cacheMock.EXPECT().
					Get(gomock.Any(), int64(1)).
					Return(domain.User{}, cache.ErrNotExists)
				cacheMock.EXPECT().Set(gomock.Any(), domain.User{
					Id:        1,
					Nickname:  "云端旅人",
					Email:     "alice@example.com",
					Password:  "hash",
					CreatedAt: 100,
					UpdatedAt: 200,
				}).Return(nil)
				daoMock := daomocks.NewMockUserDAO(ctrl)
				daoMock.EXPECT().FindById(gomock.Any(), int64(1)).Return(dao.User{
					Id:        1,
					Nickname:  "云端旅人",
					Email:     sql.NullString{String: "alice@example.com", Valid: true},
					Password:  "hash",
					CreatedAt: 100,
					UpdatedAt: 200,
				}, nil)
				return daoMock, cacheMock
			},
			want: domain.User{
				Id:        1,
				Nickname:  "云端旅人",
				Email:     "alice@example.com",
				Password:  "hash",
				CreatedAt: 100,
				UpdatedAt: 200,
			},
		},
		{
			name: "缓存命中",
			ctx:  context.Background(),
			uid:  1,
			mock: func(ctrl *gomock.Controller) (dao.UserDAO, cache.UserCache) {
				cacheMock := cachemocks.NewMockUserCache(ctrl)
				cacheMock.EXPECT().Get(gomock.Any(), int64(1)).Return(domain.User{
					Id:        1,
					Nickname:  "云端旅人",
					Email:     "alice@example.com",
					Password:  "hash",
					CreatedAt: 100,
					UpdatedAt: 200,
				}, nil)
				return daomocks.NewMockUserDAO(ctrl), cacheMock
			},
			want: domain.User{
				Id:        1,
				Nickname:  "云端旅人",
				Email:     "alice@example.com",
				Password:  "hash",
				CreatedAt: 100,
				UpdatedAt: 200,
			},
		},
		{
			name:    "空值缓存命中",
			ctx:     context.Background(),
			uid:     1,
			wantErr: ErrUserNotFound,
			mock: func(ctrl *gomock.Controller) (dao.UserDAO, cache.UserCache) {
				cacheMock := cachemocks.NewMockUserCache(ctrl)
				cacheMock.EXPECT().
					Get(gomock.Any(), int64(1)).
					Return(domain.User{}, cache.ErrCachedNotFound)
				return daomocks.NewMockUserDAO(ctrl), cacheMock
			},
		},
		{
			name: "缓存故障时降级查询数据库",
			ctx:  context.Background(),
			uid:  1,
			mock: func(ctrl *gomock.Controller) (dao.UserDAO, cache.UserCache) {
				cacheMock := cachemocks.NewMockUserCache(ctrl)
				cacheMock.EXPECT().
					Get(gomock.Any(), int64(1)).
					Return(domain.User{}, errors.New("mock cache error"))
				cacheMock.EXPECT().Set(gomock.Any(), domain.User{
					Id:        1,
					Nickname:  "云端旅人",
					Email:     "alice@example.com",
					Password:  "hash",
					CreatedAt: 100,
					UpdatedAt: 200,
				}).Return(nil)
				daoMock := daomocks.NewMockUserDAO(ctrl)
				daoMock.EXPECT().FindById(gomock.Any(), int64(1)).Return(dao.User{
					Id:        1,
					Nickname:  "云端旅人",
					Email:     sql.NullString{String: "alice@example.com", Valid: true},
					Password:  "hash",
					CreatedAt: 100,
					UpdatedAt: 200,
				}, nil)
				return daoMock, cacheMock
			},
			want: domain.User{
				Id:        1,
				Nickname:  "云端旅人",
				Email:     "alice@example.com",
				Password:  "hash",
				CreatedAt: 100,
				UpdatedAt: 200,
			},
		},
		{
			name:    "数据库未找到时回写空值缓存",
			ctx:     context.Background(),
			uid:     1,
			wantErr: ErrUserNotFound,
			mock: func(ctrl *gomock.Controller) (dao.UserDAO, cache.UserCache) {
				cacheMock := cachemocks.NewMockUserCache(ctrl)
				cacheMock.EXPECT().
					Get(gomock.Any(), int64(1)).
					Return(domain.User{}, cache.ErrNotExists)
				cacheMock.EXPECT().SetNotFound(gomock.Any(), int64(1)).Return(nil)
				daoMock := daomocks.NewMockUserDAO(ctrl)
				daoMock.EXPECT().
					FindById(gomock.Any(), int64(1)).
					Return(dao.User{}, dao.ErrUserNotFound)
				return daoMock, cacheMock
			},
		},
		{
			name: "数据库查询失败",
			ctx:  context.Background(),
			uid:  1,
			mock: func(ctrl *gomock.Controller) (dao.UserDAO, cache.UserCache) {
				cacheMock := cachemocks.NewMockUserCache(ctrl)
				cacheMock.EXPECT().
					Get(gomock.Any(), int64(1)).
					Return(domain.User{}, cache.ErrNotExists)
				daoMock := daomocks.NewMockUserDAO(ctrl)
				daoMock.EXPECT().
					FindById(gomock.Any(), int64(1)).
					Return(dao.User{}, errors.New("mock db error"))
				return daoMock, cacheMock
			},
			wantErr: errors.New("mock db error"),
		},
		{
			name: "缓存回写失败不影响查询结果",
			ctx:  context.Background(),
			uid:  1,
			mock: func(ctrl *gomock.Controller) (dao.UserDAO, cache.UserCache) {
				cacheMock := cachemocks.NewMockUserCache(ctrl)
				cacheMock.EXPECT().
					Get(gomock.Any(), int64(1)).
					Return(domain.User{}, cache.ErrNotExists)
				cacheMock.EXPECT().Set(gomock.Any(), domain.User{
					Id:        1,
					Nickname:  "云端旅人",
					Email:     "alice@example.com",
					Password:  "hash",
					CreatedAt: 100,
					UpdatedAt: 200,
				}).Return(errors.New("mock cache set error"))
				daoMock := daomocks.NewMockUserDAO(ctrl)
				daoMock.EXPECT().FindById(gomock.Any(), int64(1)).Return(dao.User{
					Id:        1,
					Nickname:  "云端旅人",
					Email:     sql.NullString{String: "alice@example.com", Valid: true},
					Password:  "hash",
					CreatedAt: 100,
					UpdatedAt: 200,
				}, nil)
				return daoMock, cacheMock
			},
			want: domain.User{
				Id:        1,
				Nickname:  "云端旅人",
				Email:     "alice@example.com",
				Password:  "hash",
				CreatedAt: 100,
				UpdatedAt: 200,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			daoMock, cacheMock := tc.mock(ctrl)
			got, err := NewUserRepositoryImpl(
				daoMock,
				cacheMock,
				&logger.NopLogger{},
			).FindById(tc.ctx, tc.uid)
			if tc.wantErr != nil {
				assert.EqualError(t, err, tc.wantErr.Error())
				assert.Equal(t, domain.User{}, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestUserRepositoryFindByEmail(t *testing.T) {
	testCases := []struct {
		name    string
		email   string
		mock    func(*gomock.Controller) dao.UserDAO
		want    domain.User
		wantErr error
	}{
		{
			name:  "查询成功",
			email: "alice@example.com",
			mock: func(ctrl *gomock.Controller) dao.UserDAO {
				daoMock := daomocks.NewMockUserDAO(ctrl)
				daoMock.EXPECT().FindByEmail(gomock.Any(), "alice@example.com").Return(dao.User{
					Id:        1,
					Nickname:  "云端旅人",
					Email:     sql.NullString{String: "alice@example.com", Valid: true},
					Password:  "hash",
					CreatedAt: 100,
					UpdatedAt: 200,
				}, nil)
				return daoMock
			},
			want: domain.User{
				Id:        1,
				Nickname:  "云端旅人",
				Email:     "alice@example.com",
				Password:  "hash",
				CreatedAt: 100,
				UpdatedAt: 200,
			},
		},
		{
			name:  "查询失败",
			email: "missing@example.com",
			mock: func(ctrl *gomock.Controller) dao.UserDAO {
				daoMock := daomocks.NewMockUserDAO(ctrl)
				daoMock.EXPECT().
					FindByEmail(gomock.Any(), "missing@example.com").
					Return(dao.User{}, errors.New("mock db error"))
				return daoMock
			},
			wantErr: errors.New("mock db error"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := NewUserRepositoryImpl(tc.mock(ctrl), nil, &logger.NopLogger{})
			got, err := repo.FindByEmail(context.Background(), tc.email)
			if tc.wantErr != nil {
				assert.EqualError(t, err, tc.wantErr.Error())
				assert.Equal(t, domain.User{}, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestUserRepositoryCreate(t *testing.T) {
	testCases := []struct {
		name    string
		user    domain.User
		mock    func(*gomock.Controller) dao.UserDAO
		wantErr error
	}{
		{
			name: "创建成功",
			user: domain.User{
				Nickname: "云端旅人",
				Email:    "alice@example.com",
				Password: "hash",
			},
			mock: func(ctrl *gomock.Controller) dao.UserDAO {
				daoMock := daomocks.NewMockUserDAO(ctrl)
				daoMock.EXPECT().Insert(gomock.Any(), domain.User{
					Nickname: "云端旅人",
					Email:    "alice@example.com",
					Password: "hash",
				}).Return(nil)
				return daoMock
			},
		},
		{
			name: "创建失败",
			user: domain.User{
				Nickname: "云端旅人",
				Email:    "alice@example.com",
				Password: "hash",
			},
			mock: func(ctrl *gomock.Controller) dao.UserDAO {
				daoMock := daomocks.NewMockUserDAO(ctrl)
				daoMock.EXPECT().Insert(gomock.Any(), domain.User{
					Nickname: "云端旅人",
					Email:    "alice@example.com",
					Password: "hash",
				}).Return(errors.New("mock insert error"))
				return daoMock
			},
			wantErr: errors.New("mock insert error"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			err := NewUserRepositoryImpl(
				tc.mock(ctrl),
				nil,
				&logger.NopLogger{},
			).Create(context.Background(), tc.user)
			if tc.wantErr != nil {
				assert.EqualError(t, err, tc.wantErr.Error())
				return
			}
			require.NoError(t, err)
		})
	}
}
