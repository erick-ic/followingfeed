package cache

import (
	"context"
	"time"

	"github.com/ecodeclub/ekit"
)

type Cache interface {
	Set(ctx context.Context, key string, value any, expiration time.Duration) error
	Get(ctx context.Context, key string) ekit.AnyValue
}
