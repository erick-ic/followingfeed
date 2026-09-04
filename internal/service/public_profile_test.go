package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"followingfeed/internal/domain"
	"followingfeed/internal/repository/cache"
	cachemocks "followingfeed/internal/repository/cache/mocks"
	"followingfeed/internal/service"
	svcmocks "followingfeed/internal/service/mocks"
	"followingfeed/pkg/logger"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newPublicProfileServiceForTest(
	ctrl *gomock.Controller,
	userSvc service.UserService,
	followSvc service.FollowService,
	articleSvc service.ArticleService,
) service.PublicProfileService {
	profileCache := cachemocks.NewMockPublicProfileCache(ctrl)
	profileCache.EXPECT().
		Get(gomock.Any(), gomock.Any()).
		Return(cache.PublicProfile{}, redis.Nil).
		AnyTimes()
	profileCache.EXPECT().Set(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	profileCache.EXPECT().SetNotFound(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	return service.NewPublicProfileService(
		userSvc,
		followSvc,
		articleSvc,
		profileCache,
		&logger.NopLogger{},
	)
}

func TestPublicProfileServiceGet(t *testing.T) {
	testCases := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "缓存未命中后并发查询三项统计", run: testPublicProfileServiceGetRunsStatisticsConcurrently},
		{name: "缓存命中", run: testPublicProfileServiceGetCacheHit},
		{name: "缓存命中空值", run: testPublicProfileServiceGetCachedNotFound},
		{name: "错误分类", run: testPublicProfileServiceGetClassifiesErrors},
	}
	for _, tc := range testCases {
		t.Run(tc.name, tc.run)
	}
}

func testPublicProfileServiceGetCacheHit(t *testing.T) {
	ctrl := gomock.NewController(t)
	profileCache := cachemocks.NewMockPublicProfileCache(ctrl)
	want := cache.PublicProfile{User: domain.User{Id: 1}, FollowersCount: 3}
	profileCache.EXPECT().Get(gomock.Any(), int64(1)).Return(want, nil)

	got, err := service.NewPublicProfileService(
		svcmocks.NewMockUserService(ctrl),
		svcmocks.NewMockFollowService(ctrl),
		svcmocks.NewMockArticleService(ctrl),
		profileCache,
		&logger.NopLogger{},
	).Get(context.Background(), 1)

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func testPublicProfileServiceGetCachedNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	profileCache := cachemocks.NewMockPublicProfileCache(ctrl)
	profileCache.EXPECT().
		Get(gomock.Any(), int64(1)).
		Return(cache.PublicProfile{}, cache.ErrCachedNotFound)

	_, err := service.NewPublicProfileService(
		svcmocks.NewMockUserService(ctrl),
		svcmocks.NewMockFollowService(ctrl),
		svcmocks.NewMockArticleService(ctrl),
		profileCache,
		&logger.NopLogger{},
	).Get(context.Background(), 1)

	assert.ErrorIs(t, err, service.ErrUserNotFound)
}

func testPublicProfileServiceGetRunsStatisticsConcurrently(t *testing.T) {
	ctrl := gomock.NewController(t)
	userSvc := svcmocks.NewMockUserService(ctrl)
	followSvc := svcmocks.NewMockFollowService(ctrl)
	articleSvc := svcmocks.NewMockArticleService(ctrl)

	user := domain.User{Id: 1, Nickname: "云端旅人", CreatedAt: 100}
	userSvc.EXPECT().Profile(gomock.Any(), int64(1)).Return(user, nil)

	started := make(chan struct{}, 3)
	release := make(chan struct{})
	wait := func(count int64) func(context.Context, int64) (int64, error) {
		return func(context.Context, int64) (int64, error) {
			started <- struct{}{}
			<-release
			return count, nil
		}
	}
	followSvc.EXPECT().CountFollowing(gomock.Any(), int64(1)).DoAndReturn(wait(2))
	followSvc.EXPECT().CountFollowers(gomock.Any(), int64(1)).DoAndReturn(wait(3))
	articleSvc.EXPECT().CountPublishedByAuthor(gomock.Any(), int64(1)).DoAndReturn(wait(4))

	type response struct {
		profile service.PublicProfile
		err     error
	}
	resultCh := make(chan response, 1)
	go func() {
		profile, err := newPublicProfileServiceForTest(
			ctrl,
			userSvc,
			followSvc,
			articleSvc,
		).Get(context.Background(), 1)
		resultCh <- response{profile: profile, err: err}
	}()

	for range 3 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("统计查询未并行启动")
		}
	}
	close(release)

	result := <-resultCh
	require.NoError(t, result.err)
	assert.Equal(t, service.PublicProfile{
		User: user, FollowingCount: 2, FollowersCount: 3, ArticleCount: 4,
	}, result.profile)
}

