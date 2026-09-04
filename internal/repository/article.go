package repository

import (
	"context"
	"errors"
	"followingfeed/internal/domain"
	"followingfeed/internal/repository/cache"
	"followingfeed/internal/repository/dao"
	"followingfeed/pkg/logger"
	"sync"

	"github.com/ecodeclub/ekit/slice"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var ErrArticleNotFound = errors.New("文章不存在")

const (
	articleFirstPageSize = 10
	// articleCacheLockCount 是缓存锁的固定分片数。分片越多，不同用户发生锁碰撞的概率越低，
	// 但会占用更多常驻内存；当前 64 个分片适合本项目的并发规模。
	articleCacheLockCount = 64
)

// ArticleRepository 在领域模型与 DAO 模型间转换，并管理公开列表缓存。
type ArticleRepository interface {
	Create(ctx context.Context, article domain.Article) (int64, error)
	Update(ctx context.Context, article domain.Article) error
	Sync(ctx context.Context, article domain.Article) (int64, error)
	SyncStatus(ctx context.Context, id int64, uid int64, status domain.ArticleStatus) (int64, error)
	SoftDelete(ctx context.Context, id int64, uid int64) (int64, error)
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
		authorID int64,
		offset int,
		limit int,
	) ([]domain.PublishArticle, error)
	CountPublishedByAuthor(ctx context.Context, authorID int64) (int64, error)
}

type ArticleRepositoryImpl struct {
	dao   dao.ArticleDAO
	cache cache.ArticleCache
	l     logger.LoggerV1
	// cacheLocks 按用户 ID 分片。相同用户的首页缓存回源、回写和失效串行执行，
	// 避免旧查询结果在写操作删除缓存后重新写入；不同分片的用户可以并发执行。
	// 固定分片不会像按用户动态保存 Mutex 那样随用户数量持续占用内存。
	cacheLocks [articleCacheLockCount]sync.Mutex
	// publishedCacheLock 保护公开首页缓存回源以及影响公开列表的写操作。
	publishedCacheLock sync.Mutex
}

func (ar *ArticleRepositoryImpl) CountPublishedByAuthor(
	ctx context.Context,
	authorID int64,
) (int64, error) {
	return ar.dao.CountPublishedByAuthor(ctx, authorID)
}

func (ar *ArticleRepositoryImpl) PubListByAuthor(
	ctx context.Context,
	authorID int64,
	offset int,
	limit int,
) ([]domain.PublishArticle, error) {
	res, err := ar.dao.GetPublishedByAuthor(ctx, authorID, offset, limit)
	if err != nil {
		return nil, err
	}
	return slice.Map[dao.PublishArticle, domain.PublishArticle](
		res,
		func(_ int, src dao.PublishArticle) domain.PublishArticle {
			return ar.toPublishDomain(src)
		}), nil
}

func (ar *ArticleRepositoryImpl) CountFeed(ctx context.Context, uid int64) (int64, error) {
	return ar.dao.CountFeed(ctx, uid)
}

func (ar *ArticleRepositoryImpl) Feed(
	ctx context.Context,
	uid int64,
	offset int,
	limit int,
) ([]domain.PublishArticle, error) {
	res, err := ar.dao.GetFeed(ctx, uid, offset, limit)
	if err != nil {
		return nil, err
	}
	return slice.Map[dao.PublishArticle, domain.PublishArticle](
		res,
		func(_ int, src dao.PublishArticle) domain.PublishArticle {
			return ar.toPublishDomain(src)
		}), nil
}

func (ar *ArticleRepositoryImpl) GetByPubId(ctx context.Context, id int64) (domain.Article, error) {
	res, err := ar.dao.GetByPubId(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Article{}, ErrArticleNotFound
	}
	if err != nil {
		return domain.Article{}, err
	}
	data := domain.Article(ar.toPublishDomain(res))
	return data, nil
}

func (ar *ArticleRepositoryImpl) CountPub(ctx context.Context) (int64, error) {
	if ar.cache == nil {
		return ar.dao.CountPublished(ctx)
	}
	count, err := ar.cache.GetPublishedCount(ctx)
	if err == nil {
		return count, nil
	}
	if !errors.Is(err, redis.Nil) {
		logger.FromContext(ctx, ar.l).Warn("读取已发布文章总数缓存失败，回源数据库", logger.Error(err))
	}

	ar.publishedCacheLock.Lock()
	defer ar.publishedCacheLock.Unlock()

	count, err = ar.cache.GetPublishedCount(ctx)
	if err == nil {
		return count, nil
	}
	count, err = ar.dao.CountPublished(ctx)
	if err != nil {
		return 0, err
	}
	cacheCtx, cancel := context.WithTimeout(context.Background(), cache.ArticleOperationTimeout)
	defer cancel()
	if err = ar.cache.SetPublishedCount(cacheCtx, count); err != nil {
		logger.FromContext(ctx, ar.l).Warn("回写已发布文章总数缓存失败", logger.Error(err))
	}
	return count, nil
}

