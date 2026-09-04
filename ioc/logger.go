package ioc

import (
	"followingfeed/config"
	"followingfeed/pkg/logger"

	"go.uber.org/zap"
)

// InitLogger 根据运行环境创建 Zap 日志器，并附加所有日志共享的服务与部署环境字段。
// 日志器是应用启动的基础依赖，初始化失败时直接终止启动，避免服务在无日志状态下运行。
func InitLogger(cfg config.Config) logger.LoggerV1 {
	var (
		l   *zap.Logger
		err error
	)
	if cfg.Env == "production" {
		// 生产模式输出适合日志平台采集的结构化 JSON，并使用更保守的默认日志级别。
		l, err = zap.NewProduction()
	} else {
		// 开发模式优先保证本地可读性，并保留调试级日志。
		l, err = zap.NewDevelopment()
	}
	if err != nil {
		panic(err)
	}
	return logger.NewZapLogger(l).With(
		logger.String("service.name", "followingfeed"),
		logger.String("deployment.environment.name", cfg.Env),
	)
}
