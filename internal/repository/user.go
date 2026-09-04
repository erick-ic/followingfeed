package repository

import (
	"context"
	"database/sql"
	"errors"
	"followingfeed/internal/domain"
	"followingfeed/internal/repository/cache"
	"followingfeed/internal/repository/dao"
	"followingfeed/pkg/logger"
	"strconv"

	"golang.org/x/sync/singleflight"
)

var (
	ErrUserDuplicated = dao.ErrUserDuplicated
	ErrUserNotFound   = dao.ErrUserNotFound
)

type UserRepository interface {
	Create(ctx context.Context, u domain.User) error
	FindByEmail(ctx context.Context, email string) (domain.User, error)
	FindById(ctx context.Context, uid int64) (domain.User, error)
}

type UserRepositoryImpl struct {
	dao   dao.UserDAO
	cache cache.UserCache
	l     logger.LoggerV1
	// loadGroup 合并当前进程内相同用户 ID 的并发回源请求。
	// 当缓存未命中时，同一个 key 只会执行一次数据库查询和缓存回写；
	// 其他请求等待并复用该结果，防止热点 Key 击穿数据库。
	// singleflight.Group 的零值可直接使用，不需要额外初始化；
	// 它不负责保存长期缓存，也不能跨服务实例合并请求。
	loadGroup singleflight.Group
}

func (ur *UserRepositoryImpl) FindById(ctx context.Context, uid int64) (domain.User, error) {
	//1.先从cache里找
	cacheCtx, cancel := context.WithTimeout(ctx, cache.ReadTimeout)
	u, cacheErr := ur.cache.Get(cacheCtx, uid)
	cancel()
	if cacheErr == nil {
		return u, nil // 缓存命中
	}
	if errors.Is(cacheErr, cache.ErrCachedNotFound) {
		return domain.User{}, ErrUserNotFound
	}
	if !errors.Is(cacheErr, cache.ErrNotExists) {
		logger.FromContext(ctx, ur.l).Warn("读取用户缓存失败，回源数据库",
			logger.Int64("user_id", uid),
			logger.Error(cacheErr),
		)
	}

	//2.缓存未命中（redis.Nil）或缓存故障（其他错误）
	//redis没有这个数据，去数据库找
	// uid 的字符串形式作为 singleflight key：相同 uid 合并执行，不同 uid 仍可并行查询。
	// Do 返回 value、error 和 shared；这里不需要区分结果是否被其他请求复用，因此忽略 shared。
	loaded, err, _ := ur.loadGroup.Do(strconv.FormatInt(uid, 10), func() (any, error) {
		// 只有同一 uid 的首个请求会执行该函数；并发等待者直接复用它返回的用户或错误。
		ue, err := ur.dao.FindById(ctx, uid)
		if err != nil {
			if errors.Is(err, ErrUserNotFound) {
				ur.cacheUserNotFound(ctx, uid)
			}
			return domain.User{}, err
		}

		user := ur.entityToDomain(ue)
		ur.cacheUser(ctx, user)
		return user, nil
	})
	if err != nil {
		return domain.User{}, err
	}
	// Do 的返回类型是 any，本组约定成功时始终返回 domain.User，因此在此恢复具体类型。
	return loaded.(domain.User), nil
}

func (ur *UserRepositoryImpl) cacheUser(ctx context.Context, user domain.User) {
	cacheCtx, cancel := context.WithTimeout(ctx, cache.WriteTimeout)
	defer cancel()
	if err := ur.cache.Set(cacheCtx, user); err != nil {
		logger.FromContext(ctx, ur.l).Warn("回写用户缓存失败", logger.Int64("user_id", user.Id), logger.Error(err))
	}
}

func (ur *UserRepositoryImpl) cacheUserNotFound(ctx context.Context, uid int64) {
	cacheCtx, cancel := context.WithTimeout(ctx, cache.WriteTimeout)
	defer cancel()
	if err := ur.cache.SetNotFound(cacheCtx, uid); err != nil {
		logger.FromContext(ctx, ur.l).Warn("回写用户空值缓存失败", logger.Int64("user_id", uid), logger.Error(err))
	}
}

func (ur *UserRepositoryImpl) FindByEmail(ctx context.Context, email string) (domain.User, error) {
	u, err := ur.dao.FindByEmail(ctx, email)
	if err != nil {
		return domain.User{}, err
	}
	return ur.entityToDomain(u), nil
}

func (ur *UserRepositoryImpl) Create(ctx context.Context, u domain.User) error {
	return ur.dao.Insert(ctx, u)
}

func NewUserRepositoryImpl(
	dao dao.UserDAO,
	cache cache.UserCache,
	l logger.LoggerV1,
) UserRepository {
	return &UserRepositoryImpl{
		dao:   dao,
		cache: cache,
		l:     l,
	}
}

// 数据访问模型转换为领域模型。
func (ur *UserRepositoryImpl) entityToDomain(u dao.User) domain.User {
	return domain.User{
		Id:        int64(u.Id),
		Nickname:  u.Nickname,
		Email:     u.Email.String,
		Password:  u.Password,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

// 领域模型转换为数据访问模型。
func (ur *UserRepositoryImpl) domainToEntity(u domain.User) dao.User {
	return dao.User{
		Id:        int(u.Id),
		Nickname:  u.Nickname,
		Email:     sql.NullString{String: u.Email, Valid: u.Email != ""},
		Password:  u.Password,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}
