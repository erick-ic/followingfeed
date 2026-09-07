package dao

import (
	"context"
	"fmt"
	"followingfeed/internal/domain"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// 列表只读取生成摘要所需的前 100 个 Unicode 字符，避免加载完整 BLOB 正文。
	articleListSelect = "articles.id, articles.title, " +
		"LEFT(CONVERT(articles.content USING utf8mb4), 100) AS content, " +
		"articles.author_id, articles.status, articles.created_at, " +
		"articles.updated_at, articles.deleted_at"
	articleDetailSelect = "articles.id, articles.title, articles.content, " +
		"articles.author_id, articles.status, articles.created_at, articles.updated_at"
	publishArticleListSelect = "publish_articles.id, publish_articles.title, " +
		"LEFT(CONVERT(publish_articles.content USING utf8mb4), 100) AS content, " +
		"publish_articles.author_id, publish_articles.status, publish_articles.created_at, " +
		"publish_articles.updated_at, publish_articles.deleted_at, " +
		"users.nickname AS author_nickname"
	publishArticleDetailSelect = "publish_articles.id, publish_articles.title, " +
		"publish_articles.content, publish_articles.author_id, publish_articles.status, " +
		"publish_articles.created_at, publish_articles.updated_at, " +
		"users.nickname AS author_nickname"
)

// ArticleDAO 只负责持久化；uid/authorID 条件用于限制数据归属。
type ArticleDAO interface {
	Insert(ctx context.Context, article Article) (int64, error)
	UpdateByArticleId(ctx context.Context, article Article) error
	Sync(ctx context.Context, article Article) (int64, error)
	Upsert(ctx context.Context, article PublishArticle) error
	SyncStatus(
		ctx context.Context,
		articleId int64,
		uid int64,
		status domain.ArticleStatus,
	) (int64, error)
	SoftDelete(ctx context.Context, articleId int64, uid int64) (int64, error)
	GetByAuthor(ctx context.Context, uid int64, offset int, limit int) ([]Article, error)
	CountByAuthor(ctx context.Context, uid int64) (int64, error)
	CountByAuthorStatus(ctx context.Context, uid int64) ([]ArticleStatusCount, error)
	GetPublished(ctx context.Context, offset int, limit int) ([]PublishArticle, error)
	CountPublished(ctx context.Context) (int64, error)
	GetById(ctx context.Context, id int64, uid int64) (Article, error)
	GetByPubId(ctx context.Context, id int64) (PublishArticle, error)
	GetFeed(ctx context.Context, uid int64, offset int, limit int) ([]PublishArticle, error)
	CountFeed(ctx context.Context, uid int64) (int64, error)
	GetPublishedByAuthor(
		ctx context.Context,
		authorID int64,
		offset int,
		limit int,
	) ([]PublishArticle, error)
	CountPublishedByAuthor(ctx context.Context, authorID int64) (int64, error)
}

type ArticleDAOImpl struct {
	db *gorm.DB
}

// ArticleStatusCount 对应 SELECT status, COUNT(*) ... GROUP BY status 的结果行。
type ArticleStatusCount struct {
	Status uint8 `gorm:"column:status"`
	Count  int64 `gorm:"column:count"`
}

// CountPublishedByAuthor 统计线上库中指定作者尚未删除的已发布文章数量。
func (ad *ArticleDAOImpl) CountPublishedByAuthor(
	ctx context.Context,
	authorID int64,
) (int64, error) {
	var count int64
	err := ad.db.WithContext(ctx).Model(&PublishArticle{}).
		Where(
			"author_id = ? AND status = ? AND deleted_at = 0",
			authorID, domain.ArticleStatusPublished.ToUint8(),
		).
		Count(&count).Error
	return count, err
}

// GetPublishedByAuthor 分页查询线上库中指定作者尚未删除的已发布文章。
func (ad *ArticleDAOImpl) GetPublishedByAuthor(
	ctx context.Context,
	authorID int64,
	offset int,
	limit int,
) ([]PublishArticle, error) {
	var articles []PublishArticle
	err := ad.db.WithContext(ctx).Model(&PublishArticle{}).
		Select(publishArticleListSelect).
		Joins("LEFT JOIN users ON users.id = publish_articles.author_id").
		Where(
			"publish_articles.author_id = ? AND publish_articles.status = ? AND publish_articles.deleted_at = 0",
			authorID,
			domain.ArticleStatusPublished.ToUint8(),
		).
		Offset(offset).
		Limit(limit).
		Order("publish_articles.updated_at DESC, publish_articles.id DESC").
		Find(&articles).Error
	return articles, err
}

