package ioc

import (
	"followingfeed/config"
	"followingfeed/pkg/logger"

	"go.uber.org/zap"
)

func InitLogger(cfg config.Config) logger.LoggerV1 {
	var (
		l   *zap.Logger
		err error
	)
	if cfg.Env == "production" {
		l, err = zap.NewProduction()
	} else {
		l, err = zap.NewDevelopment()
	}
	if err != nil {
		panic(err)
	}
	return logger.NewZapLogger(l)
}
