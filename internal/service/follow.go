package service

import (
	"context"
	"errors"
	"followingfeed/internal/domain"
	"followingfeed/internal/repository"
	"followingfeed/internal/repository/cache"
	"followingfeed/pkg/logger"
)

var (
	ErrCannotFollowSelf   = errors.New("不能关注自己")
	ErrFollowDuplicated   = repository.ErrFollowDuplicated
	ErrTargetUserNotFound = errors.New("目标用户不存在")
)

type FollowService interface {
	Follow(ctx context.Context, uid int64, targetId int64) error
	UnFollow(ctx context.Context, uid int64, targetId int64) error
	IsFollowing(ctx context.Context, uid int64, targetId int64) (bool, error)
	ListFollowingPage(ctx context.Context, uid int64, offset, limit int) ([]domain.Follow, error)
	CountFollowing(ctx context.Context, uid int64) (int64, error)
	ListFollowersPage(ctx context.Context, uid int64, offset, limit int) ([]domain.Follow, error)
	CountFollowers(ctx context.Context, uid int64) (int64, error)
}

type FollowServiceImpl struct {
	repo        repository.FollowRepository
	userSvc     UserService
	invalidator cache.UserProfileCacheInvalidator
	l           logger.LoggerV1
}

func (f *FollowServiceImpl) CountFollowers(ctx context.Context, uid int64) (int64, error) {
	return f.repo.CountFollowers(ctx, uid)
}

func (f *FollowServiceImpl) ListFollowersPage(
	ctx context.Context,
	uid int64,
	offset, limit int,
) ([]domain.Follow, error) {
	return f.repo.ListFollowersPage(ctx, uid, offset, limit)
}

func (f *FollowServiceImpl) CountFollowing(ctx context.Context, uid int64) (int64, error) {
	return f.repo.CountFollowing(ctx, uid)
}

func (f *FollowServiceImpl) ListFollowingPage(
	ctx context.Context,
	uid int64,
	offset, limit int,
) ([]domain.Follow, error) {
	return f.repo.ListFollowingPage(ctx, uid, offset, limit)
}

func (f *FollowServiceImpl) IsFollowing(
	ctx context.Context,
	uid int64,
	targetId int64,
) (bool, error) {
	return f.repo.IsFollowing(ctx, uid, targetId)
}

func (f *FollowServiceImpl) UnFollow(ctx context.Context, uid int64, targetId int64) error {
	if err := f.repo.UnFollow(ctx, uid, targetId); err != nil {
		return err
	}
	f.invalidatePublicProfiles(ctx, uid, targetId)
	return nil
}

func (f *FollowServiceImpl) Follow(ctx context.Context, uid int64, targetId int64) error {
	if uid == targetId {
		return ErrCannotFollowSelf
	}
	if _, err := f.userSvc.Profile(ctx, targetId); errors.Is(err, ErrUserNotFound) {
		return ErrTargetUserNotFound
	} else if err != nil {
		return err
	}
	if err := f.repo.Follow(ctx, uid, targetId); err != nil {
		return err
	}
	f.invalidatePublicProfiles(ctx, uid, targetId)
	return nil
}

func (f *FollowServiceImpl) invalidatePublicProfiles(ctx context.Context, uids ...int64) {
	cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cache.WriteTimeout)
	defer cancel()
	if err := f.invalidator.DelPublicProfiles(cacheCtx, uids...); err != nil {
		logger.FromContext(ctx, f.l).Warn("删除公开资料缓存失败", logger.Error(err))
	}
}

func NewFollowService(
	repo repository.FollowRepository,
	userSvc UserService,
	invalidator cache.UserProfileCacheInvalidator,
	l logger.LoggerV1,
) FollowService {
	return &FollowServiceImpl{repo: repo, userSvc: userSvc, invalidator: invalidator, l: l}
}
