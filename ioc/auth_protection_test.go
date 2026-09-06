package ioc

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"followingfeed/config"
	"followingfeed/internal/repository/cache/redismocks"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAuthProtectionClosedFeaturesDoNotReachHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := gin.New()
	s.Use(authProtection(config.AuthConfig{MaxConcurrent: 1, RateWindow: time.Minute, RateThreshold: 10}, nil))
	for _, path := range []string{"/api/v1/users/signup", "/api/v1/articles/publish"} {
		s.POST(path, func(ctx *gin.Context) { t.Fatal("closed endpoint executed") })
	}
	s.GET("/api/v1/pub/list", func(ctx *gin.Context) { ctx.Status(200) })
	for _, path := range []string{"/api/v1/users/signup", "/api/v1/articles/publish"} {
		r := httptest.NewRecorder()
		s.ServeHTTP(r, httptest.NewRequest("POST", path, nil))
		require.Equal(t, http.StatusForbidden, r.Code)
	}
	r := httptest.NewRecorder()
	s.ServeHTTP(r, httptest.NewRequest("GET", "/api/v1/pub/list", nil))
	require.Equal(t, 200, r.Code)
}

func TestAuthConcurrencyLimitRejectsWithoutQueueAndReleasesSlot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := redismocks.NewMockCmdable(gomock.NewController(t))
	client.EXPECT().Eval(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(redis.NewCmdResult(false, nil)).Times(2)
	s := gin.New()
	s.Use(authProtection(config.AuthConfig{SignupEnabled: true, MaxConcurrent: 1, RateWindow: time.Minute, RateThreshold: 10}, client))
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	s.POST("/api/v1/users/login", func(ctx *gin.Context) {
		if ctx.Query("block") == "1" {
			close(entered)
			<-release
		}
		ctx.Status(200)
	})
	go func() {
		defer close(done)
		s.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/api/v1/users/login?block=1", nil))
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}
	r := httptest.NewRecorder()
	s.ServeHTTP(r, httptest.NewRequest("POST", "/api/v1/users/login", nil))
	require.Equal(t, 429, r.Code)
	close(release)
	<-done
	r = httptest.NewRecorder()
	s.ServeHTTP(r, httptest.NewRequest("POST", "/api/v1/users/login", nil))
	require.Equal(t, 200, r.Code)
}

func TestAuthRateLimitRejectsBeforePasswordWork(t *testing.T) {
	client := redismocks.NewMockCmdable(gomock.NewController(t))
	client.EXPECT().Eval(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(redis.NewCmdResult(true, nil))
	s := gin.New()
	s.Use(authProtection(config.AuthConfig{MaxConcurrent: 1, RateWindow: time.Minute, RateThreshold: 10}, client))
	s.POST("/api/v1/users/login", func(ctx *gin.Context) { t.Fatal("rate limited handler executed") })
	r := httptest.NewRecorder()
	s.ServeHTTP(r, httptest.NewRequest("POST", "/api/v1/users/login", nil))
	require.Equal(t, 429, r.Code)
	require.Equal(t, "60", r.Header().Get("Retry-After"))
}
