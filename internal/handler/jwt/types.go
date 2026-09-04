package jwt

import (
	"context"

	"github.com/gin-gonic/gin"
)

type JWTHandler interface {
	SetLoginToken(ctx *gin.Context, uid int64) error
	SetJWTToken(ctx *gin.Context, uid int64, Ssid string) error
	ClearToken(ctx *gin.Context, ssid string) error
	ClearRefreshToken(ctx *gin.Context)
	CheckSession(ctx context.Context, Ssid string) error
	ExtractToken(ctx *gin.Context) string
	ExtractRefreshToken(ctx *gin.Context) string
	ParseAccessToken(token string) (*UserClaims, error)
	ParseRefreshToken(token string) (*RefreshClaims, error)
}
