package cache

import (
	"math/rand/v2"
	"time"
)

const (
	cacheKeyVersion = "v1"

	articleListTTL         = 10 * time.Second
	userTTL                = 15 * time.Minute
	userTTLJitter          = 2 * time.Minute
	publicProfileTTL       = 5 * time.Minute
	publicProfileTTLJitter = time.Minute
	articleStatsTTL        = 30 * time.Second
	articleStatsTTLJitter  = 30 * time.Second
	negativeCacheTTL       = time.Minute

	ReadTimeout             = 100 * time.Millisecond
	WriteTimeout            = 200 * time.Millisecond
	ArticleOperationTimeout = time.Second
)

func withJitter(base, jitter time.Duration) time.Duration {
	return base + time.Duration(rand.Int64N(int64(jitter)+1))
}
