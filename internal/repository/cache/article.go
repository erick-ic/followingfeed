package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"followingfeed/internal/domain"
	"followingfeed/internal/observability"

	"github.com/redis/go-redis/v9"
)

const (
	cacheNameArticleAuthorFirstPage    = "article_author_first_page"
	cacheNameArticlePublishedFirstPage = "article_published_first_page"
	cacheNameArticlePublishedCount     = "article_published_count"
	cacheNameArticlePublished          = "article_published"
)

type ArticleCache interface {
	GetFirstPage(ctx context.Context, authorId int64) ([]domain.Article, error)
	SetFirstPage(ctx context.Context, authorId int64, articles []domain.Article) error
	DelFirstPage(ctx context.Context, authorId int64) error
	GetPublishedFirstPage(ctx context.Context) ([]domain.PublishArticle, error)
	SetPublishedFirstPage(ctx context.Context, articles []domain.PublishArticle) error
	GetPublishedCount(ctx context.Context) (int64, error)
	SetPublishedCount(ctx context.Context, count int64) error
	DelPublished(ctx context.Context) error
}

type RedisArticleCache struct {
	client redis.Cmdable
}

func (r *RedisArticleCache) GetFirstPage(
	ctx context.Context,
	authorId int64,
) ([]domain.Article, error) {
	ctx = observability.WithCacheName(ctx, cacheNameArticleAuthorFirstPage)
	data, err := r.client.Get(ctx, r.key(authorId)).Bytes()
	if err != nil {
		return nil, err
	}
	var articles []domain.Article
	er := json.Unmarshal(data, &articles)
	return articles, er
}

func (r *RedisArticleCache) SetFirstPage(
	ctx context.Context,
	authorId int64,
	articles []domain.Article,
) error {
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
	ctx = observability.WithCacheName(ctx, cacheNameArticleAuthorFirstPage)
	return r.client.Set(ctx, r.key(authorId), data, articleListTTL).Err()
}

func (r *RedisArticleCache) DelFirstPage(ctx context.Context, authorId int64) error {
	ctx = observability.WithCacheName(ctx, cacheNameArticleAuthorFirstPage)
	return r.client.Del(ctx, r.key(authorId)).Err()
}

func (r *RedisArticleCache) GetPublishedFirstPage(
	ctx context.Context,
) ([]domain.PublishArticle, error) {
	ctx = observability.WithCacheName(ctx, cacheNameArticlePublishedFirstPage)
	data, err := r.client.Get(ctx, r.publishedFirstPageKey()).Bytes()
	if err != nil {
		return nil, err
	}
	var articles []domain.PublishArticle
	err = json.Unmarshal(data, &articles)
	return articles, err
}

func (r *RedisArticleCache) SetPublishedFirstPage(
	ctx context.Context,
	articles []domain.PublishArticle,
) error {
	if articles == nil {
		articles = make([]domain.PublishArticle, 0)
	}
	cached := append([]domain.PublishArticle(nil), articles...)
	for i := range cached {
		article := domain.Article(cached[i])
		article.Content = article.Abstract()
		cached[i] = domain.PublishArticle(article)
	}
	data, err := json.Marshal(cached)
	if err != nil {
		return err
	}
	ctx = observability.WithCacheName(ctx, cacheNameArticlePublishedFirstPage)
	return r.client.Set(ctx, r.publishedFirstPageKey(), data, articleListTTL).Err()
}

func (r *RedisArticleCache) GetPublishedCount(ctx context.Context) (int64, error) {
	ctx = observability.WithCacheName(ctx, cacheNameArticlePublishedCount)
	return r.client.Get(ctx, r.publishedCountKey()).Int64()
}

func (r *RedisArticleCache) SetPublishedCount(ctx context.Context, count int64) error {
	ctx = observability.WithCacheName(ctx, cacheNameArticlePublishedCount)
	return r.client.Set(ctx, r.publishedCountKey(), count, articleListTTL).Err()
}

func (r *RedisArticleCache) DelPublished(ctx context.Context) error {
	ctx = observability.WithCacheName(ctx, cacheNameArticlePublished)
	return r.client.Del(ctx, r.publishedFirstPageKey(), r.publishedCountKey()).Err()
}

func NewRedisArticleCache(client redis.Cmdable) ArticleCache {
	return &RedisArticleCache{
		client: client,
	}
}

func (r *RedisArticleCache) key(authorId int64) string {
	return fmt.Sprintf("article:first_page:%s:%d", cacheKeyVersion, authorId)
}

func (r *RedisArticleCache) publishedFirstPageKey() string {
	return "article:published:first_page:" + cacheKeyVersion
}

func (r *RedisArticleCache) publishedCountKey() string {
	return "article:published:count:" + cacheKeyVersion
}
