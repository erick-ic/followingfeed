package ratelimit

import (
	"net/http/httptest"
	"testing"
	"time"

	"followingfeed/internal/repository/cache/redismocks"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestBuilderShouldIgnoreOperationalPaths(t *testing.T) {
	builder := NewBuilder(nil, time.Second, 100).
		IgnorePaths("/health/live", "/health/ready").
		IgnorePathPrefix("/assets/")

	assert.True(t, builder.shouldIgnore("/health/live"))
	assert.True(t, builder.shouldIgnore("/assets/app.css"))
	assert.False(t, builder.shouldIgnore("/api/v1/users/login"))
}

func TestBuilderUsesUniqueMemberForEveryRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cmd := redismocks.NewMockCmdable(gomock.NewController(t))
	builder := NewBuilder(cmd, time.Second, 100)
	members := []string{"request-1", "request-2"}
	builder.newMember = func() string {
		member := members[0]
		members = members[1:]
		return member
	}

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/", nil)
	ctx.Request.RemoteAddr = "192.0.2.1:1234"
	for _, member := range []string{"request-1", "request-2"} {
		cmd.EXPECT().Eval(
			gomock.Any(), luaScript, []string{"ip-limiter:192.0.2.1"},
			int64(1000), 100, gomock.Any(), member,
		).Return(redis.NewCmdResult("false", nil))
	}

	limited, err := builder.limit(ctx)
	require.NoError(t, err)
	assert.False(t, limited)
	limited, err = builder.limit(ctx)
	require.NoError(t, err)
	assert.False(t, limited)
}
