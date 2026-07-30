package logger

import "go.uber.org/zap"

type ZapLogger struct {
	logger *zap.Logger
}

func NewZapLogger(l *zap.Logger) *ZapLogger {
	return &ZapLogger{
		logger: l,
	}
}

func (z *ZapLogger) Debug(msg string, fields ...Field) {
	z.logger.Debug(msg, z.toZapFields(fields)...)
}

func (z *ZapLogger) Info(msg string, fields ...Field) {
	z.logger.Info(msg, z.toZapFields(fields)...)
}

func (z *ZapLogger) Warn(msg string, fields ...Field) {
	z.logger.Warn(msg, z.toZapFields(fields)...)
}

func (z *ZapLogger) Error(msg string, fields ...Field) {
	z.logger.Error(msg, z.toZapFields(fields)...)
}

// With 创建一个携带预设字段的子 logger，后续所有日志自动附加这些字段。
func (z *ZapLogger) With(args ...Field) LoggerV1 {
	return &ZapLogger{
		logger: z.logger.With(z.toZapFields(args)...),
	}
}

// 缺陷：参数转换存在[]zap.Field
func (z *ZapLogger) toZapFields(fields []Field) []zap.Field {
	res := make([]zap.Field, 0, len(fields))
	for _, field := range fields {
		res = append(res, zap.Any(field.Key, field.Value))
	}
	return res
}
