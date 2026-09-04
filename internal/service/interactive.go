package service

import (
	"context"

	"followingfeed/internal/domain"
	"followingfeed/internal/repository"
)

const articleBiz = "article"

var ErrInteractiveTargetNotFound = repository.ErrInteractiveTargetNotFound

type InteractiveService interface {
	BatchGet(ctx context.Context, biz string, bizIDs []int64) (map[int64]domain.Interactive, error)
	Get(ctx context.Context, biz string, bizID, uid int64) (domain.Interactive, error)
	Like(ctx context.Context, biz string, bizID, uid int64) error
	CancelLike(ctx context.Context, biz string, bizID, uid int64) error
	Collect(ctx context.Context, biz string, bizID, uid int64) error
	CancelCollect(ctx context.Context, biz string, bizID, uid int64) error
	ListCollected(
		ctx context.Context,
		biz string,
		uid int64,
		offset, limit int,
	) ([]domain.PublishArticle, error)
	CountCollected(ctx context.Context, biz string, uid int64) (int64, error)

	RecordRead(ctx context.Context, biz string, bizID int64) error
	GetArticleStatsByAuthor(
		ctx context.Context,
		biz string,
		authorID int64,
	) (domain.Interactive, error)
}

type interactiveService struct {
	repo repository.InteractiveRepository
}

func (s *interactiveService) GetArticleStatsByAuthor(
	ctx context.Context,
	biz string,
	authorID int64,
) (domain.Interactive, error) {
	return s.repo.GetArticleStatsByAuthor(ctx, biz, authorID)
}

func (s *interactiveService) RecordRead(ctx context.Context, biz string, bizID int64) error {
	return s.repo.IncrRead(ctx, biz, bizID)
}

func (s *interactiveService) CountCollected(
	ctx context.Context,
	biz string,
	uid int64,
) (int64, error) {
	return s.repo.CountCollected(ctx, biz, uid)
}

func (s *interactiveService) ListCollected(
	ctx context.Context,
	biz string,
	uid int64,
	offset, limit int,
) ([]domain.PublishArticle, error) {
	return s.repo.ListCollected(ctx, biz, uid, offset, limit)
}

func (s *interactiveService) CancelCollect(
	ctx context.Context,
	biz string,
	bizID, uid int64,
) error {
	return s.repo.DecrCollect(ctx, biz, bizID, uid)
}

func (s *interactiveService) Collect(ctx context.Context, biz string, bizID, uid int64) error {
	return s.repo.IncrCollect(ctx, biz, bizID, uid)
}

func (s *interactiveService) CancelLike(ctx context.Context, biz string, bizID, uid int64) error {
	return s.repo.DecrLike(ctx, biz, bizID, uid)
}

func (s *interactiveService) Like(ctx context.Context, biz string, bizID, uid int64) error {
	return s.repo.IncrLike(ctx, biz, bizID, uid)
}

func (s *interactiveService) Get(
	ctx context.Context,
	biz string,
	bizID, uid int64,
) (domain.Interactive, error) {
	inter, err := s.repo.Get(ctx, biz, bizID)
	if err != nil {
		return domain.Interactive{}, err
	}
	if uid <= 0 {
		return inter, nil
	}
	inter.Liked, err = s.repo.Liked(ctx, biz, bizID, uid)
	if err != nil {
		return domain.Interactive{}, err
	}
	inter.Collected, err = s.repo.Collected(ctx, biz, bizID, uid)
	return inter, err
}

func (s *interactiveService) BatchGet(
	ctx context.Context,
	biz string,
	bizIDs []int64,
) (map[int64]domain.Interactive, error) {
	return s.repo.BatchGet(ctx, biz, bizIDs)
}

func NewInteractiveService(r repository.InteractiveRepository) InteractiveService {
	return &interactiveService{
		repo: r,
	}
}
