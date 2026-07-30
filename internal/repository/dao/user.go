package dao

import (
	"context"
	"database/sql"
	"errors"
	"followingfeed/internal/domain"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

var (
	ErrUserDuplicated = errors.New("邮箱冲突！")
	ErrUserNotFound   = gorm.ErrRecordNotFound
)

type UserDAO interface {
	Insert(ctx context.Context, u domain.User) error
	FindByEmail(ctx context.Context, email string) (User, error)
	FindById(ctx context.Context, uid int64) (User, error)
}

type GORMUserDAO struct {
	db *gorm.DB
}

func (ud *GORMUserDAO) FindById(ctx context.Context, uid int64) (User, error) {
	var u User
	err := ud.db.WithContext(ctx).Where("`id` = ?", uid).First(&u).Error
	return u, err
}

func (ud *GORMUserDAO) FindByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := ud.db.WithContext(ctx).Where("email = ?", email).First(&u).Error
	return u, err
}

func (ud *GORMUserDAO) Insert(ctx context.Context, u domain.User) error {
	now := time.Now().UnixMilli()
	u.CreatedAt = now
	u.UpdatedAt = now
	err := ud.db.WithContext(ctx).Create(&u).Error

	// 捕获 MySQL 1062 错误（唯一键冲突）
	if mysqlErr, ok := err.(*mysql.MySQLError); ok {
		const uniqueConflictsErrNo uint16 = 1062
		if mysqlErr.Number == uniqueConflictsErrNo {
			//唯一索引冲突，即邮箱/手机号冲突
			return ErrUserDuplicated
		}
	}
	return err
}

func NewGORMUserDAO(db *gorm.DB) UserDAO {
	return &GORMUserDAO{
		db: db,
	}
}

// User 数据库表结构
// 别称entity、model、PO(persistent object)
type User struct {
	Id       int            `gorm:"primaryKey, autoIncrement"`
	Nickname string         `gorm:"type:varchar(24);not null;default:技术旅人"`
	Email    sql.NullString `gorm:"unique"`
	Password string
	//创建时间，毫秒数
	CreatedAt int64
	//更新时间，毫秒数
	UpdatedAt int64
}
