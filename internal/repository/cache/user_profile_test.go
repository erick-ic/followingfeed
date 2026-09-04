package cache

import (
	"context"
	"testing"

	"followingfeed/internal/domain"
	"followingfeed/internal/repository/cache/redismocks"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

//go:generate mockgen -source=user_profile.go -package=cachemocks -destination=mocks/user_profile.mock.go
func TestRedisPublicProfileCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	cmd := redismocks.NewMockCmdable(ctrl)
	profile := PublicProfile{User: domain.User{Id: 1, Nickname: "云端旅人"}, FollowersCount: 3}
	setResult := redis.NewStatusCmd(context.Background())
	cmd.EXPECT().Set(
		gomock.Any(), "user:public_profile:v1:1",
		[]byte(
			`{"User":{"Id":1,"Nickname":"云端旅人","Email":"","Password":"","CreatedAt":0,"UpdatedAt":0},"FollowingCount":0,"FollowersCount":3,"ArticleCount":0}`,
		),
		gomock.Any(),
	).
		Return(setResult)
	require.NoError(t, NewPublicProfileCache(cmd).Set(context.Background(), profile))

	getResult := redis.NewStringCmd(context.Background())
	getResult.SetVal(`{"User":{"Id":1,"Nickname":"云端旅人"},"FollowersCount":3}`)
	cmd.EXPECT().Get(gomock.Any(), "user:public_profile:v1:1").Return(getResult)
	got, err := NewPublicProfileCache(cmd).Get(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, profile, got)
}

func TestRedisPublicProfileCacheNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	cmd := redismocks.NewMockCmdable(ctrl)
	result := redis.NewStringCmd(context.Background())
	result.SetVal(cachedNotFoundValue)
	cmd.EXPECT().Get(gomock.Any(), "user:public_profile:v1:1").Return(result)

	_, err := NewPublicProfileCache(cmd).Get(context.Background(), 1)
	assert.ErrorIs(t, err, ErrCachedNotFound)
}

func TestRedisUserArticleStatsCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	cmd := redismocks.NewMockCmdable(ctrl)
	stats := ArticleStats{LikeCount: 2, ReadCount: 3, CollectCount: 4}
	setResult := redis.NewStatusCmd(context.Background())
	cmd.EXPECT().Set(
		gomock.Any(), "user:article_stats:v1:1",
		[]byte(`{"LikeCount":2,"ReadCount":3,"CollectCount":4}`), gomock.Any(),
	).Return(setResult)
	require.NoError(t, NewUserArticleStatsCache(cmd).Set(context.Background(), 1, stats))

	getResult := redis.NewStringCmd(context.Background())
	getResult.SetVal(`{"LikeCount":2,"ReadCount":3,"CollectCount":4}`)
	cmd.EXPECT().Get(gomock.Any(), "user:article_stats:v1:1").Return(getResult)
	got, err := NewUserArticleStatsCache(cmd).Get(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, stats, got)
}
