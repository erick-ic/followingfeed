package service

import (
	"context"
	"errors"
	"fmt"
	"followingfeed/internal/repository/cache"
	"followingfeed/pkg/logger"
	"strconv"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

type PublicProfile = cache.PublicProfile

// PublicProfileService 聚合用户资料和跨领域统计。
type PublicProfileService interface {
	Get(ctx context.Context, uid int64) (PublicProfile, error)
}

type publicProfileService struct {
	userSvc    UserService
	followSvc  FollowService
	articleSvc ArticleService
	cache      cache.PublicProfileCache
	l          logger.LoggerV1
	loadGroup  singleflight.Group
}

func (s *publicProfileService) Get(ctx context.Context, uid int64) (PublicProfile, error) {
	cacheCtx, cancel := context.WithTimeout(ctx, cache.ReadTimeout)
	profile, cacheErr := s.cache.Get(cacheCtx, uid)
	cancel()
	if cacheErr == nil {
		return profile, nil
	}
	if errors.Is(cacheErr, cache.ErrCachedNotFound) {
		return PublicProfile{}, ErrUserNotFound
	}
	if !errors.Is(cacheErr, cache.ErrNotExists) {
		logger.FromContext(ctx, s.l).Warn("读取公开资料缓存失败，回源数据库", logger.Int64("user_id", uid), logger.Error(cacheErr))
	}

	loaded, err, _ := s.loadGroup.Do(strconv.FormatInt(uid, 10), func() (any, error) {
		return s.load(ctx, uid)
	})
	if err != nil {
		return PublicProfile{}, err
	}
	return loaded.(PublicProfile), nil
}

func (s *publicProfileService) load(ctx context.Context, uid int64) (PublicProfile, error) {
	user, err := s.userSvc.Profile(ctx, uid)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			s.cacheNotFound(ctx, uid)
		}
		return PublicProfile{}, fmt.Errorf("查询用户资料失败：%w", err)
	}

	result := PublicProfile{User: user}
	group, groupCtx := errgroup.WithContext(ctx)

	group.Go(func() error {
		count, err := s.followSvc.CountFollowing(groupCtx, uid)
		if err != nil {
			return fmt.Errorf("统计关注数量失败：%w", err)
		}
		result.FollowingCount = count
		return nil
	})
	group.Go(func() error {
		count, err := s.followSvc.CountFollowers(groupCtx, uid)
		if err != nil {
			return fmt.Errorf("统计粉丝数量失败：%w", err)
		}
		result.FollowersCount = count
		return nil
	})
	group.Go(func() error {
		count, err := s.articleSvc.CountPublishedByAuthor(groupCtx, uid)
		if err != nil {
			return fmt.Errorf("统计已发布文章数量失败：%w", err)
		}
		result.ArticleCount = count
		return nil
	})

	if err = group.Wait(); err != nil {
		return PublicProfile{}, err
	}
	s.cacheProfile(ctx, result)
	return result, nil
}

func (s *publicProfileService) cacheProfile(ctx context.Context, profile PublicProfile) {
	cacheCtx, cancel := context.WithTimeout(ctx, cache.WriteTimeout)
	defer cancel()
	if err := s.cache.Set(cacheCtx, profile); err != nil {
		logger.FromContext(ctx, s.l).Warn("回写公开资料缓存失败", logger.Int64("user_id", profile.User.Id), logger.Error(err))
	}
}

func (s *publicProfileService) cacheNotFound(ctx context.Context, uid int64) {
	cacheCtx, cancel := context.WithTimeout(ctx, cache.WriteTimeout)
	defer cancel()
	if err := s.cache.SetNotFound(cacheCtx, uid); err != nil {
		logger.FromContext(ctx, s.l).Warn("回写公开资料空值缓存失败", logger.Int64("user_id", uid), logger.Error(err))
	}
}

func NewPublicProfileService(
	userSvc UserService,
	followSvc FollowService,
	articleSvc ArticleService,
	profileCache cache.PublicProfileCache,
	l logger.LoggerV1,
) PublicProfileService {
	return &publicProfileService{
		userSvc:    userSvc,
		followSvc:  followSvc,
		articleSvc: articleSvc,
		cache:      profileCache,
		l:          l,
	}
}
