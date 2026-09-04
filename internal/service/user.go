package service

import (
	"context"
	"errors"
	"followingfeed/internal/domain"
	"followingfeed/internal/repository"
	"followingfeed/pkg/logger"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrUserDuplicated      = repository.ErrUserDuplicated
	ErrUserNotFound        = repository.ErrUserNotFound
	ErrInvalidUserPassword = errors.New("账号/邮箱或密码不对")
	ErrInvalidNickname     = errors.New("昵称格式不正确")
)

type UserService interface {
	Create(ctx context.Context, u domain.User) error
	Login(ctx context.Context, email string, password string) (domain.User, error)
	Profile(ctx context.Context, uid int64) (domain.User, error)
}

type UserServiceImpl struct {
	repo repository.UserRepository
}

func (us *UserServiceImpl) Profile(ctx context.Context, uid int64) (domain.User, error) {
	u, err := us.repo.FindById(ctx, uid)
	if err != nil {
		return domain.User{}, err
	}
	return u, nil
}

func (us *UserServiceImpl) Login(
	ctx context.Context,
	email string,
	password string,
) (domain.User, error) {
	//1. 根据邮箱从数据库查询用户信息（含加密密码）
	u, err := us.repo.FindByEmail(ctx, email)
	//未找到用户
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.User{}, ErrInvalidUserPassword
	}
	if err != nil {
		return domain.User{}, err
	}
	// 2. 比较明文密码与数据库中存储的哈希
	err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	if err != nil {
		return domain.User{}, ErrInvalidUserPassword
	}
	return u, nil
}

func (us *UserServiceImpl) Create(ctx context.Context, u domain.User) error {
	u.Nickname = strings.TrimSpace(u.Nickname)
	if !domain.IsValidNickname(u.Nickname) {
		return ErrInvalidNickname
	}

	//1. 生成加密哈希
	hash, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// 2. 用哈希值替换明文密码
	u.Password = string(hash)

	// 3. 存储到数据库
	return us.repo.Create(ctx, u)
}

func NewUserServiceImpl(repo repository.UserRepository, _ logger.LoggerV1) UserService {
	return &UserServiceImpl{
		repo: repo,
	}
}