// CountFeed 统计当前用户所关注作者发布的文章数量。
func (ad *ArticleDAOImpl) CountFeed(ctx context.Context, uid int64) (int64, error) {
	var count int64
	err := ad.db.WithContext(ctx).Model(&PublishArticle{}).
		Joins("JOIN follows ON follows.following_id = publish_articles.author_id AND follows.follower_id = ?", uid).
		Where(
			"publish_articles.status = ? AND publish_articles.deleted_at = 0",
			domain.ArticleStatusPublished.ToUint8()).
		Count(&count).Error
	return count, err
}

// GetFeed 分页查询当前用户所关注作者发布的文章。
func (ad *ArticleDAOImpl) GetFeed(
	ctx context.Context,
	uid int64,
	offset int,
	limit int,
) ([]PublishArticle, error) {
	var articles []PublishArticle
	// 先分页索引中的 ID 和时间，再读取正文，避免对全部候选文章回表后排序。
	selected := ad.db.WithContext(ctx).Model(&PublishArticle{}).
		Select("publish_articles.id, publish_articles.updated_at").
		Joins("JOIN follows ON follows.following_id = publish_articles.author_id AND follows.follower_id = ?", uid).
		Where("publish_articles.status = ? AND publish_articles.deleted_at = 0", domain.ArticleStatusPublished.ToUint8()).
		Order("publish_articles.updated_at DESC, publish_articles.id DESC").
		Offset(offset).
		Limit(limit)
	err := ad.db.WithContext(ctx).Table("(?) AS selected", selected).
		Select(publishArticleListSelect).
		Joins("JOIN publish_articles ON publish_articles.id = selected.id").
		Joins("LEFT JOIN users ON users.id = publish_articles.author_id").
		Order("selected.updated_at DESC, selected.id DESC").
		Find(&articles).Error
	return articles, err
}

// GetByPubId 按文章 ID 查询线上库中尚未删除的已发布文章及作者信息。
func (ad *ArticleDAOImpl) GetByPubId(ctx context.Context, id int64) (PublishArticle, error) {
	var pubArticle PublishArticle
	err := ad.db.WithContext(ctx).
		Select(publishArticleDetailSelect).
		Joins("LEFT JOIN users ON users.id = publish_articles.author_id").
		Where(
			"publish_articles.id = ? AND publish_articles.status = ? AND publish_articles.deleted_at = 0",
			id,
			domain.ArticleStatusPublished.ToUint8(),
		).
		First(&pubArticle).Error
	return pubArticle, err
}

// CountPublished 统计线上库中尚未删除的已发布文章数量。
func (ad *ArticleDAOImpl) CountPublished(ctx context.Context) (int64, error) {
	var count int64
	err := ad.db.WithContext(ctx).Model(&PublishArticle{}).
		Where("status = ? AND deleted_at = 0", domain.ArticleStatusPublished.ToUint8()).
		Count(&count).Error
	return count, err
}

// GetPublished 分页查询线上库中所有尚未删除的已发布文章。
// 线上库可能保留已撤回文章，因此必须显式过滤已发布状态。
func (ad *ArticleDAOImpl) GetPublished(
	ctx context.Context,
	offset int,
	limit int,
) ([]PublishArticle, error) {
	var pubArticles []PublishArticle
	err := ad.db.WithContext(ctx).Model(&PublishArticle{}).
		Select(publishArticleListSelect).
		Joins("LEFT JOIN users ON users.id = publish_articles.author_id").
		Where(
			"publish_articles.status = ? AND publish_articles.deleted_at = 0",
			domain.ArticleStatusPublished.ToUint8(),
		).
		Offset(offset).
		Limit(limit).
		Order("publish_articles.updated_at DESC, publish_articles.id DESC").
		Find(&pubArticles).Error
	return pubArticles, err
}

