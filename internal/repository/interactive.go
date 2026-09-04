package repository

import (
	"context"
	"errors"

	"followingfeed/internal/domain"
	"followingfeed/internal/repository/dao"
)

var ErrInteractiveTargetNotFound = dao.ErrInteractiveTargetNotFound

type InteractiveRepository interface {
	BatchGet(ctx context.Context, biz string, bizIDs []int64) (map[int64]domain.Interactive, error)
	Get(ctx context.Context, biz string, bizID int64) (domain.Interactive, error)
	Liked(ctx context.Context, biz string, bizID, uid int64) (bool, error)
	Collected(ctx context.Context, biz string, bizID, uid int64) (bool, error)
	IncrLike(ctx context.Context, biz string, bizID, uid int64) error
	DecrLike(ctx context.Context, biz string, bizID, uid int64) error
	IncrCollect(ctx context.Context, biz string, bizID, uid int64) error
	DecrCollect(ctx context.Context, biz string, bizID, uid int64) error
	ListCollected(
		ctx context.Context,
		biz string,
		uid int64,
		offset, limit int,
	) ([]domain.PublishArticle, error)
	CountCollected(ctx context.Context, biz string, uid int64) (int64, error)

	IncrRead(ctx context.Context, biz string, bizID int64) error
	GetArticleStatsByAuthor(
		ctx context.Context,
		biz string,
		authorID int64,
	) (domain.Interactive, error)
}

type interactiveRepository struct {
	dao dao.InteractiveDAO
}

func (r *interactiveRepository) GetArticleStatsByAuthor(
	ctx context.Context,
	biz string,
	authorID int64,
) (domain.Interactive, error) {
	stats, err := r.dao.GetArticleStatsByAuthor(ctx, biz, authorID)
	if err != nil {
		return domain.Interactive{}, err
	}
	return domain.Interactive{
		LikeCnt:    stats.LikeCount,
		ReadCnt:    stats.ReadCount,
		CollectCnt: stats.CollectCount,
	}, nil
}

func (r *interactiveRepository) IncrRead(ctx context.Context, biz string, bizID int64) error {
	return r.dao.IncrRead(ctx, biz, bizID)
}

func (r *interactiveRepository) CountCollected(
	ctx context.Context,
	biz string,
	uid int64,
) (int64, error) {
	return r.dao.CountCollected(ctx, biz, uid)
}

func (r *interactiveRepository) ListCollected(
	ctx context.Context,
	biz string,
	uid int64,
	offset, limit int,
) ([]domain.PublishArticle, error) {
	items, err := r.dao.ListCollected(ctx, biz, uid, offset, limit)
	if err != nil {
		return nil, err
	}
	result := make([]domain.PublishArticle, len(items))
	for i, item := range items {
		result[i] = domain.PublishArticle{
			Id:      item.Id,
			Title:   item.Title,
			Content: item.Content,
			Author: domain.Author{
				Id:       item.AuthorId,
				Nickname: item.AuthorNickname,
			},
			Status:    domain.ArticleStatus(item.Status),
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		}
	}
	return result, nil
}

func (r *interactiveRepository) DecrCollect(
	ctx context.Context,
	biz string,
	bizID, uid int64,
) error {
	_, err := r.dao.DeleteCollectInfo(ctx, biz, bizID, uid)
	return err
}

func (r *interactiveRepository) IncrCollect(
	ctx context.Context,
	biz string,
	bizID, uid int64,
) error {
	_, err := r.dao.InsertCollectInfo(ctx, biz, bizID, uid)
	return err
}

func (r *interactiveRepository) DecrLike(ctx context.Context, biz string, bizID, uid int64) error {
	_, err := r.dao.DeleteLikeInfo(ctx, biz, bizID, uid)
	return err
}

func (r *interactiveRepository) IncrLike(ctx context.Context, biz string, bizID, uid int64) error {
	_, err := r.dao.InsertLikeInfo(ctx, biz, bizID, uid)
	return err
}

func (r *interactiveRepository) Collected(
	ctx context.Context,
	biz string,
	bizID, uid int64,
) (bool, error) {
	return r.dao.Collected(ctx, biz, bizID, uid)
}

func (r *interactiveRepository) Liked(
	ctx context.Context,
	biz string,
	bizID, uid int64,
) (bool, error) {
	return r.dao.Liked(ctx, biz, bizID, uid)
}

func (r *interactiveRepository) Get(
	ctx context.Context,
	biz string,
	bizID int64,
) (domain.Interactive, error) {
	inter, err := r.dao.Get(ctx, biz, bizID)
	if errors.Is(err, dao.ErrInteractiveNotFound) {
		return domain.Interactive{
			Biz:   biz,
			BizId: bizID,
		}, nil
	}
	if err != nil {
		return domain.Interactive{}, err
	}
	return domain.Interactive{
		Biz:        inter.Biz,
		BizId:      inter.BizId,
		ReadCnt:    inter.ReadCnt,
		LikeCnt:    inter.LikeCnt,
		CollectCnt: inter.CollectCnt,
	}, nil
}

func (r *interactiveRepository) BatchGet(
	ctx context.Context,
	biz string,
	bizIDs []int64,
) (map[int64]domain.Interactive, error) {
	items, err := r.dao.BatchGet(ctx, biz, bizIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]domain.Interactive, len(items))
	for _, item := range items {
		result[item.BizId] = domain.Interactive{
			Biz:        item.Biz,
			BizId:      item.BizId,
			ReadCnt:    item.ReadCnt,
			LikeCnt:    item.LikeCnt,
			CollectCnt: item.CollectCnt,
		}
	}
	return result, nil
}

func NewInteractiveRepository(d dao.InteractiveDAO) InteractiveRepository {
	return &interactiveRepository{
		dao: d,
	}
}
