package service

import (
	"context"
	"errors"
	"followingfeed/internal/domain"
	cachemocks "followingfeed/internal/repository/cache/mocks"
	repomocks "followingfeed/internal/repository/mocks"
	"followingfeed/pkg/logger"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"testing"
)

//go:generate mockgen -source=follow.go -package=svcmocks -destination=mocks/follow.mock.go
type noopUserProfileCacheInvalidator struct{}

func (noopUserProfileCacheInvalidator) DelPublicProfiles(
	context.Context,
	...int64,
) error {
	return nil
}

func (noopUserProfileCacheInvalidator) DelArticleStats(
	context.Context,
	int64,
) error {
	return nil
}

type followTestUserService struct{ profileErr error }

func (s followTestUserService) Create(context.Context, domain.User) error { return nil }
func (s followTestUserService) Login(context.Context, string, string) (domain.User, error) {
	return domain.User{}, nil
}
func (s followTestUserService) Profile(context.Context, int64) (domain.User, error) {
	return domain.User{Id: 2}, s.profileErr
}

func TestFollowServiceFollow(t *testing.T) {
	databaseErr := errors.New("database unavailable")
	testCases := []struct {
		name        string
		targetID    int64
		repoErr     error
		profileErr  error
		cacheErr    error
		wantErr     error
		callsRepo   bool
		invalidates bool
	}{
		{name: "关注成功后失效双方资料缓存", targetID: 2, callsRepo: true, invalidates: true},
		{name: "不能关注自己", targetID: 1, wantErr: ErrCannotFollowSelf},
		{name: "目标用户不存在", targetID: 2, profileErr: ErrUserNotFound, wantErr: ErrTargetUserNotFound},
		{name: "查询目标用户失败", targetID: 2, profileErr: databaseErr, wantErr: databaseErr},
		{name: "关注写入失败", targetID: 2, repoErr: databaseErr, wantErr: databaseErr, callsRepo: true},
		{
			name:        "缓存失效失败不影响关注结果",
			targetID:    2,
			cacheErr:    databaseErr,
			callsRepo:   true,
			invalidates: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := repomocks.NewMockFollowRepository(ctrl)
			invalidator := cachemocks.NewMockUserProfileCacheInvalidator(ctrl)
			if tc.callsRepo {
				repo.EXPECT().Follow(gomock.Any(), int64(1), tc.targetID).Return(tc.repoErr)
			}
			if tc.invalidates {
				invalidator.EXPECT().
					DelPublicProfiles(gomock.Any(), int64(1), tc.targetID).
					Return(tc.cacheErr)
			}
			err := NewFollowService(
				repo,
				followTestUserService{profileErr: tc.profileErr},
				invalidator,
				&logger.NopLogger{},
			).Follow(context.Background(), 1, tc.targetID)
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestFollowServiceUnFollow(t *testing.T) {
	databaseErr := errors.New("database unavailable")
	testCases := []struct {
		name        string
		repoErr     error
		cacheErr    error
		invalidates bool
	}{
		{name: "取消成功后失效双方资料缓存", invalidates: true},
		{name: "取消写入失败", repoErr: databaseErr},
		{name: "缓存失效失败不影响取消结果", cacheErr: databaseErr, invalidates: true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := repomocks.NewMockFollowRepository(ctrl)
			invalidator := cachemocks.NewMockUserProfileCacheInvalidator(ctrl)
			repo.EXPECT().UnFollow(gomock.Any(), int64(1), int64(2)).Return(tc.repoErr)
			if tc.invalidates {
				invalidator.EXPECT().
					DelPublicProfiles(gomock.Any(), int64(1), int64(2)).
					Return(tc.cacheErr)
			}
			err := NewFollowService(
				repo,
				followTestUserService{},
				invalidator,
				&logger.NopLogger{},
			).UnFollow(context.Background(), 1, 2)
			assert.ErrorIs(t, err, tc.repoErr)
		})
	}
}
