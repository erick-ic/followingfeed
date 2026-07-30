//go:build wireinject

package main

import (
	"followingfeed/config"
	"followingfeed/internal/handler/article"
	ijwt "followingfeed/internal/handler/jwt"
	"followingfeed/internal/handler/user"
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
)

var userHandlerSet = wire.NewSet(
	cache.NewUserCache,
	dao.NewGORMUserDAO,
	repository.NewUserRepositoryImpl,
	service.NewUserServiceImpl,
	user.NewUserHandler,
)

var articleHandlerSet = wire.NewSet(
	dao.NewGORMArticleDAO,
	cache.NewRedisArticleCache,
	repository.NewArticleRepositoryImpl,
	service.NewArticleServiceImpl,
	article.NewArticleHandler,
)

func InitApp(cfg config.Config) (*App, error) {
	wire.Build(
		thirdServiceSet,
		ijwt.NewRedisJWTHandler,

		userHandlerSet,
		articleHandlerSet,

		//初始化路由
		ioc.InitGin,
		ioc.InitMiddlewares,
		ioc.InitLogger,

		wire.Struct(new(App), "*"),
	)

	return nil, nil
}
