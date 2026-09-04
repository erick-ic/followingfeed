package repository

//go:generate mockgen -source=follow.go -package=repomocks -destination=mocks/follow.mock.go

import (
	"context"
	"followingfeed/internal/domain"
	"followingfeed/internal/repository/dao"
)

var ErrFollowDuplicated = dao.ErrFollowDuplicated

type FollowRepository interface {
	Follow(ctx context.Context, uid int64, targetId int64) error
	UnFollow(ctx context.Context, uid int64, targetId int64) error
	IsFollowing(ctx context.Context, uid int64, targetId int64) (bool, error)
	ListFollowingPage(ctx context.Context, uid int64, offset, limit int) ([]domain.Follow, error)
	CountFollowing(ctx context.Context, uid int64) (int64, error)
	ListFollowersPage(ctx context.Context, uid int64, offset, limit int) ([]domain.Follow, error)
	CountFollowers(ctx context.Context, uid int64) (int64, error)
}

type FollowRepositoryImpl struct {
	dao dao.FollowDAO
}

func (fr *FollowRepositoryImpl) CountFollowers(ctx context.Context, uid int64) (int64, error) {
	return fr.dao.CountFollowers(ctx, uid)
}

func (fr *FollowRepositoryImpl) ListFollowersPage(
	ctx context.Context,
	uid int64,
	offset, limit int,
) ([]domain.Follow, error) {
	items, err := fr.dao.ListFollowersPage(ctx, uid, offset, limit)
	if err != nil {
		return nil, err
	}
	return fr.toDomains(items), nil
}

func (fr *FollowRepositoryImpl) CountFollowing(ctx context.Context, uid int64) (int64, error) {
	return fr.dao.CountFollowing(ctx, uid)
}

func (fr *FollowRepositoryImpl) ListFollowingPage(
	ctx context.Context,
	uid int64,
	offset, limit int,
) ([]domain.Follow, error) {
	items, err := fr.dao.ListFollowingPage(ctx, uid, offset, limit)
	if err != nil {
		return nil, err
	}
	return fr.toDomains(items), nil
}

func (fr *FollowRepositoryImpl) IsFollowing(
	ctx context.Context,
	uid int64,
	targetId int64,
) (bool, error) {
	return fr.dao.IsFollowing(ctx, uid, targetId)
}

func (fr *FollowRepositoryImpl) UnFollow(ctx context.Context, uid int64, targetId int64) error {
	return fr.dao.UnFollow(ctx, uid, targetId)
}

func (fr *FollowRepositoryImpl) Follow(ctx context.Context, uid int64, targetId int64) error {
	return fr.dao.Insert(ctx, uid, targetId)
}

func NewFollowRepository(dao dao.FollowDAO) FollowRepository {
	return &FollowRepositoryImpl{
		dao: dao,
	}
}

func (fr *FollowRepositoryImpl) toDomains(items []dao.Follow) []domain.Follow {
	res := make([]domain.Follow, 0, len(items))
	for _, item := range items {
		res = append(res, domain.Follow{
			Id:          item.Id,
			FollowerId:  item.FollowerId,
			FollowingId: item.FollowingId,
			Nickname:    item.Nickname,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}
	return res
}
