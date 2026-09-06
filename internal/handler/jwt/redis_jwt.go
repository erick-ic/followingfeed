package jwt

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"followingfeed/config"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	tokenIssuer            = "followingfeed"
	accessTokenAudience    = "followingfeed-api"
	refreshTokenAudience   = "followingfeed-token"
	accessTokenLifetime    = 15 * time.Minute
	refreshTokenLifetime   = 7 * 24 * time.Hour
	refreshTokenCookieName = "followingfeed_refresh_token"
	refreshTokenCookiePath = "/api/v1/users"
	tokenClockSkew         = 30 * time.Second
)

var (
	ErrSessionRevoked = errors.New("登录会话已注销")  // 令牌所属的登录会话已被注销。
	ErrInvalidClaims  = errors.New("JWT 声明无效") // 令牌声明缺失、无效或与预期身份不一致。
)

// RedisJWTHandler 负责签发、解析 JWT，并通过 Redis 维护有效会话白名单。
type RedisJWTHandler struct {
	cmd             redis.Cmdable // 读写 Redis 中的会话注销记录。
	accessTokenKey  []byte        // 签发和验证访问令牌的 HMAC 密钥。
	refreshTokenKey []byte        // 签发和验证刷新令牌的 HMAC 密钥。
	secureCookies   bool          // 决定刷新令牌 Cookie 是否仅允许通过 HTTPS 发送。
}

// NewRedisJWTHandler 根据 Redis 客户端和应用配置创建 JWT 处理器。
func NewRedisJWTHandler(cmd redis.Cmdable, cfg config.Config) JWTHandler {
	return &RedisJWTHandler{
		cmd:             cmd,
		accessTokenKey:  []byte(cfg.JWT.AccessTokenKey),
		refreshTokenKey: []byte(cfg.JWT.RefreshTokenKey),
		// 配置默认环境是 development；其他环境均默认只通过 HTTPS 发送刷新 Cookie。
		secureCookies: cfg.Env != "development",
	}
}

// UserClaims 是访问令牌携带的声明。
type UserClaims struct {
	jwt.RegisteredClaims        // RegisteredClaims 包含签发者、受众、有效期、用户标识及令牌 ID 等标准声明。
	Uid                  int64  `json:"uid"`  // 当前用户的唯一标识，必须与 RegisteredClaims.Subject 一致。
	Ssid                 string `json:"ssid"` // 登录会话标识，用于检查该会话是否已被注销。
}

// RefreshClaims 是刷新令牌携带的声明。
type RefreshClaims struct {
	jwt.RegisteredClaims
	Uid  int64  `json:"uid"`
	Ssid string `json:"ssid"` // 登录会话标识，新访问令牌沿用该标识以保持同一登录会话。
}

// SetLoginToken 为用户创建新会话，并同时向响应写入访问令牌和刷新令牌。
func (rj *RedisJWTHandler) SetLoginToken(ctx *gin.Context, uid int64) error {
	ssid := uuid.New().String()
	// 先登记会话，写入失败时不得向客户端签发令牌。
	if err := rj.cmd.Set(ctx.Request.Context(), sessionKey(ssid), "1", refreshTokenLifetime).Err(); err != nil {
		return fmt.Errorf("登记登录会话失败：%w", err)
	}
	err := rj.SetJWTToken(ctx, uid, ssid)
	if err != nil {
		return err
	}

	err = rj.setRefreshToken(ctx, uid, ssid)
	if err != nil {
		return err
	}
	return nil
}

// SetJWTToken 签发访问令牌，并通过 X-JWT-Token 响应头返回给客户端。
// uid 是用户唯一标识，ssid 是令牌所属的登录会话标识。
func (rj *RedisJWTHandler) SetJWTToken(ctx *gin.Context, uid int64, Ssid string) error {
	now := time.Now()
	claims := UserClaims{
		RegisteredClaims: newRegisteredClaims(now, uid, accessTokenAudience, accessTokenLifetime),
		Uid:              uid,
		Ssid:             Ssid,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenStr, err := token.SignedString(rj.accessTokenKey)
	if err != nil {
		return err
	}
	// 访问令牌由前端保存在内存中，通过自定义响应头返回。
	ctx.Header("x-jwt-token", tokenStr)
	return nil
}

// ParseAccessToken 验证并解析访问令牌字符串。
func (rj *RedisJWTHandler) ParseAccessToken(tokenStr string) (*UserClaims, error) {
	claims := &UserClaims{}
	token, err := jwt.ParseWithClaims(
		tokenStr,
		claims,
		rj.keyFunc(rj.accessTokenKey),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(tokenIssuer),
		jwt.WithAudience(accessTokenAudience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(tokenClockSkew),
	)
	if err != nil || token == nil || !token.Valid || !validIdentity(claims.RegisteredClaims, claims.Uid) || claims.Ssid == "" {
		return nil, ErrInvalidClaims
	}
	return claims, nil
}

// ParseRefreshToken 验证并解析刷新令牌字符串。
func (rj *RedisJWTHandler) ParseRefreshToken(tokenStr string) (*RefreshClaims, error) {
	claims := &RefreshClaims{}
	token, err := jwt.ParseWithClaims(
		tokenStr,
		claims,
		rj.keyFunc(rj.refreshTokenKey),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(tokenIssuer),
		jwt.WithAudience(refreshTokenAudience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(tokenClockSkew),
	)
	if err != nil || token == nil || !token.Valid || !validIdentity(claims.RegisteredClaims, claims.Uid) || claims.Ssid == "" {
		return nil, ErrInvalidClaims
	}
	return claims, nil
}

func (rj *RedisJWTHandler) keyFunc(key []byte) jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, ErrInvalidClaims
		}
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidClaims
		}
		return key, nil
	}
}

