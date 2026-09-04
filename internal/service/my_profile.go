package service

import (
	"context"
	"errors"
	"fmt"
	"followingfeed/internal/domain"
	"followingfeed/internal/repository/cache"
	"followingfeed/pkg/logger"
	"strconv"

	"golang.org/x/sync/singleflight"
)

// MyProfile 是当前登录用户的私有资料和文章互动汇总。
type MyProfile struct {
	User                domain.User
	ArticleLikeCount    int64
	ArticleReadCount    int64
	ArticleCollectCount int64
}

type MyProfileService interface {
	Get(ctx context.Context, uid int64) (MyProfile, error)
}

type myProfileService struct {
	userSvc        UserService
	interactiveSvc InteractiveService
	statsCache     cache.UserArticleStatsCache
	l              logger.LoggerV1
	loadGroup      singleflight.Group
}

func (s *myProfileService) Get(ctx context.Context, uid int64) (MyProfile, error) {
	user, err := s.userSvc.Profile(ctx, uid)
	if err != nil {
		return MyProfile{}, fmt.Errorf("查询当前用户资料失败：%w", err)
	}

	result := MyProfile{User: user}
	stats, err := s.getArticleStats(ctx, uid)
	if err != nil {
		return MyProfile{}, err
	}
	result.ArticleLikeCount = stats.LikeCount
	result.ArticleReadCount = stats.ReadCount
	result.ArticleCollectCount = stats.CollectCount
	return result, nil
}

func (s *myProfileService) getArticleStats(
	ctx context.Context,
	uid int64,
) (cache.ArticleStats, error) {
	cacheCtx, cancel := context.WithTimeout(ctx, cache.ReadTimeout)
	stats, cacheErr := s.statsCache.Get(cacheCtx, uid)
	cancel()
	if cacheErr == nil {
		return stats, nil
	}
	if !errors.Is(cacheErr, cache.ErrNotExists) {
		logger.FromContext(ctx, s.l).Warn("读取用户文章统计缓存失败，回源数据库", logger.Int64("user_id", uid), logger.Error(cacheErr))
	}

	loaded, err, _ := s.loadGroup.Do(strconv.FormatInt(uid, 10), func() (any, error) {
		interaction, err := s.interactiveSvc.GetArticleStatsByAuthor(ctx, articleBiz, uid)
		if err != nil {
			return cache.ArticleStats{}, fmt.Errorf("查询文章互动统计失败：%w", err)
		}
		stats := cache.ArticleStats{
			LikeCount:    interaction.LikeCnt,
			ReadCount:    interaction.ReadCnt,
			CollectCount: interaction.CollectCnt,
		}
		writeCtx, cancel := context.WithTimeout(ctx, cache.WriteTimeout)
		defer cancel()
		if err = s.statsCache.Set(writeCtx, uid, stats); err != nil {
			logger.FromContext(ctx, s.l).Warn("回写用户文章统计缓存失败", logger.Int64("user_id", uid), logger.Error(err))
		}
		return stats, nil
	})
	if err != nil {
		return cache.ArticleStats{}, err
	}
	return loaded.(cache.ArticleStats), nil
}

func NewMyProfileService(
	userSvc UserService,
	interactiveSvc InteractiveService,
	statsCache cache.UserArticleStatsCache,
	l logger.LoggerV1,
) MyProfileService {
	return &myProfileService{
		userSvc:        userSvc,
		interactiveSvc: interactiveSvc,
		statsCache:     statsCache,
		l:              l,
	}
}
