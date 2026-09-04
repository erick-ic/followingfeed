package dao

//go:generate mockgen -source=follow.go -package=daomocks -destination=mocks/follow.mock.go

import (
	"context"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"

	"gorm.io/gorm"
)

var ErrFollowDuplicated = errors.New("关注关系已存在")

const followListSelect = "follows.id, follows.follower_id, follows.following_id, " +
	"follows.created_at, follows.updated_at, users.nickname AS nickname"

type FollowDAO interface {
	Insert(ctx context.Context, uid int64, targetId int64) error
	UnFollow(ctx context.Context, uid int64, targetId int64) error
	IsFollowing(ctx context.Context, uid int64, targetId int64) (bool, error)
	ListFollowingPage(ctx context.Context, uid int64, offset, limit int) ([]Follow, error)
	CountFollowing(ctx context.Context, uid int64) (int64, error)
	ListFollowersPage(ctx context.Context, uid int64, offset, limit int) ([]Follow, error)
	CountFollowers(ctx context.Context, uid int64) (int64, error)
}

type FollowDAOImpl struct {
	db *gorm.DB
}

func (fd *FollowDAOImpl) CountFollowers(ctx context.Context, uid int64) (int64, error) {
	var count int64
	err := fd.db.WithContext(ctx).Model(&Follow{}).
		Where("following_id = ?", uid).
		Count(&count).Error
	return count, err
}

func (fd *FollowDAOImpl) ListFollowersPage(
	ctx context.Context,
	uid int64,
	offset, limit int,
) ([]Follow, error) {
	var follows []Follow
	err := fd.db.WithContext(ctx).
		Select(followListSelect).
		Joins("LEFT JOIN users ON users.id = follows.follower_id").
		Where("follows.following_id = ?", uid).
		Order("follows.created_at DESC, follows.id DESC").
		Offset(offset).
		Limit(limit).
		Find(&follows).Error
	return follows, err
}

func (fd *FollowDAOImpl) CountFollowing(ctx context.Context, uid int64) (int64, error) {
	var count int64
	err := fd.db.WithContext(ctx).Model(&Follow{}).
		Where("follower_id = ?", uid).
		Count(&count).Error
	return count, err
}

func (fd *FollowDAOImpl) ListFollowingPage(
	ctx context.Context,
	uid int64,
	offset, limit int,
) ([]Follow, error) {
	var follows []Follow
	err := fd.db.WithContext(ctx).
		Select(followListSelect).
		Joins("LEFT JOIN users ON users.id = follows.following_id").
		Where("follows.follower_id = ?", uid).
		Order("follows.created_at DESC, follows.id DESC").
		Offset(offset).
		Limit(limit).
		Find(&follows).Error
	return follows, err
}

func (fd *FollowDAOImpl) IsFollowing(ctx context.Context, uid int64, targetId int64) (bool, error) {
	var count int64
	err := fd.db.WithContext(ctx).Model(&Follow{}).
		Where("follower_id = ? AND following_id = ?", uid, targetId).
		Count(&count).Error
	return count > 0, err
}

func (fd *FollowDAOImpl) UnFollow(ctx context.Context, uid int64, targetId int64) error {
	res := fd.db.WithContext(ctx).
		Where("follower_id = ? AND following_id = ?", uid, targetId).
		Delete(&Follow{})
	return res.Error
}

func (fd *FollowDAOImpl) Insert(ctx context.Context, uid int64, targetId int64) error {
	if uid == targetId {
		return gorm.ErrInvalidData
	}

	now := time.Now().UnixMilli()
	follow := Follow{
		FollowerId:  uid,
		FollowingId: targetId,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	err := fd.db.WithContext(ctx).Create(&follow).Error
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return ErrFollowDuplicated
	}
	return err
}

func NewFollow(db *gorm.DB) FollowDAO {
	return &FollowDAOImpl{
		db: db,
	}
}

type Follow struct {
	Id          int64 `gorm:"primaryKey;autoIncrement"`
	FollowerId  int64
	FollowingId int64
	Nickname    string `gorm:"column:nickname;->;-:migration"`
	CreatedAt   int64
	UpdatedAt   int64
}
