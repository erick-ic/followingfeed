package dao

import (
	"context"
	"errors"
	"followingfeed/internal/domain"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInteractiveNotFound       = gorm.ErrRecordNotFound
	ErrInteractiveTargetNotFound = errors.New("互动目标不存在或不可操作")
)

type InteractiveDAO interface {
	BatchGet(ctx context.Context, biz string, bizIDs []int64) ([]Interactive, error)
	Get(ctx context.Context, biz string, bizID int64) (Interactive, error)
	Liked(ctx context.Context, biz string, bizID, uid int64) (bool, error)
	Collected(ctx context.Context, biz string, bizID, uid int64) (bool, error)
	InsertLikeInfo(ctx context.Context, biz string, bizID, uid int64) (bool, error)
	DeleteLikeInfo(ctx context.Context, biz string, bizID, uid int64) (bool, error)
	InsertCollectInfo(ctx context.Context, biz string, bizID, uid int64) (bool, error)
	DeleteCollectInfo(ctx context.Context, biz string, bizID, uid int64) (bool, error)
	ListCollected(
		ctx context.Context,
		biz string,
		uid int64,
		offset, limit int,
	) ([]PublishArticle, error)
	CountCollected(ctx context.Context, biz string, uid int64) (int64, error)

	IncrRead(ctx context.Context, biz string, bizID int64) error
	GetArticleStatsByAuthor(ctx context.Context, biz string, authorID int64) (ArticleStats, error)
}

type interactiveDAO struct {
	db *gorm.DB
}

func (d *interactiveDAO) GetArticleStatsByAuthor(
	ctx context.Context,
	biz string,
	authorID int64,
) (ArticleStats, error) {
	var stats ArticleStats
	err := d.db.WithContext(ctx).Model(&PublishArticle{}).
		Select(`COALESCE(SUM(i.like_cnt), 0) AS like_count,
                COALESCE(SUM(i.read_cnt), 0) AS read_count,
                COALESCE(SUM(i.collect_cnt), 0) AS collect_count`).
		Joins("LEFT JOIN interactives AS i ON i.biz = ? AND i.biz_id = publish_articles.id", biz).
		Where(
			"publish_articles.author_id = ? AND publish_articles.status = ? AND publish_articles.deleted_at = 0",
			authorID, domain.ArticleStatusPublished.ToUint8(),
		).
		Scan(&stats).Error
	return stats, err
}

func (d *interactiveDAO) IncrRead(ctx context.Context, biz string, bizID int64) error {
	now := time.Now().UnixMilli()
	return d.db.WithContext(ctx).Clauses(clause.OnConflict{
		DoUpdates: clause.Assignments(map[string]any{
			"read_cnt":   gorm.Expr("read_cnt + 1"),
			"updated_at": now,
		})}).
		Create(&Interactive{
			Biz:       biz,
			BizId:     bizID,
			ReadCnt:   1,
			CreatedAt: now,
			UpdatedAt: now,
		}).Error
}

func (d *interactiveDAO) CountCollected(ctx context.Context, biz string, uid int64) (int64, error) {
	var count int64
	err := d.db.WithContext(ctx).Model(&PublishArticle{}).
		Joins(
			"JOIN user_collection_bizs ON "+
				"user_collection_bizs.biz_id = publish_articles.id AND "+
				"user_collection_bizs.biz = ? AND user_collection_bizs.uid = ? AND "+
				"user_collection_bizs.status = 1",
			biz,
			uid,
		).
		Where("publish_articles.status = ? AND publish_articles.deleted_at = 0", 2).
		Count(&count).Error
	return count, err
}

func (d *interactiveDAO) ListCollected(
	ctx context.Context,
	biz string,
	uid int64,
	offset, limit int,
) ([]PublishArticle, error) {
	var articles []PublishArticle
	err := d.db.WithContext(ctx).Model(&PublishArticle{}).
		Select(publishArticleListSelect).
		Joins(
			"JOIN user_collection_bizs ON "+
				"user_collection_bizs.biz_id = publish_articles.id AND "+
				"user_collection_bizs.biz = ? AND user_collection_bizs.uid = ? AND "+
				"user_collection_bizs.status = 1",
			biz,
			uid,
		).
		Joins("LEFT JOIN users ON users.id = publish_articles.author_id").
		Where("publish_articles.status = ? AND publish_articles.deleted_at = 0", 2).
		Order("user_collection_bizs.updated_at DESC, user_collection_bizs.id DESC").
		Offset(offset).
		Limit(limit).
		Find(&articles).Error
	return articles, err
}

