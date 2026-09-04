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
	cacheNamePublicProfile    = "public_profile"
	cacheNameUserArticleStats = "user_article_stats"
)

type PublicProfile struct {
	User           domain.User
	FollowingCount int64
	FollowersCount int64
	ArticleCount   int64
}

type PublicProfileCache interface {
	Get(ctx context.Context, uid int64) (PublicProfile, error)
	Set(ctx context.Context, profile PublicProfile) error
	SetNotFound(ctx context.Context, uid int64) error
	Del(ctx context.Context, uids ...int64) error
}

type ArticleStats struct {
	LikeCount    int64
	ReadCount    int64
	CollectCount int64
}

type UserArticleStatsCache interface {
	Get(ctx context.Context, uid int64) (ArticleStats, error)
	Set(ctx context.Context, uid int64, stats ArticleStats) error
	Del(ctx context.Context, uid int64) error
}

type UserProfileCacheInvalidator interface {
	DelPublicProfiles(ctx context.Context, uids ...int64) error
	DelArticleStats(ctx context.Context, uid int64) error
}

type redisPublicProfileCache struct {
	client redis.Cmdable
}
type redisUserArticleStatsCache struct {
	client redis.Cmdable
}
type redisUserProfileCacheInvalidator struct {
	client redis.Cmdable
}

func NewPublicProfileCache(client redis.Cmdable) PublicProfileCache {
	return &redisPublicProfileCache{
		client: client,
	}
}

func NewUserArticleStatsCache(client redis.Cmdable) UserArticleStatsCache {
	return &redisUserArticleStatsCache{
		client: client,
	}
}

func NewUserProfileCacheInvalidator(client redis.Cmdable) UserProfileCacheInvalidator {
	return &redisUserProfileCacheInvalidator{
		client: client,
	}
}

func (c *redisUserProfileCacheInvalidator) DelPublicProfiles(
	ctx context.Context,
	uids ...int64,
) error {
	if len(uids) == 0 {
		return nil
	}
	keys := make([]string, len(uids))
	for i, uid := range uids {
		keys[i] = publicProfileKey(uid)
	}
	ctx = observability.WithCacheName(ctx, cacheNamePublicProfile)
	return c.client.Del(ctx, keys...).Err()
}

func (c *redisUserProfileCacheInvalidator) DelArticleStats(ctx context.Context, uid int64) error {
	ctx = observability.WithCacheName(ctx, cacheNameUserArticleStats)
	return c.client.Del(ctx, articleStatsKey(uid)).Err()
}

func (c *redisPublicProfileCache) Get(ctx context.Context, uid int64) (PublicProfile, error) {
	ctx = observability.WithCacheName(ctx, cacheNamePublicProfile)
	data, err := c.client.Get(ctx, publicProfileKey(uid)).Bytes()
	if err != nil {
		return PublicProfile{}, err
	}
	if string(data) == cachedNotFoundValue {
		return PublicProfile{}, ErrCachedNotFound
	}
	var profile PublicProfile
	err = json.Unmarshal(data, &profile)
	return profile, err
}

func (c *redisPublicProfileCache) Set(ctx context.Context, profile PublicProfile) error {
	// 公开资料缓存不得包含邮箱和密码哈希。
	profile.User.Email = ""
	profile.User.Password = ""
	data, err := json.Marshal(profile)
	if err != nil {
		return err
	}
	ctx = observability.WithCacheName(ctx, cacheNamePublicProfile)
	return c.client.Set(ctx, publicProfileKey(profile.User.Id), data, withJitter(publicProfileTTL, publicProfileTTLJitter)).
		Err()
}

func (c *redisPublicProfileCache) SetNotFound(ctx context.Context, uid int64) error {
	ctx = observability.WithCacheName(ctx, cacheNamePublicProfile)
	return c.client.Set(ctx, publicProfileKey(uid), cachedNotFoundValue, negativeCacheTTL).Err()
}

func (c *redisPublicProfileCache) Del(ctx context.Context, uids ...int64) error {
	if len(uids) == 0 {
		return nil
	}
	keys := make([]string, len(uids))
	for i, uid := range uids {
		keys[i] = publicProfileKey(uid)
	}
	ctx = observability.WithCacheName(ctx, cacheNamePublicProfile)
	return c.client.Del(ctx, keys...).Err()
}

func (c *redisUserArticleStatsCache) Get(ctx context.Context, uid int64) (ArticleStats, error) {
	ctx = observability.WithCacheName(ctx, cacheNameUserArticleStats)
	data, err := c.client.Get(ctx, articleStatsKey(uid)).Bytes()
	if err != nil {
		return ArticleStats{}, err
	}
	var stats ArticleStats
	err = json.Unmarshal(data, &stats)
	return stats, err
}

func (c *redisUserArticleStatsCache) Set(ctx context.Context, uid int64, stats ArticleStats) error {
	data, err := json.Marshal(stats)
	if err != nil {
		return err
	}
	ctx = observability.WithCacheName(ctx, cacheNameUserArticleStats)
	return c.client.Set(ctx, articleStatsKey(uid), data, withJitter(articleStatsTTL, articleStatsTTLJitter)).
		Err()
}

func (c *redisUserArticleStatsCache) Del(ctx context.Context, uid int64) error {
	ctx = observability.WithCacheName(ctx, cacheNameUserArticleStats)
	return c.client.Del(ctx, articleStatsKey(uid)).Err()
}

func publicProfileKey(uid int64) string {
	return fmt.Sprintf("user:public_profile:%s:%d", cacheKeyVersion, uid)
}

func articleStatsKey(uid int64) string {
	return fmt.Sprintf("user:article_stats:%s:%d", cacheKeyVersion, uid)
}
