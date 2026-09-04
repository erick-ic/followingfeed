package observability

import (
	"errors"
	"fmt"
	"followingfeed/internal/handler/jwt"
	"followingfeed/pkg/logger"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	requestIDKey  = "request_id"
	panicKey      = "observability_panic"
	panicStackKey = "observability_panic_stack"
)

func RequestID() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		requestID := ctx.GetHeader("X-Request-ID")
		if !validRequestID(requestID) {
			requestID = uuid.NewString()
		}
		ctx.Set(requestIDKey, requestID)
		ctx.Header("X-Request-ID", requestID)
		ctx.Request = ctx.Request.WithContext(logger.ContextWithFields(
			ctx.Request.Context(),
			logger.String("request_id", requestID),
		))
		ctx.Next()
	}
}

func validRequestID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	return strings.IndexFunc(value, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.')
	}) == -1
}

func RequestIDFromContext(ctx *gin.Context) string {
	return ctx.GetString(requestIDKey)
}

// HTTPObserver 在内部中间件和处理器执行完成后，仅记录一条访问日志，
// 并更新低基数的 HTTP 指标。
func (m *Metrics) HTTPObserver(l logger.LoggerV1) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()
		m.httpInFlight.Inc()
		defer m.httpInFlight.Dec()

		ctx.Next()

		duration := time.Since(start)
		route := ctx.FullPath()
		if route == "" {
			route = "unknown"
		}
		status := ctx.Writer.Status()
		statusText := strconv.Itoa(status)
		m.httpRequests.WithLabelValues(ctx.Request.Method, route, statusText).Inc()
		m.httpDuration.WithLabelValues(ctx.Request.Method, route, statusText).Observe(duration.Seconds())

		if isSuccessfulProbe(ctx.Request.URL.Path, status) {
			return
		}

		fields := []logger.Field{
			logger.String("event.name", "http.server.request"),
			logger.String("request_id", RequestIDFromContext(ctx)),
			logger.String("http.request.method", ctx.Request.Method),
			logger.String("http.route", route),
			logger.Int64("http.response.status_code", int64(status)),
			logger.Int64("server.request.duration_ms", duration.Milliseconds()),
			logger.String("client.address", ctx.ClientIP()),
		}
		if eventName := ctx.GetString("security_event"); eventName != "" {
			fields[0] = logger.String("event.name", eventName)
		}
		if claimsValue, ok := ctx.Get("claims"); ok {
			if claims, ok := claimsValue.(*jwt.UserClaims); ok {
				fields = append(fields, logger.Int64("user_id", claims.Uid))
			}
		} else if uid, ok := ctx.Get("auth_user_id"); ok {
			if userID, ok := uid.(int64); ok {
				fields = append(fields, logger.Int64("user_id", userID))
			}
		}
		if reason := ctx.GetString("auth_failure_reason"); reason != "" {
			fields = append(fields, logger.String("auth.reason", reason))
		}
		if last := ctx.Errors.Last(); last != nil && status >= http.StatusInternalServerError {
			fields = append(fields, logger.Error(last.Err))
		}
		if panicValue, ok := ctx.Get(panicKey); ok {
			fields = append(fields,
				logger.String("error.type", "panic"),
				logger.String("panic", fmt.Sprint(panicValue)),
			)
			if stack, ok := ctx.Get(panicStackKey); ok {
				fields = append(fields, logger.String("stack", stack.(string)))
			}
		}

		switch {
		case status >= http.StatusInternalServerError || ctx.GetString("observability_level") == "error":
			l.Error("HTTP 请求处理完成", fields...)
		case status == http.StatusTooManyRequests:
			l.Warn("HTTP 请求处理完成", fields...)
		default:
			l.Info("HTTP 请求处理完成", fields...)
		}
	}
}

func isSuccessfulProbe(path string, status int) bool {
	return status < http.StatusBadRequest && (path == "/health/live" || path == "/health/ready")
}

// Recovery 将 panic 转换为 HTTP 500。外层 HTTPObserver 统一记录错误日志，
// 使 panic 堆栈与访问字段保持关联。
func (m *Metrics) Recovery() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if panicValue := recover(); panicValue != nil {
				m.panics.Inc()
				ctx.Set(panicKey, panicValue)
				ctx.Set(panicStackKey, string(debug.Stack()))
				_ = ctx.Error(errors.New("已捕获程序异常"))
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"code": 5,
					"msg":  "系统错误",
				})
			}
		}()
		ctx.Next()
	}
}
