package jwt

import (
	"github.com/gin-gonic/gin"
)

type JWTHandler interface {
	SetLoginToken(ctx *gin.Context, uid int64) error
	SetJWTToken(ctx *gin.Context, uid int64, Ssid string) error
	ClearToken(ctx *gin.Context) error
	CheckSession(ctx *gin.Context, Ssid string) error
	ExtractToken(ctx *gin.Context) string
	ParseAccessToken(token string) (*UserClaims, error)
	ParseRefreshToken(token string) (*RefreshClaims, error)
}
