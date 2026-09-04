package service

import (
	"context"
	"followingfeed/internal/domain"
	"followingfeed/internal/repository"
	"followingfeed/internal/repository/cache"
	"followingfeed/pkg/logger"
)

var ErrArticleNotFound = repository.ErrArticleNotFound

// ArticleService 负责文章状态流转和业务查询，不暴露数据库模型。
type ArticleService interface {
	Save(ctx context.Context, article domain.Article) (int64, error)
	Publish(ctx context.Context, article domain.Article) (int64, error)
	Withdraw(ctx context.Context, id int64, uid int64) (int64, error)
	Delete(ctx context.Context, id int64, uid int64) (int64, error)
	List(ctx context.Context, uid int64, offset int, limit int) ([]domain.Article, error)
	Count(ctx context.Context, uid int64) (int64, error)
	CountByStatus(ctx context.Context, uid int64) (map[domain.ArticleStatus]int64, error)
	GetById(ctx context.Context, id int64, uid int64) (domain.Article, error)
	PubList(ctx context.Context, offset int, limit int) ([]domain.PublishArticle, error)
	CountPub(ctx context.Context) (int64, error)
	GetByPubId(ctx context.Context, id int64) (domain.Article, error)
	Feed(ctx context.Context, uid int64, offset int, limit int) ([]domain.PublishArticle, error)
	CountFeed(ctx context.Context, uid int64) (int64, error)
	PubListByAuthor(
		ctx context.Context,
		authorId int64,
		offset int,
		limit int,
	) ([]domain.PublishArticle, error)
	CountPublishedByAuthor(ctx context.Context, authorId int64) (int64, error)
}

type ArticleServiceImpl struct {
	repo        repository.ArticleRepository
	invalidator cache.UserProfileCacheInvalidator
	l           logger.LoggerV1
}

func (as *ArticleServiceImpl) CountPublishedByAuthor(ctx context.Context, authorId int64) (int64, error) {
	return as.repo.CountPublishedByAuthor(ctx, authorId)
}

func (as *ArticleServiceImpl) PubListByAuthor(
	ctx context.Context,
	authorId int64,
	offset int,
	limit int,
) ([]domain.PublishArticle, error) {
	return as.repo.PubListByAuthor(ctx, authorId, offset, limit)
}

func (as *ArticleServiceImpl) CountFeed(ctx context.Context, uid int64) (int64, error) {
	return as.repo.CountFeed(ctx, uid)
}

func (as *ArticleServiceImpl) Feed(
	ctx context.Context,
	uid int64,
	offset int,
	limit int,
) ([]domain.PublishArticle, error) {
	return as.repo.Feed(ctx, uid, offset, limit)
}

func (as *ArticleServiceImpl) GetByPubId(ctx context.Context, id int64) (domain.Article, error) {
	return as.repo.GetByPubId(ctx, id)
}

func (as *ArticleServiceImpl) CountPub(ctx context.Context) (int64, error) {
	return as.repo.CountPub(ctx)
}

func (as *ArticleServiceImpl) PubList(
	ctx context.Context,
	offset int,
	limit int,
) ([]domain.PublishArticle, error) {
	return as.repo.PubList(ctx, offset, limit)
}

func (as *ArticleServiceImpl) GetById(
	ctx context.Context,
	id int64,
	uid int64,
) (domain.Article, error) {
	return as.repo.GetById(ctx, id, uid)
}

// CountByStatus 返回作者全部未删除文章的状态统计，不受列表分页影响。
func (as *ArticleServiceImpl) CountByStatus(
	ctx context.Context,
	uid int64,
) (map[domain.ArticleStatus]int64, error) {
	return as.repo.CountByStatus(ctx, uid)
}

func (as *ArticleServiceImpl) Count(ctx context.Context, uid int64) (int64, error) {
	return as.repo.Count(ctx, uid)
}

func (as *ArticleServiceImpl) List(
	ctx context.Context,
	uid int64,
	offset int,
	limit int,
) ([]domain.Article, error) {
	return as.repo.List(ctx, uid, offset, limit)
}

func (as *ArticleServiceImpl) Delete(ctx context.Context, id int64, uid int64) (int64, error) {
	res, err := as.repo.SoftDelete(ctx, id, uid)
	if err == nil {
		as.invalidateProfileCaches(ctx, uid)
	}
	return res, err
}

func (as *ArticleServiceImpl) Withdraw(ctx context.Context, id int64, uid int64) (int64, error) {
	status := domain.ArticleStatusUnPublished
	res, err := as.repo.SyncStatus(ctx, id, uid, status)
	if err == nil {
		as.invalidateProfileCaches(ctx, uid)
	}
	return res, err
}

func (as *ArticleServiceImpl) Publish(ctx context.Context, article domain.Article) (int64, error) {
	article.Status = domain.ArticleStatusPublished
	id, err := as.repo.Sync(ctx, article)
	if err == nil {
		as.invalidateProfileCaches(ctx, article.Author.Id)
	}
	return id, err
}

func (as *ArticleServiceImpl) Save(ctx context.Context, article domain.Article) (int64, error) {
	article.Status = domain.ArticleStatusUnPublished
	if article.Id > 0 {
		err := as.repo.Update(ctx, article)
		return article.Id, err
	}
	id, err := as.repo.Create(ctx, article)
	return id, err
}

func (as *ArticleServiceImpl) invalidateProfileCaches(ctx context.Context, uid int64) {
	cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cache.WriteTimeout)
	defer cancel()
	if err := as.invalidator.DelPublicProfiles(cacheCtx, uid); err != nil {
		logger.FromContext(ctx, as.l).Warn("删除公开资料缓存失败", logger.Int64("user_id", uid), logger.Error(err))
	}
	if err := as.invalidator.DelArticleStats(cacheCtx, uid); err != nil {
		logger.FromContext(ctx, as.l).Warn("删除用户文章统计缓存失败", logger.Int64("user_id", uid), logger.Error(err))
	}
}

func NewArticleServiceImpl(
	repo repository.ArticleRepository,
	invalidator cache.UserProfileCacheInvalidator,
	l logger.LoggerV1,
) ArticleService {
	return &ArticleServiceImpl{
		repo: repo, invalidator: invalidator, l: l,
	}
}
