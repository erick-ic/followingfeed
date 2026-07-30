package dao

import (
	"context"
	"fmt"
	"followingfeed/internal/domain"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ArticleDAO interface {
	Insert(ctx context.Context, article Article) (int64, error)
	UpdateByArticleId(ctx context.Context, article Article) error
	Sync(ctx context.Context, article Article) (int64, error) // 事务内同步文章到制作库和线上库
	Upsert(ctx context.Context, article PublishArticle) error
	SyncStatus(ctx context.Context, articleId int64, uid int64, status domain.ArticleStatus) (int64, error)
	SoftDelete(ctx context.Context, articleId int64, uid int64) (int64, error)
	GetByAuthor(ctx context.Context, uid int64, offset int, limit int) ([]Article, error)
	GetPublished(ctx context.Context, offset int, limit int) ([]PublishArticle, error)
	GetById(ctx context.Context, id int64) (Article, error)
	GetByPubId(ctx context.Context, id int64) (PublishArticle, error)
}

type ArticleDAOImpl struct {
	db *gorm.DB
}

func (ad *ArticleDAOImpl) GetByPubId(ctx context.Context, id int64) (PublishArticle, error) {
	var pubArticle PublishArticle
	err := ad.db.WithContext(ctx).
		Select("publish_articles.*, users.nickname AS author_nickname").
		Joins("LEFT JOIN users ON users.id = publish_articles.author_id").
		Where(
			"publish_articles.id = ? AND publish_articles.status = ? AND publish_articles.deleted_at = 0",
			id,
			domain.ArticleStatusPublished.ToUint8(),
		).
		First(&pubArticle).Error
	return pubArticle, err
}

func (ad *ArticleDAOImpl) GetById(ctx context.Context, id int64) (Article, error) {
	var article Article
	err := ad.db.WithContext(ctx).Where("id = ? AND deleted_at = 0", id).First(&article).Error
	return article, err
}

func (ad *ArticleDAOImpl) GetByAuthor(ctx context.Context, uid int64, offset int, limit int) ([]Article, error) {
	var articles []Article
	err := ad.db.WithContext(ctx).Model(&articles).
		Where("author_id = ? AND deleted_at = 0", uid).
		Offset(offset).
		Limit(limit).
		Order("updated_at DESC").
		Find(&articles).Error
	return articles, err
}

// GetPublished 从线上库查询全部已发布文章。
// 线上库可能保留已撤回文章，因此必须显式过滤已发布状态。
func (ad *ArticleDAOImpl) GetPublished(ctx context.Context, offset int, limit int) ([]PublishArticle, error) {
	var articles []PublishArticle
	err := ad.db.WithContext(ctx).Model(&PublishArticle{}).
		Select("publish_articles.*, users.nickname AS author_nickname").
		Joins("LEFT JOIN users ON users.id = publish_articles.author_id").
		Where(
			"publish_articles.status = ? AND publish_articles.deleted_at = 0",
			domain.ArticleStatusPublished.ToUint8(),
		).
		Offset(offset).
		Limit(limit).
		Order("updated_at DESC").
		Find(&articles).Error
	return articles, err
}

func (ad *ArticleDAOImpl) SyncStatus(ctx context.Context, articleId int64, uid int64, status domain.ArticleStatus) (int64, error) {
	var (
		id = articleId
	)
	now := time.Now().UnixMilli()

	return id, ad.db.Transaction(func(tx *gorm.DB) error {
		// 制作库：带作者ID校验，防止修改他人文章
		res := tx.Model(&Article{}).
			Where("id = ? AND author_id = ? AND deleted_at = 0", id, uid).
			Updates(map[string]any{
				"status":     status,
				"updated_at": now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return fmt.Errorf("更新失败，可能是作者非法，id %d，author_id %d", articleId, uid)
		}
		// 线上库：只按 id 更新（读者视角，无需校验作者）
		return tx.Model(&PublishArticle{}).
			Where("id = ? ", id).
			Updates(map[string]any{
				"status":     status,
				"updated_at": now,
			}).Error
	})
}

// SoftDelete 同时软删除制作库和线上库记录。线上库没有对应记录时不视为错误。
func (ad *ArticleDAOImpl) SoftDelete(ctx context.Context, articleId int64, uid int64) (int64, error) {
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
		if res.RowsAffected != 1 {
			return fmt.Errorf("删除失败，文章不存在或无权限，id %d，author_id %d", articleId, uid)
		}

		return tx.Model(&PublishArticle{}).
			Where("id = ? AND deleted_at = 0", articleId).
			Updates(map[string]any{
				"deleted_at": now,
				"updated_at": now,
			}).Error
	})
}

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

// Upsert 线上库插入或更新（UPSERT），实现INSERT OR UPDATE语义
// 使用GORM的OnConflict子句，对应MySQL的INSERT ... ON DUPLICATE KEY UPDATE
// 若主键冲突则更新标题、内容、状态和更新时间，否则插入新记录
func (ad *ArticleDAOImpl) Upsert(ctx context.Context, article PublishArticle) error {
	now := time.Now().UnixMilli()
	article.CreatedAt = now
	article.UpdatedAt = now
	err := ad.db.Clauses(clause.OnConflict{
		DoUpdates: clause.Assignments(map[string]interface{}{
			"title":      article.Title,
			"content":    article.Content,
			"status":     article.Status,
			"updated_at": now,
		}),
	}).Create(&article).Error
	return err
}

func (ad *ArticleDAOImpl) Insert(ctx context.Context, article Article) (int64, error) {
	now := time.Now().UnixMilli()
	article.CreatedAt = now
	article.UpdatedAt = now
	err := ad.db.WithContext(ctx).Create(&article).Error
	return article.Id, err
}

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
		return fmt.Errorf("更新失败，可能是作者非法，id %d，author_id %d", article.Id, article.AuthorId)
	}
	return res.Error
}

func NewGORMArticleDAO(db *gorm.DB) ArticleDAO {
	return &ArticleDAOImpl{
		db: db,
	}
}

type Article struct {
	Id      int64  `gorm:"primaryKey;autoIncrement"` // 主键ID
	Title   string `gorm:"type=varchar(1024)"`       // 文章标题
	Content string `gorm:"type=BLOB"`                // 文章内容（大文本）

	AuthorId       int64  `gorm:"index"`                                 // 作者ID（索引）
	AuthorNickname string `gorm:"column:author_nickname;->;-:migration"` // 查询公开文章时关联读取
	Status         uint8
	CreatedAt      int64
	UpdatedAt      int64
	DeletedAt      int64 `gorm:"index"`
}

type PublishArticle Article
