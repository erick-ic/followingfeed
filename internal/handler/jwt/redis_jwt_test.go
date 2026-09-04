package jwt

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"followingfeed/config"

	"github.com/gin-gonic/gin"
	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisJWTHandlerIssuesStandardClaimsAndRefreshCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newTestJWTHandler("production")
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/users/login", nil)

	require.NoError(t, handler.SetLoginToken(ctx, 42))

	accessToken := recorder.Header().Get("X-JWT-Token")
	require.NotEmpty(t, accessToken)
	accessClaims, err := handler.ParseAccessToken(accessToken)
	require.NoError(t, err)
	assert.Equal(t, int64(42), accessClaims.Uid)
	assert.Equal(t, tokenIssuer, accessClaims.Issuer)
	assert.Equal(t, "42", accessClaims.Subject)
	assert.Contains(t, accessClaims.Audience, accessTokenAudience)
	assert.NotEmpty(t, accessClaims.ID)
	assert.WithinDuration(t, time.Now().Add(accessTokenLifetime), accessClaims.ExpiresAt.Time, 2*time.Second)

	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	refreshCookie := cookies[0]
	assert.Equal(t, refreshTokenCookieName, refreshCookie.Name)
	assert.Equal(t, refreshTokenCookiePath, refreshCookie.Path)
	assert.True(t, refreshCookie.HttpOnly)
	assert.True(t, refreshCookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, refreshCookie.SameSite)
	assert.Equal(t, int(refreshTokenLifetime.Seconds()), refreshCookie.MaxAge)

	refreshClaims, err := handler.ParseRefreshToken(refreshCookie.Value)
	require.NoError(t, err)
	assert.Equal(t, int64(42), refreshClaims.Uid)
	assert.Equal(t, accessClaims.Ssid, refreshClaims.Ssid)
	assert.Contains(t, refreshClaims.Audience, refreshTokenAudience)
	assert.WithinDuration(t, time.Now().Add(refreshTokenLifetime), refreshClaims.ExpiresAt.Time, 2*time.Second)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/users/refreshToken", nil)
	request.AddCookie(refreshCookie)
	ctx.Request = request
	assert.Equal(t, refreshCookie.Value, handler.ExtractRefreshToken(ctx))
}

func TestRedisJWTHandlerRejectsWrongRefreshAudience(t *testing.T) {
	handler := newTestJWTHandler("development")
	now := time.Now()
	claims := RefreshClaims{
		RegisteredClaims: newRegisteredClaims(now, 1, accessTokenAudience, refreshTokenLifetime),
		Uid:              1,
		Ssid:             "ssid-1",
	}
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(handler.refreshTokenKey)
	require.NoError(t, err)

	_, err = handler.ParseRefreshToken(tokenString)
	assert.ErrorIs(t, err, ErrInvalidClaims)
}

func TestRedisJWTHandlerClearsRefreshCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newTestJWTHandler("development")
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	handler.ClearRefreshToken(ctx)

	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, refreshTokenCookieName, cookies[0].Name)
	assert.Equal(t, -1, cookies[0].MaxAge)
	assert.True(t, cookies[0].HttpOnly)
	assert.False(t, cookies[0].Secure)
}

func newTestJWTHandler(env string) *RedisJWTHandler {
	return NewRedisJWTHandler(nil, config.Config{
		Env: env,
		JWT: config.JWTConfig{
			AccessTokenKey:  "access-token-key-at-least-32-characters",
			RefreshTokenKey: "refresh-token-key-at-least-32-characters",
		},
	}).(*RedisJWTHandler)
}
