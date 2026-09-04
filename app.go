package main

import (
	"followingfeed/internal/observability"
	"followingfeed/pkg/logger"

	"github.com/gin-gonic/gin"
)

type App struct {
	Server  *gin.Engine
	Metrics *observability.Metrics
	Logger  logger.LoggerV1
}
