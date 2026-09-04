//go:build wireinject

package main

import (
	"followingfeed/config"
	"followingfeed/internal/handler/article"
	"followingfeed/internal/handler/follow"
	"followingfeed/internal/handler/interactive"
	ijwt "followingfeed/internal/handler/jwt"
	"followingfeed/internal/handler/user"
	"followingfeed/internal/observability"
	"followingfeed/internal/repository"
	"followingfeed/internal/repository/cache"
	"followingfeed/internal/repository/dao"
	"followingfeed/internal/service"
	"followingfeed/ioc"

	"github.com/google/wire"
)

var thirdServiceSet = wire.NewSet(
	ioc.InitMySQL,
	ioc.InitRedis,
	cache.NewPublicProfileCache,
	cache.NewUserArticleStatsCache,
	cache.NewUserProfileCacheInvalidator,
)

var userHandlerSet = wire.NewSet(
	cache.NewUserCache,
	dao.NewGORMUserDAO,
	repository.NewUserRepositoryImpl,
	service.NewUserServiceImpl,
	service.NewPublicProfileService,
	service.NewMyProfileService,
	user.NewUserHandler,
)

var articleHandlerSet = wire.NewSet(
	dao.NewGORMArticleDAO,
	cache.NewRedisArticleCache,
	repository.NewArticleRepositoryImpl,
	service.NewArticleServiceImpl,
	article.NewArticleHandler,
)

var followHandlerSet = wire.NewSet(
	dao.NewFollow,
	repository.NewFollowRepository,
	service.NewFollowService,
	follow.NewHandler,
)
var interactiveHandlerSet = wire.NewSet(
	dao.NewInteractiveDAO,
	repository.NewInteractiveRepository,
	service.NewInteractiveService,
	interactive.NewHandler,
)

func InitApp(cfg config.Config) (*App, func(), error) {
	wire.Build(
		thirdServiceSet,
		ijwt.NewRedisJWTHandler,

		userHandlerSet,
		articleHandlerSet,
		followHandlerSet,
		interactiveHandlerSet,

		//初始化路由
		ioc.InitGin,
		ioc.InitMiddlewares,
		ioc.InitLogger,
		observability.NewMetrics,

		wire.Struct(new(App), "*"),
	)

	return nil, nil, nil
}
