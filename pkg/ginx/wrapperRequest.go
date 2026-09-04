package ginx

import (
	"errors"
	"followingfeed/pkg/logger"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Result struct {
	Code       int    `json:"code"`
	Msg        string `json:"msg"`
	Data       any    `json:"data"`
	HTTPStatus int    `json:"-"`
}

func writeResult(ctx *gin.Context, res Result, err error) {
	status := res.HTTPStatus
	if status == 0 {
		switch {
		case err != nil || res.Code == 5:
			status = http.StatusInternalServerError
		case res.Code == 4:
			status = http.StatusBadRequest
		default:
			status = http.StatusOK
		}
	}
	ctx.JSON(status, res)
}

func bindRequest[T any](ctx *gin.Context, req *T) bool {
	if err := ctx.ShouldBind(req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			ctx.JSON(http.StatusRequestEntityTooLarge, Result{Code: 4, Msg: "请求体过大"})
			return false
		}
		ctx.JSON(http.StatusBadRequest, Result{Code: 4, Msg: "请求参数错误"})
		return false
	}
	return true
}

// Wrapper 包装“无请求体、无需登录 claims”的业务 Handler。
// 它统一记录业务错误，并根据业务结果输出对应的 HTTP 状态码。
func Wrapper(_ logger.LoggerV1, fn func(ctx *gin.Context) (Result, error)) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		res, err := fn(ctx)
		if err != nil {
			_ = ctx.Error(err)
		}
		writeResult(ctx, res, err)
	}
}

// WrapperReq 包装“有请求体、无需登录 claims”的业务 Handler。
// 它将请求绑定到 T，统一记录业务错误，并输出对应的 HTTP 状态码。
func WrapperReq[T any](
	_ logger.LoggerV1,
	fn func(ctx *gin.Context, req T) (Result, error),
) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req T
		if !bindRequest(ctx, &req) {
			return
		}

		//执行业务逻辑
		res, err := fn(ctx, req)
		if err != nil {
			_ = ctx.Error(err)
		}
		writeResult(ctx, res, err)
	}
}

// WrapperReqWithToken 包装“有请求体、需要登录 claims”的业务 Handler。
// 它将请求绑定到 T，从 Gin Context 提取 Cls；claims 缺失或类型错误时返回 HTTP 401，
// 其余业务结果使用对应的 HTTP 状态码输出，业务错误由包装器记录。
func WrapperReqWithToken[T any, Cls jwt.Claims](
	_ logger.LoggerV1,
	fn func(ctx *gin.Context, reqBody T, cls Cls) (Result, error),
) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var reqBody T
		if !bindRequest(ctx, &reqBody) {
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
			_ = ctx.Error(err)
		}

		writeResult(ctx, res, err)
	}
}

// WrapperWithToken 包装“无请求体、需要登录 claims”的业务 Handler。
// 它从 Gin Context 提取 Cls；claims 缺失或类型错误时返回 HTTP 401，
// 其余业务结果使用对应的 HTTP 状态码输出，业务错误由包装器记录。
func WrapperWithToken[Cls jwt.Claims](
	_ logger.LoggerV1,
	fn func(ctx *gin.Context, cls Cls) (Result, error),
) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		val, ok := ctx.Get("claims")
		if !ok {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		claims, ok := val.(Cls)
		if !ok {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		res, err := fn(ctx, claims)
		if err != nil {
			_ = ctx.Error(err)
		}

		writeResult(ctx, res, err)
	}
}