func (d *interactiveDAO) DeleteCollectInfo(
	ctx context.Context,
	biz string,
	bizID, uid int64,
) (bool, error) {
	now := time.Now().UnixMilli()
	changed := false
	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&UserCollectionBiz{}).
			Where("uid = ? AND biz = ? AND biz_id = ? AND status = 1", uid, biz, bizID).
			Updates(map[string]any{
				"status":     0,
				"updated_at": now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}
		changed = true
		return tx.Model(&Interactive{}).
			Where("biz = ? AND biz_id = ?", biz, bizID).
			Updates(map[string]any{
				"collect_cnt": gorm.Expr("GREATEST(collect_cnt - 1, 0)"),
				"updated_at":  now,
			}).Error
	})
	return changed, err
}

func (d *interactiveDAO) InsertCollectInfo(
	ctx context.Context,
	biz string,
	bizID, uid int64,
) (bool, error) {
	now := time.Now().UnixMilli()
	changed := false
	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureInteractiveTarget(tx, biz, bizID); err != nil {
			return err
		}
		res := tx.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&UserCollectionBiz{
				Uid:       uid,
				Biz:       biz,
				BizId:     bizID,
				Status:    1,
				CreatedAt: now,
				UpdatedAt: now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			res = tx.Model(&UserCollectionBiz{}).
				Where("uid = ? AND biz = ? AND biz_id = ? AND status = 0", uid, biz, bizID).
				Updates(map[string]any{
					"status":     1,
					"updated_at": now,
				})
			if res.Error != nil {
				return res.Error
			}
		}
		if res.RowsAffected == 0 {
			return nil
		}
		changed = true
		return tx.Clauses(clause.OnConflict{
			DoUpdates: clause.Assignments(map[string]any{
				"collect_cnt": gorm.Expr("collect_cnt + 1"),
				"updated_at":  now,
			})}).
			Create(&Interactive{
				Biz:        biz,
				BizId:      bizID,
				CollectCnt: 1,
				CreatedAt:  now,
				UpdatedAt:  now,
			}).Error
	})
	return changed, err
}

func (d *interactiveDAO) DeleteLikeInfo(
	ctx context.Context,
	biz string,
	bizID, uid int64,
) (bool, error) {
	now := time.Now().UnixMilli()
	changed := false
	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&UserLikeBiz{}).
			Where("uid = ? AND biz = ? AND biz_id = ? AND status = 1", uid, biz, bizID).
			Updates(map[string]any{
				"status":     0,
				"updated_at": now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}
		changed = true
		return tx.Model(&Interactive{}).
			Where("biz = ? AND biz_id = ?", biz, bizID).
			Updates(map[string]any{
				"like_cnt":   gorm.Expr("GREATEST(like_cnt - 1, 0)"),
				"updated_at": now,
			}).Error
	})
	return changed, err
}

func (d *interactiveDAO) InsertLikeInfo(
	ctx context.Context,
	biz string,
	bizID, uid int64,
) (bool, error) {
	now := time.Now().UnixMilli()
	changed := false
	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureInteractiveTarget(tx, biz, bizID); err != nil {
			return err
		}
		// 首次点赞时插入状态为 1 的关系记录；
		res := tx.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&UserLikeBiz{
				Uid:       uid,
				Biz:       biz,
				BizId:     bizID,
				Status:    1,
				CreatedAt: now,
				UpdatedAt: now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			// 若唯一键冲突，说明该用户以前操作过该对象。
			// 唯一键冲突后仅恢复已取消的记录（status=0）；已处于点赞状态的记录不会被更新。
			res = tx.Model(&UserLikeBiz{}).
				Where("uid = ? AND biz = ? AND biz_id = ? AND status = 0", uid, biz, bizID).
				Updates(map[string]any{"status": 1, "updated_at": now})
			if res.Error != nil {
				return res.Error
			}
		}
		if res.RowsAffected == 0 {
			// 插入和恢复都没有改变数据，表示重复点赞；按幂等成功返回，避免重复增加点赞数。
			return nil
		}
		// 只有点赞关系从“未点赞”变为“已点赞”时，才同步增加聚合点赞数。
		changed = true
		return tx.Clauses(clause.OnConflict{
			DoUpdates: clause.Assignments(map[string]any{
				"like_cnt":   gorm.Expr("like_cnt + 1"),
				"updated_at": now,
			})}).
			Create(&Interactive{
				Biz:       biz,
				BizId:     bizID,
				LikeCnt:   1,
				CreatedAt: now,
				UpdatedAt: now,
			}).Error
	})
	return changed, err
}

