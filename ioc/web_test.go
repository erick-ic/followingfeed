package ioc

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"followingfeed/config"
	"followingfeed/internal/repository/cache/redismocks"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestHealthRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("存活探针不检查外部依赖", func(t *testing.T) {
		server := gin.New()
		registerHealthRoutes(server, nil, nil)

		resp := httptest.NewRecorder()
		server.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, livePath, nil))

		assert.Equal(t, http.StatusOK, resp.Code)
	})

	t.Run("依赖正常时就绪", func(t *testing.T) {
		db, sqlMock := newHealthTestDB(t)
		sqlMock.ExpectPing()
		redisClient := redismocks.NewMockCmdable(gomock.NewController(t))
		redisClient.EXPECT().Ping(gomock.Any()).Return(redis.NewStatusResult("PONG", nil))

		server := gin.New()
		registerHealthRoutes(server, db, redisClient)
		resp := httptest.NewRecorder()
		server.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, readyPath, nil))

		assert.Equal(t, http.StatusOK, resp.Code)
		require.NoError(t, sqlMock.ExpectationsWereMet())
	})

	t.Run("MySQL 异常时未就绪", func(t *testing.T) {
		db, sqlMock := newHealthTestDB(t)
		sqlMock.ExpectPing().WillReturnError(errors.New("mysql unavailable"))
		redisClient := redismocks.NewMockCmdable(gomock.NewController(t))

		server := gin.New()
		registerHealthRoutes(server, db, redisClient)
		resp := httptest.NewRecorder()
		server.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, readyPath, nil))

		assert.Equal(t, http.StatusServiceUnavailable, resp.Code)
		require.NoError(t, sqlMock.ExpectationsWereMet())
	})

	t.Run("Redis 异常时未就绪", func(t *testing.T) {
		db, sqlMock := newHealthTestDB(t)
		sqlMock.ExpectPing()
		redisClient := redismocks.NewMockCmdable(gomock.NewController(t))
		redisClient.EXPECT().Ping(gomock.Any()).Return(redis.NewStatusResult("", errors.New("redis unavailable")))

		server := gin.New()
		registerHealthRoutes(server, db, redisClient)
		resp := httptest.NewRecorder()
		server.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, readyPath, nil))

		assert.Equal(t, http.StatusServiceUnavailable, resp.Code)
		require.NoError(t, sqlMock.ExpectationsWereMet())
	})
}

func TestLimitRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := gin.New()
	server.Use(limitRequestBody(4))
	server.POST("/", func(ctx *gin.Context) {
		var body map[string]any
		if err := ctx.ShouldBindJSON(&body); err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				ctx.Status(http.StatusRequestEntityTooLarge)
				return
			}
			ctx.Status(http.StatusBadRequest)
			return
		}
		ctx.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"too long"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	server.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.Code)
}

func TestHandleCorsAllowsConfiguredOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := gin.New()
	server.Use(handleCors(config.CORSConfig{AllowedOrigins: []string{"https://web.example"}}))
	server.POST("/resource", func(ctx *gin.Context) { ctx.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodOptions, "/resource", nil)
	req.Header.Set("Origin", "https://web.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	resp := httptest.NewRecorder()
	server.ServeHTTP(resp, req)

	assert.Equal(t, "https://web.example", resp.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", resp.Header().Get("Access-Control-Allow-Credentials"))
}

func newHealthTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, sqlMock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	// sqlmock 未设置 Close 期望时会返回错误；这里仍关闭资源，但不把该模拟行为当作探针断言。
	t.Cleanup(func() { _ = sqlDB.Close() })

	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DisableAutomaticPing: true})
	require.NoError(t, err)
	return db, sqlMock
}

func TestEmptyDevelopmentCORSDoesNotPanicOrAllowCrossOrigin(t *testing.T) {
	server := gin.New()
	server.Use(handleCors(config.CORSConfig{}))
	server.GET("/", func(ctx *gin.Context) { ctx.Status(200) })
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://untrusted.example")
	resp := httptest.NewRecorder()
	server.ServeHTTP(resp, req)
	assert.Equal(t, 200, resp.Code)
	assert.Empty(t, resp.Header().Get("Access-Control-Allow-Origin"))
}