// GetById 按文章 ID 和作者 ID 查询制作库中尚未删除的文章。
func (ad *ArticleDAOImpl) GetById(ctx context.Context, id int64, uid int64) (Article, error) {
	var article Article
	err := ad.db.WithContext(ctx).
		Select(articleDetailSelect).
		Where("id = ? AND author_id = ? AND deleted_at = 0", id, uid).
		First(&article).Error
	return article, err
}

// CountByAuthorStatus 按状态聚合指定作者在制作库中尚未删除的文章。
func (ad *ArticleDAOImpl) CountByAuthorStatus(
	ctx context.Context,
	uid int64,
) ([]ArticleStatusCount, error) {
	var counts []ArticleStatusCount
	// 查询结果不是完整的 Article，而是每种状态及其文章数量。
	// COUNT(*) 使用别名 count，以便 GORM 将聚合值映射到 ArticleStatusCount.Count。
	err := ad.db.WithContext(ctx).Model(&Article{}).
		Select("status, COUNT(*) AS count").
		Where("author_id = ? AND deleted_at = 0", uid).
		Group("status").
		// Find 保持 Query 回调链，使统计查询也受查询超时约束。
		Find(&counts).Error
	return counts, err
}

// CountByAuthor 统计指定作者在制作库中尚未删除的文章数量。
func (ad *ArticleDAOImpl) CountByAuthor(ctx context.Context, uid int64) (int64, error) {
	var count int64
	err := ad.db.WithContext(ctx).Model(&Article{}).
		Where("author_id = ? AND deleted_at = 0", uid).
		Count(&count).Error
	return count, err
}

// GetByAuthor 分页查询指定作者在制作库中尚未删除的文章。
func (ad *ArticleDAOImpl) GetByAuthor(
	ctx context.Context,
	uid int64,
	offset int,
	limit int,
) ([]Article, error) {
	var articles []Article
	err := ad.db.WithContext(ctx).Model(&articles).
		Select(articleListSelect).
		Where("author_id = ? AND deleted_at = 0", uid).
		Offset(offset).
		Limit(limit).
		Order("updated_at DESC, id DESC").
		Find(&articles).Error
	return articles, err
}

// SoftDelete 校验作者身份后，在同一事务中软删除制作库和线上库的文章。
func (ad *ArticleDAOImpl) SoftDelete(
	ctx context.Context,
	articleId int64,
	uid int64,
) (int64, error) {
	now := time.Now().UnixMilli()
	return articleId, ad.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&Article{}).
			Where("id = ? AND author_id = ? AND deleted_at = 0", articleId, uid).
			Updates(map[string]any{
				"deleted_at": now,
				"updated_at": now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			// 已删除、不存在或无权限都不再改变数据，统一按幂等成功处理。
			return nil
		}

		// 仅制作库首次删除成功时同步线上库；线上记录不存在不视为错误。
		return tx.Model(&PublishArticle{}).
			Where("id = ? AND author_id = ? AND deleted_at = 0", articleId, uid).
			Updates(map[string]any{
				"deleted_at": now,
				"updated_at": now,
			}).Error
	})
}

// SyncStatus 校验作者身份后，在同一事务中同步制作库和线上库的文章状态。
func (ad *ArticleDAOImpl) SyncStatus(
	ctx context.Context,
	articleId int64,
	uid int64,
	status domain.ArticleStatus,
) (int64, error) {
	var (
		id = articleId
	)
	now := time.Now().UnixMilli()

	return id, ad.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 制作库：将文章设置为未发布；不限制原状态，使重复撤回保持幂等。
		res := tx.Model(&Article{}).
			Where("id = ? AND author_id = ? AND deleted_at = 0", id, uid).
			Updates(map[string]any{
				"status":     status,
				"updated_at": now,
			})
		if res.Error != nil {
			return res.Error
		}
		// 已经处于目标状态、文章不存在、已删除或作者不匹配时均不会修改数据，
		// 为保持幂等且避免泄漏资源归属，统一按成功处理。
		if res.RowsAffected == 0 {
			// 已撤回、不存在或无权限都不再改变数据，统一按幂等成功处理。
			return nil
		}
		// 制作库首次更新成功时，线上库必须存在同一作者的对应记录。
		res = tx.Model(&PublishArticle{}).
			Where("id = ? AND author_id = ? AND deleted_at = 0", id, uid).
			Updates(map[string]any{
				"status":     status,
				"updated_at": now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return fmt.Errorf("线上文章撤回失败，文章不存在或无权限，id %d，author_id %d", articleId, uid)
		}
		return nil
	})
}

