package service

import (
	"context"
	"followingfeed/internal/domain"
	"followingfeed/internal/repository"
)

type ArticleService interface {
	Save(ctx context.Context, article domain.Article) (int64, error)
	Publish(ctx context.Context, article domain.Article) (int64, error)
	Withdraw(ctx context.Context, id int64, uid int64) (int64, error)
	Delete(ctx context.Context, id int64, uid int64) (int64, error)
	List(ctx context.Context, uid int64, offset int, limit int) ([]domain.Article, error)
	GetById(ctx context.Context, id int64) (domain.Article, error)
	PubList(ctx context.Context, offset int, limit int) ([]domain.PublishArticle, error)
	GetByPubId(ctx context.Context, id int64) (domain.Article, error)
}

type ArticleServiceImpl struct {
	repo repository.ArticleRepository
}

func (as *ArticleServiceImpl) GetByPubId(ctx context.Context, id int64) (domain.Article, error) {
	return as.repo.GetByPubId(ctx, id)
}

func (as *ArticleServiceImpl) PubList(ctx context.Context, offset int, limit int) ([]domain.PublishArticle, error) {
	return as.repo.PubList(ctx, offset, limit)
}
func (as *ArticleServiceImpl) GetById(ctx context.Context, id int64) (domain.Article, error) {
	return as.repo.GetById(ctx, id)
}

func (as *ArticleServiceImpl) List(ctx context.Context, uid int64, offset int, limit int) ([]domain.Article, error) {
	return as.repo.List(ctx, uid, offset, limit)
}

func (as *ArticleServiceImpl) Withdraw(ctx context.Context, id int64, uid int64) (int64, error) {
	status := domain.ArticleStatusUnPublished
	return as.repo.SyncStatus(ctx, id, uid, status)
}

func (as *ArticleServiceImpl) Delete(ctx context.Context, id int64, uid int64) (int64, error) {
	return as.repo.SoftDelete(ctx, id, uid)
}

func (as *ArticleServiceImpl) Publish(ctx context.Context, article domain.Article) (int64, error) {
	article.Status = domain.ArticleStatusPublished
	return as.repo.Sync(ctx, article)
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

func NewArticleServiceImpl(repo repository.ArticleRepository) ArticleService {
	return &ArticleServiceImpl{
		repo: repo,
	}
}
