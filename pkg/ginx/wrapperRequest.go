package ginx

import (
	"followingfeed/pkg/logger"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Result struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

func WrapperReq[T any](l logger.LoggerV1, fn func(ctx *gin.Context, req T) (Result, error)) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req T
		if err := ctx.Bind(&req); err != nil {
			return
		}

		//执行业务逻辑
		res, err := fn(ctx, req)
		if err != nil {
			l.Error("处理业务逻辑失败！", logger.Error(err))
		}
		ctx.JSON(http.StatusOK, res)
	}
}

func WrapperReqWithToken[T any, Cls jwt.Claims](l logger.LoggerV1, fn func(ctx *gin.Context, reqBody T, cls Cls) (Result, error)) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var reqBody T
		if err := ctx.Bind(&reqBody); err != nil {
			return
		}

		val, ok := ctx.Get("claims")
		if !ok {
			// claims 不存在
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c, ok := val.(Cls)
		if !ok {
			//类型不匹配
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		res, err := fn(ctx, reqBody, c)
		if err != nil {
			l.Error("处理业务逻辑失败！", logger.Error(err))
		}

		ctx.JSON(http.StatusOK, res)
	}
}
