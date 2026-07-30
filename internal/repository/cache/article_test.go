package cache

import (
	"context"
	"testing"
	"time"

	"followingfeed/internal/domain"
	"followingfeed/internal/repository/cache/redismocks"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRedisArticleCacheSetFirstPageCachesEmptyList(t *testing.T) {
	ctrl := gomock.NewController(t)
	cmd := redismocks.NewMockCmdable(ctrl)
	result := redis.NewStatusCmd(context.Background())
	cmd.EXPECT().Set(gomock.Any(), "article:first_page:1", []byte("[]"), 10*time.Second).Return(result)

	err := NewRedisArticleCache(cmd).SetFirstPage(context.Background(), 1, nil)
	assert.NoError(t, err)
}

func TestRedisArticleCacheGetFirstPage(t *testing.T) {
	ctrl := gomock.NewController(t)
	cmd := redismocks.NewMockCmdable(ctrl)
	result := redis.NewStringCmd(context.Background(), "article:first_page:1")
	result.SetVal(`[{"Id":1,"Title":"title","Content":"summary","Author":{"Id":1}}]`)
	cmd.EXPECT().Get(gomock.Any(), "article:first_page:1").Return(result)

	articles, err := NewRedisArticleCache(cmd).GetFirstPage(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, []domain.Article{{
		Id:      1,
		Title:   "title",
		Content: "summary",
		Author:  domain.Author{Id: 1},
	}}, articles)
}