// ClearToken 删除有效会话。键缺失一律拒绝，Redis 数据丢失不会复活旧令牌。
func (rj *RedisJWTHandler) ClearToken(ctx *gin.Context, ssid string) error {
	if ssid == "" {
		return fmt.Errorf("%w: SSID 为空", ErrInvalidClaims)
	}
	// 撤销成功后才清 Cookie；依赖故障时客户端仍可重试注销。
	if err := rj.cmd.Del(ctx.Request.Context(), sessionKey(ssid)).Err(); err != nil {
		return err
	}
	ctx.Header("X-JWT-Token", "")
	rj.ClearRefreshToken(ctx)
	return nil
}

func sessionKey(ssid string) string { return "users:session:v2:" + ssid }

// CheckSession 要求会话仍明确存在；旧版本黑名单式令牌也会被拒绝。
func (rj *RedisJWTHandler) CheckSession(ctx context.Context, ssid string) error {
	if ssid == "" {
		return ErrSessionRevoked
	}
	cnt, err := rj.cmd.Exists(ctx, sessionKey(ssid)).Result()
	if err != nil {
		return err
	}
	if cnt != 1 {
		return ErrSessionRevoked
	}
	return nil
}

// ExtractToken 从 Authorization 请求头提取 Bearer 访问令牌。
func (rj *RedisJWTHandler) ExtractToken(ctx *gin.Context) string {
	// 访问令牌采用“Authorization: Bearer <token>”格式。
	parts := strings.Fields(ctx.GetHeader("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

// ExtractRefreshToken 从 HttpOnly Cookie 读取刷新令牌，避免令牌暴露给浏览器脚本。
func (rj *RedisJWTHandler) ExtractRefreshToken(ctx *gin.Context) string {
	token, err := ctx.Cookie(refreshTokenCookieName)
	if err != nil {
		return ""
	}
	return token
}

// setRefreshToken 签发刷新令牌，并将其写入 HttpOnly Cookie。
func (rj *RedisJWTHandler) setRefreshToken(ctx *gin.Context, uid int64, Ssid string) error {
	now := time.Now()
	claims := RefreshClaims{
		RegisteredClaims: newRegisteredClaims(now, uid, refreshTokenAudience, refreshTokenLifetime),
		Uid:              uid,
		Ssid:             Ssid,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenStr, err := token.SignedString(rj.refreshTokenKey)
	if err != nil {
		return err
	}
	rj.setRefreshTokenCookie(ctx, tokenStr, now.Add(refreshTokenLifetime), int(refreshTokenLifetime.Seconds()))
	return nil
}

// ClearRefreshToken 删除浏览器保存的刷新令牌，不影响 Redis 中的会话状态。
func (rj *RedisJWTHandler) ClearRefreshToken(ctx *gin.Context) {
	rj.setRefreshTokenCookie(ctx, "", time.Unix(1, 0), -1)
}

// setRefreshTokenCookie 使用统一的安全属性设置或删除刷新令牌 Cookie。
func (rj *RedisJWTHandler) setRefreshTokenCookie(
	ctx *gin.Context,
	value string,
	expires time.Time,
	maxAge int,
) {
	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:     refreshTokenCookieName,
		Value:    value,
		Path:     refreshTokenCookiePath,
		Expires:  expires,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   rj.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

// newRegisteredClaims 创建包含身份、受众和有效期的标准 JWT 声明。
func newRegisteredClaims(now time.Time, uid int64, audience string, lifetime time.Duration) jwt.RegisteredClaims {
	return jwt.RegisteredClaims{
		Issuer:    tokenIssuer,
		Subject:   strconv.FormatInt(uid, 10),
		Audience:  jwt.ClaimStrings{audience},
		ExpiresAt: jwt.NewNumericDate(now.Add(lifetime)),
		NotBefore: jwt.NewNumericDate(now),
		IssuedAt:  jwt.NewNumericDate(now),
		ID:        uuid.NewString(),
	}
}

// validIdentity 校验自定义用户 ID 与标准 Subject 是否一致，并确认令牌 ID 非空。
func validIdentity(claims jwt.RegisteredClaims, uid int64) bool {
	return uid > 0 && claims.Subject == strconv.FormatInt(uid, 10) && claims.ID != ""
}
