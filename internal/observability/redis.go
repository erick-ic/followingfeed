package observability

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisMetricsHook struct {
	metrics *Metrics
}

type cacheNameContextKey struct{}

// WithCacheName 将 Redis 操作标记为名称范围有限的业务缓存操作，
// 使 Redis 钩子无需记录键或标识符即可提供有意义的命中率。
func WithCacheName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, cacheNameContextKey{}, name)
}

func (m *Metrics) RedisHook() redis.Hook {
	return &redisMetricsHook{metrics: m}
}

func (h *redisMetricsHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return next(ctx, network, addr)
	}
}

func (h *redisMetricsHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmd)
		h.observe(ctx, strings.ToLower(cmd.Name()), start, err)
		return err
	}
}

func (h *redisMetricsHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmds)
		h.observe(ctx, "pipeline", start, err)
		return err
	}
}

func (h *redisMetricsHook) observe(ctx context.Context, command string, start time.Time, err error) {
	cacheName, _ := ctx.Value(cacheNameContextKey{}).(string)
	if cacheName == "" {
		return
	}
	duration := time.Since(start).Seconds()
	result := "success"
	if errors.Is(err, redis.Nil) {
		result = "miss"
	} else if err != nil {
		result = "error"
	}
	operation := cacheOperation(command)
	cacheResult := result
	if operation == "read" && result == "success" {
		cacheResult = "hit"
	}
	h.metrics.cacheOperations.WithLabelValues(cacheName, operation, cacheResult).Inc()
	h.metrics.cacheDuration.WithLabelValues(cacheName, operation).Observe(duration)
}

func cacheOperation(command string) string {
	switch command {
	case "get", "mget", "hget", "hmget":
		return "read"
	case "set", "mset", "hset":
		return "write"
	case "del", "unlink":
		return "invalidate"
	default:
		return "other"
	}
}
