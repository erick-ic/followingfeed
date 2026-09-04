package ratelimit

import (
	_ "embed"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Builder struct {
	prefix   string
	cmd      redis.Cmdable
	interval time.Duration
	// 阈值
	rate               int
	ignorePaths        map[string]struct{}
	ignorePathPrefixes []string
	newMember          func() string
}

//go:embed slide_window.lua
var luaScript string

func NewBuilder(cmd redis.Cmdable, interval time.Duration, rate int) *Builder {
	return &Builder{
		cmd:         cmd,
		prefix:      "ip-limiter",
		interval:    interval,
		rate:        rate,
		ignorePaths: make(map[string]struct{}),
		newMember:   uuid.NewString,
	}
}

func (b *Builder) IgnorePaths(paths ...string) *Builder {
	for _, path := range paths {
		b.ignorePaths[path] = struct{}{}
	}
	return b
}

func (b *Builder) IgnorePathPrefix(prefixes ...string) *Builder {
	b.ignorePathPrefixes = append(b.ignorePathPrefixes, prefixes...)
	return b
}

func (b *Builder) Prefix(prefix string) *Builder {
	b.prefix = prefix
	return b
}

func (b *Builder) Build() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if b.shouldIgnore(ctx.Request.URL.Path) {
			ctx.Next()
			return
		}
		limited, err := b.limit(ctx)
		if err != nil {
			_ = ctx.Error(fmt.Errorf("限流器依赖异常：%w", err))
			ctx.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		if limited {
			ctx.Set("security_event", "rate_limit.exceeded")
			ctx.AbortWithStatus(http.StatusTooManyRequests)
			return
		}
		ctx.Next()
	}
}

func (b *Builder) shouldIgnore(path string) bool {
	if _, ok := b.ignorePaths[path]; ok {
		return true
	}
	for _, prefix := range b.ignorePathPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func (b *Builder) limit(ctx *gin.Context) (bool, error) {
	key := fmt.Sprintf("%s:%s", b.prefix, ctx.ClientIP())
	return b.cmd.Eval(ctx, luaScript, []string{key},
		b.interval.Milliseconds(), b.rate, time.Now().UnixMilli(), b.newMember()).Bool()
}
