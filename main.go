package main

import (
	"context"
	"fmt"
	"followingfeed/config"
	"followingfeed/pkg/logger"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// 将退出码处理留在最外层，使 run 返回前仍能执行全部 defer 清理逻辑。
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run 负责应用从配置加载、依赖初始化、服务启动到优雅退出的完整生命周期。
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("加载应用配置失败：%w", err)
	}

	app, cleanup, err := InitApp(cfg)
	if err != nil {
		return fmt.Errorf("初始化应用失败：%w", err)
	}
	// Wire 返回的 cleanup 统一释放数据库连接池等基础设施资源。
	defer cleanup()
	// 尽量刷出缓冲日志；部分平台或标准输出实现不支持 Sync，因此忽略关闭阶段错误。
	defer func() {
		if syncer, ok := app.Logger.(interface{ Sync() error }); ok {
			_ = syncer.Sync()
		}
	}()
	server := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           app.Server,
		ReadTimeout:       cfg.Server.ReadTimeout,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}
	// 指标使用独立监听地址，避免业务路由、鉴权和限流影响监控系统抓取。
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", app.Metrics.Handler())
	metricsServer := &http.Server{
		Addr:              cfg.Observability.MetricsAddr,
		Handler:           metricsMux,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	// 缓冲区覆盖两个监听协程，确保其中一个触发退出后，另一个不会因上报错误而阻塞。
	serverErrors := make(chan error, 2)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- fmt.Errorf("启动 HTTP 服务失败：%w", err)
		}
	}()
	go func() {
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- fmt.Errorf("启动监控指标服务失败：%w", err)
		}
	}()
	app.Logger.Info(
		"HTTP 服务已启动",
		logger.String("event.name", "application.started"),
		logger.String("server.address", cfg.Server.Addr),
		logger.String("metrics.address", cfg.Observability.MetricsAddr),
	)

	// SIGINT 用于本地中断，SIGTERM 用于容器编排平台发起的滚动停止。
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	var runErr error
	select {
	case sig := <-signalCh:
		app.Logger.Info(
			"收到服务关闭信号",
			logger.String("event.name", "application.shutdown_started"),
			logger.String("signal", sig.String()),
		)
	case err := <-serverErrors:
		runErr = err
		app.Logger.Error(
			"HTTP 服务异常停止",
			logger.String("event.name", "application.server_failed"),
			logger.Error(err),
		)
	}
	// 两个服务共享同一个截止时间，使整个关闭过程受统一的最大时长约束。
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		app.Logger.Error("关闭 HTTP 服务失败", logger.Error(err))
		if runErr == nil {
			runErr = fmt.Errorf("关闭 HTTP 服务失败：%w", err)
		}
	}
	if err := metricsServer.Shutdown(ctx); err != nil {
		app.Logger.Error("关闭监控指标服务失败", logger.Error(err))
		if runErr == nil {
			runErr = fmt.Errorf("关闭监控指标服务失败：%w", err)
		}
	}
	return runErr
}
