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

// RedisJWTHandler 负责签发、解析 JWT，并通过 Redis 维护会话注销黑名单。
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

// ClearToken 注销当前会话：清空返回给客户端的令牌，并将当前 SSID 写入 Redis 黑名单。
// 黑名单记录保留 7 天，与刷新令牌的有效期一致，防止已注销的令牌在过期前再次使用。
func (rj *RedisJWTHandler) ClearToken(ctx *gin.Context, ssid string) error {
	// 清空响应头中的访问令牌，并让浏览器删除 HttpOnly 刷新令牌 Cookie。
	ctx.Header("X-JWT-Token", "")
	rj.ClearRefreshToken(ctx)

	// SSID 是会话黑名单键的一部分，为空时不能生成有效的注销记录。
	if ssid == "" {
		return fmt.Errorf("%w: SSID 为空", ErrInvalidClaims)
	}

	// 以 SSID 为键写入注销黑名单；CheckSession 会据此拒绝该会话后续的请求。
	// Redis 写入失败时将错误交给上层处理。
	/*
		ctx：传递请求的取消和超时信号；
		key：users:ssid:<ssid>，唯一标识当前登录会话；
		value：空字符串，因为这里只需要判断 Key 是否存在；
		expiration：与刷新令牌的有效期保持一致。
	*/
	return rj.cmd.Set(
		ctx.Request.Context(),
		fmt.Sprintf("users:ssid:%s", ssid),
		"",
		refreshTokenLifetime,
	).Err()
}

// CheckSession 检查 ssid 对应的登录会话是否已被加入 Redis 注销黑名单。
func (rj *RedisJWTHandler) CheckSession(ctx context.Context, Ssid string) error {
	// users:ssid:<ssid> 是注销黑名单：key 存在表示该会话已经退出登录。
	cnt, err := rj.cmd.Exists(ctx, fmt.Sprintf("users:ssid:%s", Ssid)).Result()
	if err != nil {
		return err
	}
	if cnt > 0 {
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
