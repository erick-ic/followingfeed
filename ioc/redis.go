package ioc

import (
	"context"
	"crypto/tls"
	"fmt"
	"followingfeed/config"
	"followingfeed/internal/observability"
	"time"

	"github.com/redis/go-redis/v9"
)

// InitRedis 创建 Redis 客户端，挂载业务缓存观测钩子，并在启动阶段验证连接可用性。
func InitRedis(cfg config.Config, metrics *observability.Metrics) (redis.Cmdable, error) {
	options := &redis.Options{
		Addr: cfg.Redis.Addr, Username: cfg.Redis.Username, Password: cfg.Redis.Password,
		DB: cfg.Redis.DB, PoolSize: 5, MinIdleConns: 1,
		DialTimeout: 3 * time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second,
		ContextTimeoutEnabled: true,
	}
	if cfg.Redis.TLS {
		options.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	client := redis.NewClient(options)
	// 钩子只记录带业务缓存名称的操作，避免把具体缓存键写入指标标签。
	client.AddHook(metrics.RedisHook())
	// 使用短超时快速暴露地址配置、网络或认证问题。
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		// 探测失败时释放客户端资源，避免初始化失败路径遗留连接。
		_ = client.Close()
		return nil, fmt.Errorf("连接 Redis 失败：%w", err)
	}
	return client, nil
}