func (ar *ArticleRepositoryImpl) PubList(
	ctx context.Context,
	offset int,
	limit int,
) ([]domain.PublishArticle, error) {
	if offset == 0 && limit == articleFirstPageSize && ar.cache != nil {
		return ar.publishedFirstPage(ctx, offset, limit)
	}
	return ar.pubListFromDB(ctx, offset, limit)
}

func (ar *ArticleRepositoryImpl) publishedFirstPage(
	ctx context.Context,
	offset int,
	limit int,
) ([]domain.PublishArticle, error) {
	articles, err := ar.cache.GetPublishedFirstPage(ctx)
	if err == nil {
		return articles, nil
	}
	if !errors.Is(err, redis.Nil) {
		logger.FromContext(ctx, ar.l).Warn("读取公开文章首页缓存失败，回源数据库", logger.Error(err))
	}

	// 未命中后加锁并二次检查，避免同一用户的大量请求同时回源数据库。
	ar.publishedCacheLock.Lock()
	defer ar.publishedCacheLock.Unlock()

	articles, err = ar.cache.GetPublishedFirstPage(ctx)
	if err == nil {
		return articles, nil
	}
	articles, err = ar.pubListFromDB(ctx, offset, limit)
	if err != nil {
		return nil, err
	}
	cacheCtx, cancel := context.WithTimeout(context.Background(), cache.ArticleOperationTimeout)
	defer cancel()
	if err = ar.cache.SetPublishedFirstPage(cacheCtx, articles); err != nil {
		logger.FromContext(ctx, ar.l).Warn("回写公开文章首页缓存失败", logger.Error(err))
	}
	return articles, nil
}

func (ar *ArticleRepositoryImpl) pubListFromDB(
	ctx context.Context,
	offset int,
	limit int,
) ([]domain.PublishArticle, error) {
	res, err := ar.dao.GetPublished(ctx, offset, limit)
	if err != nil {
		return nil, err
	}
	return slice.Map[dao.PublishArticle, domain.PublishArticle](
		res,
		func(_ int, src dao.PublishArticle) domain.PublishArticle {
			return ar.toPublishDomain(src)
		}), nil
}

func (ar *ArticleRepositoryImpl) GetById(
	ctx context.Context,
	id int64,
	uid int64,
) (domain.Article, error) {
	res, err := ar.dao.GetById(ctx, id, uid)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Article{}, ErrArticleNotFound
	}
	if err != nil {
		return domain.Article{}, err
	}
	data := ar.toDomain(res)
	return data, nil
}

// CountByStatus 将 DAO 的分组行转换为以领域状态为键的统计。
func (ar *ArticleRepositoryImpl) CountByStatus(
	ctx context.Context,
	uid int64,
) (map[domain.ArticleStatus]int64, error) {
	rows, err := ar.dao.CountByAuthorStatus(ctx, uid)
	if err != nil {
		return nil, err
	}
	counts := make(map[domain.ArticleStatus]int64, len(rows))
	for _, row := range rows {
		counts[domain.ArticleStatus(row.Status)] = row.Count
	}
	return counts, nil
}

func (ar *ArticleRepositoryImpl) Count(ctx context.Context, uid int64) (int64, error) {
	return ar.dao.CountByAuthor(ctx, uid)
}

func (ar *ArticleRepositoryImpl) List(
	ctx context.Context,
	uid int64,
	offset int,
	limit int,
) ([]domain.Article, error) {
	if offset == 0 && limit == articleFirstPageSize && ar.cache != nil {
		return ar.authorFirstPage(ctx, uid, offset, limit)
	}
	return ar.listFromDB(ctx, uid, offset, limit)
}

func (ar *ArticleRepositoryImpl) authorFirstPage(
	ctx context.Context,
	uid int64,
	offset int,
	limit int,
) ([]domain.Article, error) {
	// 缓存命中是常见路径，无需获取进程内锁。
	articles, err := ar.cache.GetFirstPage(ctx, uid)
	if err == nil {
		return articles, nil
	}
	if !errors.Is(err, redis.Nil) {
		logger.FromContext(ctx, ar.l).Warn("读取文章首页缓存失败，回源数据库", logger.Error(err))
	}

	// 未命中后加锁并二次检查，避免同一用户的大量请求同时回源数据库。
	cacheLock := ar.cacheLock(uid)
	cacheLock.Lock()
	defer cacheLock.Unlock()

	articles, err = ar.cache.GetFirstPage(ctx, uid)
	if err == nil {
		return articles, nil
	}

	articles, err = ar.listFromDB(ctx, uid, offset, limit)
	if err != nil {
		return nil, err
	}

	cacheCtx, cancel := context.WithTimeout(context.Background(), cache.ArticleOperationTimeout)
	defer cancel()
	if err = ar.cache.SetFirstPage(cacheCtx, uid, articles); err != nil {
		logger.FromContext(ctx, ar.l).Warn("回写文章首页缓存失败", logger.Error(err))
	}
	return articles, nil
}

func (ar *ArticleRepositoryImpl) listFromDB(
	ctx context.Context,
	uid int64,
	offset int,
	limit int,
) ([]domain.Article, error) {
	res, err := ar.dao.GetByAuthor(ctx, uid, offset, limit)
	if err != nil {
		return nil, err
	}
	return slice.Map[dao.Article, domain.Article](
		res,
		func(_ int, src dao.Article) domain.Article {
			return ar.toDomain(src)
		}), nil
}

