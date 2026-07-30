package logger

//参数需要命名约束
// 业务使用的日志接口，所有日志方法接收结构化字段（Field 切片）。

type LoggerV1 interface {
	Debug(msg string, args ...Field)
	Info(msg string, args ...Field)
	Warn(msg string, args ...Field)
	Error(msg string, args ...Field)
	With(args ...Field) LoggerV1
}

type Field struct {
	Key   string
	Value any
}
