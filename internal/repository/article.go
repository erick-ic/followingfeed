package repository

import (
	"context"
	"followingfeed/internal/domain"
	"followingfeed/internal/repository/cache"
	"followingfeed/internal/repository/dao"
	"followingfeed/pkg/logger"
	"sync"
	"time"

	"github.com/ecodeclub/ekit/slice"
	"github.com/redis/go-redis/v9"
)

const articleFirstPageSize = 10

type ArticleRepository interface {
	Create(ctx context.Context, article domain.Article) (int64, error)
	Update(ctx context.Context, article domain.Article) error
	Sync(ctx context.Context, article domain.Article) (int64, error)
	SyncStatus(ctx context.Context, id int64, uid int64, status domain.ArticleStatus) (int64, error)
	SoftDelete(ctx context.Context, id int64, uid int64) (int64, error)
	List(ctx context.Context, uid int64, offset int, limit int) ([]domain.Article, error)
	GetById(ctx context.Context, id int64) (domain.Article, error)
	PubList(ctx context.Context, offset int, limit int) ([]domain.PublishArticle, error)
	GetByPubId(ctx context.Context, id int64) (domain.Article, error)
}

type ArticleRepositoryImpl struct {
	dao   dao.ArticleDAO
	cache cache.ArticleCache
	l     logger.LoggerV1
	// 首页缓存的回源、回写和失效必须串行，避免失效后旧查询结果重新写入缓存。
	cacheMu sync.Mutex
}

func (ar *ArticleRepositoryImpl) GetByPubId(ctx context.Context, id int64) (domain.Article, error) {
	res, err := ar.dao.GetByPubId(ctx, id)
	if err != nil {
		return domain.Article{}, err
	}
	data := ar.toDomain(dao.Article(res))
	return data, nil
}

func (ar *ArticleRepositoryImpl) PubList(ctx context.Context, offset int, limit int) ([]domain.PublishArticle, error) {
	res, err := ar.dao.GetPublished(ctx, offset, limit)
	if err != nil {
		return nil, err
	}
	return slice.Map[dao.PublishArticle, domain.PublishArticle](res, func(_ int, src dao.PublishArticle) domain.PublishArticle {
		return domain.PublishArticle(ar.toDomain(dao.Article(src)))
	}), nil
}

func (ar *ArticleRepositoryImpl) GetById(ctx context.Context, id int64) (domain.Article, error) {
	res, err := ar.dao.GetById(ctx, id)
	if err != nil {
		return domain.Article{}, err
	}
	data := ar.toDomain(res)
	return data, nil
}

func (ar *ArticleRepositoryImpl) List(ctx context.Context, uid int64, offset int, limit int) ([]domain.Article, error) {
	if offset != 0 || limit != articleFirstPageSize {
		return ar.listFromDB(ctx, uid, offset, limit)
	}

	ar.cacheMu.Lock()
	defer ar.cacheMu.Unlock()

	articles, err := ar.cache.GetFirstPage(ctx, uid)
	if err == nil {
		return articles, nil
	}
	if err != redis.Nil {
		ar.l.Warn("读取文章首页缓存失败，回源数据库", logger.Error(err))
	}

	articles, err = ar.listFromDB(ctx, uid, offset, limit)
	if err != nil {
		return nil, err
	}

	cacheCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err = ar.cache.SetFirstPage(cacheCtx, uid, articles); err != nil {
		ar.l.Error("回写文章首页缓存失败", logger.Error(err))
	}
	return articles, nil
}

func (ar *ArticleRepositoryImpl) listFromDB(ctx context.Context, uid int64, offset int, limit int) ([]domain.Article, error) {
	res, err := ar.dao.GetByAuthor(ctx, uid, offset, limit)
	if err != nil {
		return nil, err
	}
	return slice.Map[dao.Article, domain.Article](res, func(_ int, src dao.Article) domain.Article {
		return ar.toDomain(src)
	}), nil
}

func (ar *ArticleRepositoryImpl) SyncStatus(ctx context.Context, id int64, uid int64, status domain.ArticleStatus) (int64, error) {
	ar.cacheMu.Lock()
	defer ar.cacheMu.Unlock()

	res, err := ar.dao.SyncStatus(ctx, id, uid, status)
	if err == nil {
		ar.invalidateFirstPage(uid)
	}
	return res, err
}

func (ar *ArticleRepositoryImpl) SoftDelete(ctx context.Context, id int64, uid int64) (int64, error) {
	ar.cacheMu.Lock()
	defer ar.cacheMu.Unlock()

	res, err := ar.dao.SoftDelete(ctx, id, uid)
	if err == nil {
		ar.invalidateFirstPage(uid)
	}
	return res, err
}

func (ar *ArticleRepositoryImpl) Sync(ctx context.Context, article domain.Article) (int64, error) {
	ar.cacheMu.Lock()
	defer ar.cacheMu.Unlock()

	id, err := ar.dao.Sync(ctx, ar.toEntity(article))
	if err == nil {
		ar.invalidateFirstPage(article.Author.Id)
	}
	return id, err
}

func (ar *ArticleRepositoryImpl) Create(ctx context.Context, article domain.Article) (int64, error) {
	ar.cacheMu.Lock()
	defer ar.cacheMu.Unlock()

	id, err := ar.dao.Insert(ctx, dao.Article{
		Title:    article.Title,
		Content:  article.Content,
		AuthorId: article.Author.Id,
		Status:   article.Status.ToUint8(),
	})
	if err == nil {
		ar.invalidateFirstPage(article.Author.Id)
	}
	return id, err
}

func (ar *ArticleRepositoryImpl) Update(ctx context.Context, article domain.Article) error {
	ar.cacheMu.Lock()
	defer ar.cacheMu.Unlock()

	err := ar.dao.UpdateByArticleId(ctx, dao.Article{
		Id:       article.Id,
		Title:    article.Title,
		Content:  article.Content,
		AuthorId: article.Author.Id,
		Status:   article.Status.ToUint8(),
	})
	if err == nil {
		ar.invalidateFirstPage(article.Author.Id)
	}
	return err
}

func (ar *ArticleRepositoryImpl) invalidateFirstPage(uid int64) {
	cacheCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := ar.cache.DelFirstPage(cacheCtx, uid); err != nil {
		ar.l.Error("删除文章首页缓存失败", logger.Error(err))
	}
}

func NewArticleRepositoryImpl(
	dao dao.ArticleDAO,
	cache cache.ArticleCache,
	l logger.LoggerV1,
) ArticleRepository {
	return &ArticleRepositoryImpl{
		dao:   dao,
		cache: cache,
		l:     l,
	}
}

// toDomain 将DAO实体转换为领域模型
func (ar *ArticleRepositoryImpl) toDomain(article dao.Article) domain.Article {
	return domain.Article{
		Id:      article.Id,
		Title:   article.Title,
		Content: article.Content,
		Status:  domain.ArticleStatus(article.Status),
		Author: domain.Author{
			Id:       article.AuthorId,
			Nickname: article.AuthorNickname,
		},
		CreatedAt: article.CreatedAt,
		UpdatedAt: article.UpdatedAt,
		DeletedAt: article.DeletedAt,
	}
}

// toEntity 将领域模型转换为DAO实体
func (ar *ArticleRepositoryImpl) toEntity(article domain.Article) dao.Article {
	return dao.Article{
		Id:       article.Id,
		Title:    article.Title,
		Content:  article.Content,
		AuthorId: article.Author.Id,
		Status:   article.Status.ToUint8(),
	}
}