func (ar *ArticleRepositoryImpl) SoftDelete(
	ctx context.Context,
	id int64,
	uid int64,
) (int64, error) {
	cacheLock := ar.cacheLock(uid)
	cacheLock.Lock()
	defer cacheLock.Unlock()
	ar.publishedCacheLock.Lock()
	defer ar.publishedCacheLock.Unlock()

	res, err := ar.dao.SoftDelete(ctx, id, uid)
	if err == nil {
		ar.invalidateFirstPage(ctx, uid)
		ar.invalidatePublished(ctx)
	}
	return res, err
}

// SyncStatus 以下写方法在同一临界区内完成“写数据库 → 删除首页缓存”。
// 这样同一用户的首页缓存
// 回源不能插入到两步之间，保证数据库写成功后不会遗留或重新写入旧缓存。
func (ar *ArticleRepositoryImpl) SyncStatus(
	ctx context.Context,
	id int64,
	uid int64,
	status domain.ArticleStatus,
) (int64, error) {
	cacheLock := ar.cacheLock(uid)
	cacheLock.Lock()
	defer cacheLock.Unlock()
	ar.publishedCacheLock.Lock()
	defer ar.publishedCacheLock.Unlock()

	res, err := ar.dao.SyncStatus(ctx, id, uid, status)
	if err == nil {
		ar.invalidateFirstPage(ctx, uid)
		ar.invalidatePublished(ctx)
	}
	return res, err
}

func (ar *ArticleRepositoryImpl) Sync(ctx context.Context, article domain.Article) (int64, error) {
	cacheLock := ar.cacheLock(article.Author.Id)
	cacheLock.Lock()
	defer cacheLock.Unlock()
	ar.publishedCacheLock.Lock()
	defer ar.publishedCacheLock.Unlock()

	id, err := ar.dao.Sync(ctx, ar.toEntity(article))
	if err == nil {
		ar.invalidateFirstPage(ctx, article.Author.Id)
		ar.invalidatePublished(ctx)
	}
	return id, err
}

func (ar *ArticleRepositoryImpl) Update(ctx context.Context, article domain.Article) error {
	cacheLock := ar.cacheLock(article.Author.Id)
	cacheLock.Lock()
	defer cacheLock.Unlock()

	err := ar.dao.UpdateByArticleId(ctx, dao.Article{
		Id:       article.Id,
		Title:    article.Title,
		Content:  article.Content,
		AuthorId: article.Author.Id,
		Status:   article.Status.ToUint8(),
	})
	if err == nil {
		ar.invalidateFirstPage(ctx, article.Author.Id)
	}
	return err
}

func (ar *ArticleRepositoryImpl) Create(
	ctx context.Context,
	article domain.Article,
) (int64, error) {
	// 按作者锁住“创建文章 → 删除首页缓存”的完整过程，防止同一作者的并发查询
	// 在缓存删除后又将创建前的旧文章列表回写到缓存。
	cacheLock := ar.cacheLock(article.Author.Id)
	cacheLock.Lock()
	defer cacheLock.Unlock()

	id, err := ar.dao.Insert(ctx, dao.Article{
		Title:    article.Title,
		Content:  article.Content,
		AuthorId: article.Author.Id,
		Status:   article.Status.ToUint8(),
	})
	if err == nil {
		ar.invalidateFirstPage(ctx, article.Author.Id)
	}
	return id, err
}

func (ar *ArticleRepositoryImpl) invalidateFirstPage(ctx context.Context, uid int64) {
	if ar.cache == nil {
		return
	}
	cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cache.ArticleOperationTimeout)
	defer cancel()
	if err := ar.cache.DelFirstPage(cacheCtx, uid); err != nil {
		logger.FromContext(ctx, ar.l).Warn("删除文章首页缓存失败", logger.Error(err))
	}
}

func (ar *ArticleRepositoryImpl) invalidatePublished(ctx context.Context) {
	if ar.cache == nil {
		return
	}
	cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cache.ArticleOperationTimeout)
	defer cancel()
	if err := ar.cache.DelPublished(cacheCtx); err != nil {
		logger.FromContext(ctx, ar.l).Warn("删除公开文章缓存失败", logger.Error(err))
	}
}

// cacheLock 返回用户所属分片的锁。相同 uid 始终映射到同一把锁；
// 不同 uid 只有在分片碰撞时才会互相等待，以较小的并发损耗换取固定的内存占用。
// 该锁只在当前服务进程内有效，不能协调多个部署实例。
func (ar *ArticleRepositoryImpl) cacheLock(uid int64) *sync.Mutex {
	return &ar.cacheLocks[uint64(uid)%articleCacheLockCount]
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

// toPublishDomain 将线上库实体直接转换为公开文章领域模型。
func (ar *ArticleRepositoryImpl) toPublishDomain(article dao.PublishArticle) domain.PublishArticle {
	return domain.PublishArticle{
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
