package repository

import (
	"context"
	"database/sql"
	"followingfeed/internal/domain"
	"followingfeed/internal/repository/cache"
	"followingfeed/internal/repository/dao"
	"time"
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
}

func (ur *UserRepositoryImpl) FindById(ctx context.Context, uid int64) (domain.User, error) {
	//1.先从cache里找
	u, cacheErr := ur.cache.Get(ctx, uid)
	if cacheErr == nil {
		return u, nil // 缓存命中
	}

	//2.缓存未命中（redis.Nil）或缓存故障（其他错误）
	//redis没有这个数据，去数据库找
	ue, err := ur.dao.FindById(ctx, uid)
	if err != nil {
		return domain.User{}, err
	}

	//u = domain.User{
	//	Id:       int64(ue.Id),
	//	Email:    ue.Email,
	//	Password: ue.Password,
	//}
	u = ur.entityToDomain(ue)

	go func(user domain.User) {
		cacheCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = ur.cache.Set(cacheCtx, user)
	}(u)

	// 如果缓存故障（如连接超时），也可以不回写，或者同步尝试，但不要阻塞

	return u, err
}

func (ur *UserRepositoryImpl) FindByEmail(ctx context.Context, email string) (domain.User, error) {
	u, err := ur.dao.FindByEmail(ctx, email)
	if err != nil {
		return domain.User{}, err
	}
	//return domain.User{
	//	Id:       int64(u.Id),
	//	Email:    u.Email,
	//	Password: u.Password,
	//}, nil
	return ur.entityToDomain(u), nil
}

func (ur *UserRepositoryImpl) Create(ctx context.Context, u domain.User) error {
	return ur.dao.Insert(ctx, u)
}

func NewUserRepositoryImpl(dao dao.UserDAO, cache cache.UserCache) UserRepository {
	return &UserRepositoryImpl{
		dao:   dao,
		cache: cache,
	}
}

// DAO → Domain
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

// Domain → DAO
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
