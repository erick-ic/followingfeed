package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"followingfeed/internal/domain"
	"followingfeed/internal/observability"

	"github.com/redis/go-redis/v9"
)

var (
	ErrNotExists      = redis.Nil
	ErrCachedNotFound = errors.New("缓存中不存在该用户")
)

const cachedNotFoundValue = "__not_found__"

const cacheNameUserProfile = "user_profile"

type UserCache interface {
	Get(ctx context.Context, id int64) (domain.User, error)
	Set(ctx context.Context, u domain.User) error
	SetNotFound(ctx context.Context, id int64) error
}

type RedisUserCache struct {
	client redis.Cmdable
}

// cachedUser 是用户缓存专用的数据结构，不保存密码等敏感字段。
type cachedUser struct {
	Id        int64
	Nickname  string
	Email     string
	CreatedAt int64
	UpdatedAt int64
}

func (uc *RedisUserCache) Get(ctx context.Context, id int64) (domain.User, error) {
	key := uc.key(id)
	ctx = observability.WithCacheName(ctx, cacheNameUserProfile)
	val, err := uc.client.Get(ctx, key).Bytes()
	if err != nil {
		return domain.User{}, err
	}
	if string(val) == cachedNotFoundValue {
		return domain.User{}, ErrCachedNotFound
	}

	var cached cachedUser
	if err = json.Unmarshal(val, &cached); err != nil {
		return domain.User{}, err
	}
	return domain.User{
		Id:        cached.Id,
		Nickname:  cached.Nickname,
		Email:     cached.Email,
		CreatedAt: cached.CreatedAt,
		UpdatedAt: cached.UpdatedAt,
	}, nil
}

func (uc *RedisUserCache) Set(ctx context.Context, u domain.User) error {
	val, err := json.Marshal(cachedUser{
		Id:        u.Id,
		Nickname:  u.Nickname,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	})
	if err != nil {
		return err
	}
	key := uc.key(u.Id)
	ctx = observability.WithCacheName(ctx, cacheNameUserProfile)
	return uc.client.Set(ctx, key, val, withJitter(userTTL, userTTLJitter)).Err()
}

func (uc *RedisUserCache) SetNotFound(ctx context.Context, id int64) error {
	// 数据库确认用户不存在后写入短期占位值，拦截针对同一不存在 ID 的重复数据库查询。
	return uc.client.Set(
		observability.WithCacheName(ctx, cacheNameUserProfile),
		uc.key(id),
		cachedNotFoundValue,
		negativeCacheTTL,
	).Err()
}

func NewUserCache(client redis.Cmdable) UserCache {
	return &RedisUserCache{
		client: client,
	}
}

func (uc *RedisUserCache) key(id int64) string {
	return fmt.Sprintf("user:info:%s:%d", cacheKeyVersion, id)
}
