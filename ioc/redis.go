package ioc

import (
	"context"
	"fmt"
	"followingfeed/config"
	"time"

	"github.com/redis/go-redis/v9"
)

func InitRedis(cfg config.Config) (redis.Cmdable, error) {
	client := redis.NewClient(
		&redis.Options{
			Addr: cfg.Redis.Addr,
		})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return client, nil
}