func testPublicProfileServiceGetClassifiesErrors(t *testing.T) {
	t.Run("用户不存在", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		userSvc := svcmocks.NewMockUserService(ctrl)
		userSvc.EXPECT().
			Profile(gomock.Any(), int64(1)).
			Return(domain.User{}, service.ErrUserNotFound)

		_, err := newPublicProfileServiceForTest(ctrl,
			userSvc,
			svcmocks.NewMockFollowService(ctrl),
			svcmocks.NewMockArticleService(ctrl),
		).Get(context.Background(), 1)

		require.Error(t, err)
		assert.ErrorIs(t, err, service.ErrUserNotFound)
		assert.Contains(t, err.Error(), "查询用户资料失败")
	})

	tests := []struct {
		name       string
		operation  string
		setupError func(*svcmocks.MockFollowService, *svcmocks.MockArticleService)
	}{
		{
			name:      "关注数查询失败",
			operation: "统计关注数量失败",
			setupError: func(follow *svcmocks.MockFollowService, article *svcmocks.MockArticleService) {
				follow.EXPECT().
					CountFollowing(gomock.Any(), int64(1)).
					Return(int64(0), errors.New("database unavailable"))
			},
		},
		{
			name:      "粉丝数查询失败",
			operation: "统计粉丝数量失败",
			setupError: func(follow *svcmocks.MockFollowService, article *svcmocks.MockArticleService) {
				follow.EXPECT().
					CountFollowers(gomock.Any(), int64(1)).
					Return(int64(0), errors.New("database unavailable"))
			},
		},
		{
			name:      "文章数查询失败",
			operation: "统计已发布文章数量失败",
			setupError: func(follow *svcmocks.MockFollowService, article *svcmocks.MockArticleService) {
				article.EXPECT().
					CountPublishedByAuthor(gomock.Any(), int64(1)).
					Return(int64(0), errors.New("database unavailable"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			userSvc := svcmocks.NewMockUserService(ctrl)
			followSvc := svcmocks.NewMockFollowService(ctrl)
			articleSvc := svcmocks.NewMockArticleService(ctrl)
			userSvc.EXPECT().Profile(gomock.Any(), int64(1)).Return(domain.User{Id: 1}, nil)
			tt.setupError(followSvc, articleSvc)

			// 其他两个并行调用允许成功或因 errgroup 取消而返回，以避免测试依赖调度顺序。
			followSvc.EXPECT().
				CountFollowing(gomock.Any(), int64(1)).
				Return(int64(0), nil).
				AnyTimes()
			followSvc.EXPECT().
				CountFollowers(gomock.Any(), int64(1)).
				Return(int64(0), nil).
				AnyTimes()
			articleSvc.EXPECT().
				CountPublishedByAuthor(gomock.Any(), int64(1)).
				Return(int64(0), nil).
				AnyTimes()

			_, err := newPublicProfileServiceForTest(
				ctrl,
				userSvc,
				followSvc,
				articleSvc,
			).Get(context.Background(), 1)
			require.Error(t, err)
			assert.True(t, strings.Contains(err.Error(), tt.operation), err.Error())
		})
	}
}