// 在互动写入的同一事务中校验多态业务目标。
// 共享锁会与文章状态变更协调，避免为正在撤回或删除的文章提交新互动。
func ensureInteractiveTarget(tx *gorm.DB, biz string, bizID int64) error {
	if biz != "article" {
		// 引入其他多态业务类型时，应分别提供对应的目标校验逻辑。
		// 当前接口只开放文章互动。
		return nil
	}
	var article PublishArticle
	err := tx.Clauses(clause.Locking{Strength: "SHARE"}).
		Select("id").
		Where("id = ? AND status = ? AND deleted_at = 0", bizID, domain.ArticleStatusPublished.ToUint8()).
		First(&article).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrInteractiveTargetNotFound
	}
	return err
}

func (d *interactiveDAO) Collected(
	ctx context.Context,
	biz string,
	bizID, uid int64,
) (bool, error) {
	var count int64
	err := d.db.WithContext(ctx).Model(&UserCollectionBiz{}).
		Where("uid = ? AND biz = ? AND biz_id = ? AND status = 1", uid, biz, bizID).
		Count(&count).Error
	return count > 0, err
}

func (d *interactiveDAO) Liked(ctx context.Context, biz string, bizID, uid int64) (bool, error) {
	var count int64
	err := d.db.WithContext(ctx).Model(&UserLikeBiz{}).
		Where("uid = ? AND biz = ? AND biz_id = ? AND status = 1", uid, biz, bizID).
		Count(&count).Error
	return count > 0, err
}

func (d *interactiveDAO) Get(ctx context.Context, biz string, bizID int64) (Interactive, error) {
	var inter Interactive
	err := d.db.WithContext(ctx).
		Where("biz = ? AND biz_id = ?", biz, bizID).
		First(&inter).Error
	return inter, err
}

func (d *interactiveDAO) BatchGet(
	ctx context.Context,
	biz string,
	bizIDs []int64,
) ([]Interactive, error) {
	if len(bizIDs) == 0 {
		return []Interactive{}, nil
	}
	var items []Interactive
	err := d.db.WithContext(ctx).Where("biz = ? AND biz_id IN ?", biz, bizIDs).
		Find(&items).Error
	return items, err
}

func NewInteractiveDAO(db *gorm.DB) InteractiveDAO {
	return &interactiveDAO{
		db: db,
	}
}

type ArticleStats struct {
	LikeCount    int64
	ReadCount    int64
	CollectCount int64
}

// Interactive 对应 interactives 聚合表，保存一个业务对象的阅读、点赞和收藏总数。
// Biz 与 BizId 组成唯一业务键，使文章等不同业务类型可以复用同一套互动统计。
// 点赞或收藏关系实际变化时，应与对应的用户明细表在同一事务中更新，
// 避免计数不一致。
type Interactive struct {
	Id         int64 `gorm:"primaryKey;autoIncrement"`
	BizId      int64
	Biz        string
	ReadCnt    int64
	LikeCnt    int64
	CollectCnt int64
	CreatedAt  int64
	UpdatedAt  int64
}

// UserLikeBiz 对应 user_like_bizs 点赞明细表，记录“哪个用户点赞了哪个业务对象”。
// Uid、BizId 与 Biz 组成唯一键，用于阻止重复记录；
// Status=0 保留取消历史，Status=1 表示当前已点赞。
// 该表负责用户点赞状态和幂等判断，点赞总数由 Interactive 聚合表提供。
type UserLikeBiz struct {
	Id        int64 `gorm:"primaryKey;autoIncrement"`
	Uid       int64
	BizId     int64
	Biz       string
	Status    uint8 // 0未点赞 1点赞
	CreatedAt int64
	UpdatedAt int64
}

type UserCollectionBiz struct {
	Id        int64 `gorm:"primaryKey;autoIncrement"`
	Uid       int64
	BizId     int64
	Biz       string
	Status    uint8
	CreatedAt int64
	UpdatedAt int64
}