// Sync 在同一事务中保存制作库文章，并将其内容同步到线上库。
func (ad *ArticleDAOImpl) Sync(ctx context.Context, article Article) (int64, error) {
	var (
		id  = article.Id
		err error
	)
	err = ad.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txDAO := NewGORMArticleDAO(tx)
		if id > 0 {
			err = txDAO.UpdateByArticleId(ctx, article)
		} else {
			id, err = txDAO.Insert(ctx, article)
			article.Id = id
		}
		if err != nil {
			return err
		}
		return txDAO.Upsert(ctx, PublishArticle(article))
	})
	return id, err
}

// Upsert 向线上库新增文章，或在主键冲突时更新已有文章，实现INSERT OR UPDATE语义
// 使用GORM的OnConflict子句，对应MySQL的INSERT ... ON DUPLICATE KEY UPDATE
// 若主键冲突则更新标题、内容、状态和更新时间，否则插入新记录
func (ad *ArticleDAOImpl) Upsert(ctx context.Context, article PublishArticle) error {
	now := time.Now().UnixMilli()
	article.CreatedAt = now
	article.UpdatedAt = now
	err := ad.db.WithContext(ctx).Clauses(clause.OnConflict{
		DoUpdates: clause.Assignments(map[string]any{
			"title":      article.Title,
			"content":    article.Content,
			"status":     article.Status,
			"updated_at": now,
		}),
	}).Create(&article).Error
	return err
}

// UpdateByArticleId 按文章 ID 和作者 ID 更新制作库中尚未删除的文章。
func (ad *ArticleDAOImpl) UpdateByArticleId(ctx context.Context, article Article) error {
	now := time.Now().UnixMilli()

	// 用空模型 + 显式 WHERE 条件，避免 GORM 把零值字段作为查询条件
	res := ad.db.WithContext(ctx).Model(&Article{}).
		Where("id = ? AND author_id = ? AND deleted_at = 0", article.Id, article.AuthorId).
		Updates(map[string]any{
			"title":      article.Title,
			"content":    article.Content,
			"status":     article.Status,
			"updated_at": now,
		})

	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		// MySQL 对写入相同值的 UPDATE 返回 0 changed rows。在并发发布中，
		// updated_at 也可能落在同一毫秒，因此需要区分“记录存在但无变化”和“未匹配”。
		var count int64
		err := ad.db.WithContext(ctx).Model(&Article{}).
			Where("id = ? AND author_id = ? AND deleted_at = 0", article.Id, article.AuthorId).
			Count(&count).Error
		if err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("更新失败，可能是作者非法，id %d，author_id %d", article.Id, article.AuthorId)
		}
	}
	return nil
}

// Insert 向制作库新增文章，并返回生成的文章 ID。
func (ad *ArticleDAOImpl) Insert(ctx context.Context, article Article) (int64, error) {
	now := time.Now().UnixMilli()
	article.CreatedAt = now
	article.UpdatedAt = now
	err := ad.db.WithContext(ctx).Create(&article).Error
	return article.Id, err
}

func NewGORMArticleDAO(db *gorm.DB) ArticleDAO {
	return &ArticleDAOImpl{
		db: db,
	}
}

type Article struct {
	Id      int64  `gorm:"primaryKey;autoIncrement"` // 主键ID
	Title   string // 文章标题
	Content string // 文章内容（大文本）

	AuthorId       int64  // 作者ID
	AuthorNickname string `gorm:"column:author_nickname;->;-:migration"` // 查询公开文章时关联读取
	Status         uint8
	CreatedAt      int64
	UpdatedAt      int64
	DeletedAt      int64
}

// PublishArticle 对应 publish_articles 线上表。物理索引由 SQL Migration 统一管理。
type PublishArticle struct {
	Id      int64 `gorm:"primaryKey;autoIncrement:false"`
	Title   string
	Content string

	AuthorId       int64
	AuthorNickname string `gorm:"column:author_nickname;->;-:migration"`
	Status         uint8
	CreatedAt      int64
	UpdatedAt      int64
	DeletedAt      int64
}
