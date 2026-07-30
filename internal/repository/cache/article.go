package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"followingfeed/internal/domain"
	"time"

	"github.com/redis/go-redis/v9"
)

type ArticleCache interface {
	GetFirstPage(ctx context.Context, authorId int64) ([]domain.Article, error)
	SetFirstPage(ctx context.Context, authorId int64, articles []domain.Article) error
	DelFirstPage(ctx context.Context, authorId int64) error
}

type RedisArticleCache struct {
	client redis.Cmdable
}

func (r *RedisArticleCache) GetFirstPage(ctx context.Context, authorId int64) ([]domain.Article, error) {
	data, err := r.client.Get(ctx, r.key(authorId)).Bytes()
	if err != nil {
		return nil, err
	}
	var articles []domain.Article
	er := json.Unmarshal(data, &articles)
	return articles, er
}

func (r *RedisArticleCache) SetFirstPage(ctx context.Context, authorId int64, articles []domain.Article) error {
	// 空列表同样需要缓存，避免没有文章的用户每次都回源数据库。
	if articles == nil {
		articles = make([]domain.Article, 0)
	}
	for i := 0; i < len(articles); i++ {
		articles[i].Content = articles[i].Abstract()
	}
	data, err := json.Marshal(articles)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, r.key(authorId), data, time.Second*10).Err()
}

func (r *RedisArticleCache) DelFirstPage(ctx context.Context, authorId int64) error {
	return r.client.Del(ctx, r.key(authorId)).Err()
}

func NewRedisArticleCache(client redis.Cmdable) ArticleCache {
	return &RedisArticleCache{
		client: client,
	}
}

func (r *RedisArticleCache) key(authorId int64) string {
	return fmt.Sprintf("article:first_page:%d", authorId)
}
