package logger

import "context"

type contextFieldsKey struct{}

// ContextWithFields 在处理器、服务、仓储和异步缓存清理任务之间传递有限的日志字段，
// 避免这些层直接依赖 Gin。
func ContextWithFields(ctx context.Context, fields ...Field) context.Context {
	existing, _ := ctx.Value(contextFieldsKey{}).([]Field)
	combined := make([]Field, 0, len(existing)+len(fields))
	combined = append(combined, existing...)
	combined = append(combined, fields...)
	return context.WithValue(ctx, contextFieldsKey{}, combined)
}

func FromContext(ctx context.Context, fallback LoggerV1) LoggerV1 {
	if ctx == nil {
		return fallback
	}
	fields, _ := ctx.Value(contextFieldsKey{}).([]Field)
	if len(fields) == 0 {
		return fallback
	}
	return fallback.With(fields...)
}
