package service

import (
	"context"
	"errors"
	"testing"

	"followingfeed/internal/domain"
	repomocks "followingfeed/internal/repository/mocks"
	"followingfeed/pkg/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"
)

var errDatabaseUnavailable = errors.New("database unavailable")

//go:generate mockgen -source=../repository/user.go -package=repomocks -destination=../repository/mocks/user.mock.go
func TestUserServiceCreate(t *testing.T) {
	testCases := []struct {
		name     string
		nickname string
		mock     func(*gomock.Controller, *domain.User) *repomocks.MockUserRepository
		wantErr  error
	}{
		{
			name:     "注册成功",
			nickname: "云端旅人",
			mock: func(ctrl *gomock.Controller, persisted *domain.User) *repomocks.MockUserRepository {
				repo := repomocks.NewMockUserRepository(ctrl)
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, u domain.User) error {
						*persisted = u
						return nil
					},
				)
				return repo
			},
		},
		{
			name:     "数据库异常",
			nickname: "云端旅人",
			mock: func(ctrl *gomock.Controller, _ *domain.User) *repomocks.MockUserRepository {
				repo := repomocks.NewMockUserRepository(ctrl)
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errDatabaseUnavailable)
				return repo
			},
			wantErr: errDatabaseUnavailable,
		},
		{
			name:     "邮箱重复",
			nickname: "云端旅人",
			mock: func(ctrl *gomock.Controller, _ *domain.User) *repomocks.MockUserRepository {
				repo := repomocks.NewMockUserRepository(ctrl)
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(ErrUserDuplicated)
				return repo
			},
			wantErr: ErrUserDuplicated,
		},
		{
			name:     "昵称格式错误",
			nickname: "云",
			mock: func(ctrl *gomock.Controller, _ *domain.User) *repomocks.MockUserRepository {
				return repomocks.NewMockUserRepository(ctrl)
			},
			wantErr: ErrInvalidNickname,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			var persisted domain.User
			serviceRepo := tc.mock(ctrl, &persisted)

			err := NewUserServiceImpl(
				serviceRepo,
				&logger.NopLogger{},
			).Create(context.Background(), domain.User{
				Nickname: tc.nickname,
				Email:    "alice@example.com",
				Password: "Passw0rd!",
			})
			if tc.wantErr == nil {
				require.NoError(t, err)
				assert.Equal(t, tc.nickname, persisted.Nickname)
				assert.Equal(t, "alice@example.com", persisted.Email)
				assert.NotEqual(t, "Passw0rd!", persisted.Password)
				assert.NoError(
					t,
					bcrypt.CompareHashAndPassword([]byte(persisted.Password), []byte("Passw0rd!")),
				)
			} else {
				assert.ErrorIs(t, err, tc.wantErr)
			}
		})
	}
}

func TestUserServiceLogin(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("Passw0rd!"), bcrypt.DefaultCost)
	require.NoError(t, err)

	testCases := []struct {
		name      string
		password  string
		mock      func(*gomock.Controller) *repomocks.MockUserRepository
		wantUser  domain.User
		wantError error
	}{
		{
			name:     "登录成功",
			password: "Passw0rd!",
			mock: func(ctrl *gomock.Controller) *repomocks.MockUserRepository {
				repo := repomocks.NewMockUserRepository(ctrl)
				repo.EXPECT().
					FindByEmail(gomock.Any(), "alice@example.com").
					Return(domain.User{Id: 1, Email: "alice@example.com", Password: string(hash)}, nil)
				return repo
			},
			wantUser: domain.User{Id: 1, Email: "alice@example.com", Password: string(hash)},
		},
		{
			name:     "密码错误",
			password: "WrongPass1!",
			mock: func(ctrl *gomock.Controller) *repomocks.MockUserRepository {
				repo := repomocks.NewMockUserRepository(ctrl)
				repo.EXPECT().
					FindByEmail(gomock.Any(), "alice@example.com").
					Return(domain.User{Id: 1, Email: "alice@example.com", Password: string(hash)}, nil)
				return repo
			},
			wantError: ErrInvalidUserPassword,
		},
		{
			name:     "数据库异常",
			password: "Passw0rd!",
			mock: func(ctrl *gomock.Controller) *repomocks.MockUserRepository {
				repo := repomocks.NewMockUserRepository(ctrl)
				repo.EXPECT().
					FindByEmail(gomock.Any(), "alice@example.com").
					Return(domain.User{}, errDatabaseUnavailable)
				return repo
			},
			wantError: errDatabaseUnavailable,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			user, err := NewUserServiceImpl(
				tc.mock(ctrl),
				&logger.NopLogger{},
			).Login(context.Background(), "alice@example.com", tc.password)
			assert.Equal(t, tc.wantUser, user)
			if tc.wantError == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tc.wantError)
			}
		})
	}
}
