package service_test

import (
	"context"
	"errors"
	"testing"

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

func TestMyProfileServiceGet(t *testing.T) {
	databaseErr := errors.New("database unavailable")
	user := domain.User{Id: 1, Nickname: "云端旅人"}
	testCases := []struct {
		name       string
		setup      func(*svcmocks.MockUserService, *svcmocks.MockInteractiveService, *cachemocks.MockUserArticleStatsCache)
		want       service.MyProfile
		wantErr    error
		errMessage string
	}{
		{
			name: "缓存未命中后查询完整互动统计并回写缓存",
			setup: func(userSvc *svcmocks.MockUserService, interactiveSvc *svcmocks.MockInteractiveService, statsCache *cachemocks.MockUserArticleStatsCache) {
				userSvc.EXPECT().Profile(gomock.Any(), int64(1)).Return(user, nil)
				statsCache.EXPECT().
					Get(gomock.Any(), int64(1)).
					Return(cache.ArticleStats{}, redis.Nil)
				interactiveSvc.EXPECT().
					GetArticleStatsByAuthor(gomock.Any(), "article", int64(1)).
					Return(domain.Interactive{LikeCnt: 5, ReadCnt: 6, CollectCnt: 7}, nil)
				statsCache.EXPECT().
					Set(gomock.Any(), int64(1), cache.ArticleStats{LikeCount: 5, ReadCount: 6, CollectCount: 7}).
					Return(nil)
			},
			want: service.MyProfile{
				User:                user,
				ArticleLikeCount:    5,
				ArticleReadCount:    6,
				ArticleCollectCount: 7,
			},
		},
		{
			name: "缓存命中直接组装资料",
			setup: func(userSvc *svcmocks.MockUserService, _ *svcmocks.MockInteractiveService, statsCache *cachemocks.MockUserArticleStatsCache) {
				userSvc.EXPECT().Profile(gomock.Any(), int64(1)).Return(user, nil)
				statsCache.EXPECT().
					Get(gomock.Any(), int64(1)).
					Return(cache.ArticleStats{LikeCount: 2, ReadCount: 3, CollectCount: 4}, nil)
			},
			want: service.MyProfile{
				User:                user,
				ArticleLikeCount:    2,
				ArticleReadCount:    3,
				ArticleCollectCount: 4,
			},
		},
		{
			name: "用户资料查询失败",
			setup: func(userSvc *svcmocks.MockUserService, _ *svcmocks.MockInteractiveService, _ *cachemocks.MockUserArticleStatsCache) {
				userSvc.EXPECT().Profile(gomock.Any(), int64(1)).Return(domain.User{}, databaseErr)
			},
			wantErr: databaseErr, errMessage: "查询当前用户资料失败",
		},
		{
			name: "互动统计查询失败",
			setup: func(userSvc *svcmocks.MockUserService, interactiveSvc *svcmocks.MockInteractiveService, statsCache *cachemocks.MockUserArticleStatsCache) {
				userSvc.EXPECT().Profile(gomock.Any(), int64(1)).Return(user, nil)
				statsCache.EXPECT().
					Get(gomock.Any(), int64(1)).
					Return(cache.ArticleStats{}, redis.Nil)
				interactiveSvc.EXPECT().
					GetArticleStatsByAuthor(gomock.Any(), "article", int64(1)).
					Return(domain.Interactive{}, databaseErr)
			},
			wantErr: databaseErr, errMessage: "查询文章互动统计失败",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			userSvc := svcmocks.NewMockUserService(ctrl)
			interactiveSvc := svcmocks.NewMockInteractiveService(ctrl)
			statsCache := cachemocks.NewMockUserArticleStatsCache(ctrl)
			tc.setup(userSvc, interactiveSvc, statsCache)
			got, err := service.NewMyProfileService(userSvc, interactiveSvc, statsCache, &logger.NopLogger{}).
				Get(context.Background(), 1)
			assert.Equal(t, tc.want, got)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Contains(t, err.Error(), tc.errMessage)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
