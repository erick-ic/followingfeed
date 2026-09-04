package repository

import (
	"context"
	"errors"
	"testing"

	"followingfeed/internal/domain"
	"followingfeed/internal/repository/dao"
	daomocks "followingfeed/internal/repository/dao/mocks"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

//go:generate mockgen -source=interactive.go -package=repomocks -destination=mocks/interactive.mock.go
//go:generate mockgen -source=dao/interactive.go -package=daomocks -destination=dao/mocks/interactive.mock.go
func TestInteractiveRepositoryIncrLike(t *testing.T) {
	ctrl := gomock.NewController(t)
	d := daomocks.NewMockInteractiveDAO(ctrl)
	d.EXPECT().InsertLikeInfo(gomock.Any(), "article", int64(2), int64(1)).Return(true, nil)
	assert.NoError(t, NewInteractiveRepository(d).IncrLike(context.Background(), "article", 2, 1))
}

func TestInteractiveRepositoryIncrLikeIdempotent(t *testing.T) {
	ctrl := gomock.NewController(t)
	d := daomocks.NewMockInteractiveDAO(ctrl)
	d.EXPECT().InsertLikeInfo(gomock.Any(), "article", int64(2), int64(1)).Return(false, nil)
	assert.NoError(t, NewInteractiveRepository(d).IncrLike(context.Background(), "article", 2, 1))
}

func TestInteractiveRepositoryDecrLike(t *testing.T) {
	ctrl := gomock.NewController(t)
	d := daomocks.NewMockInteractiveDAO(ctrl)
	d.EXPECT().DeleteLikeInfo(gomock.Any(), "article", int64(2), int64(1)).Return(true, nil)
	assert.NoError(t, NewInteractiveRepository(d).DecrLike(context.Background(), "article", 2, 1))
}

func TestInteractiveRepositoryGet(t *testing.T) {
	ctrl := gomock.NewController(t)
	d := daomocks.NewMockInteractiveDAO(ctrl)
	d.EXPECT().Get(gomock.Any(), "article", int64(2)).Return(dao.Interactive{
		Biz: "article", BizId: 2, ReadCnt: 3, LikeCnt: 4, CollectCnt: 5,
	}, nil)
	got, err := NewInteractiveRepository(d).Get(context.Background(), "article", 2)
	assert.NoError(t, err)
	assert.Equal(
		t,
		domain.Interactive{Biz: "article", BizId: 2, ReadCnt: 3, LikeCnt: 4, CollectCnt: 5},
		got,
	)
}

func TestInteractiveRepositoryGetNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	d := daomocks.NewMockInteractiveDAO(ctrl)
	d.EXPECT().
		Get(gomock.Any(), "article", int64(2)).
		Return(dao.Interactive{}, dao.ErrInteractiveNotFound)
	got, err := NewInteractiveRepository(d).Get(context.Background(), "article", 2)
	assert.NoError(t, err)
	assert.Equal(t, domain.Interactive{Biz: "article", BizId: 2}, got)
}

func TestInteractiveRepositoryLikedError(t *testing.T) {
	ctrl := gomock.NewController(t)
	d := daomocks.NewMockInteractiveDAO(ctrl)
	wantErr := errors.New("database unavailable")
	d.EXPECT().Liked(gomock.Any(), "article", int64(2), int64(1)).Return(false, wantErr)
	got, err := NewInteractiveRepository(d).Liked(context.Background(), "article", 2, 1)
	assert.False(t, got)
	assert.ErrorIs(t, err, wantErr)
}

func TestInteractiveRepositoryIncrRead(t *testing.T) {
	testCases := []struct {
		name   string
		daoErr error
	}{
		{name: "增加成功"},
		{name: "增加失败", daoErr: errors.New("database unavailable")},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			d := daomocks.NewMockInteractiveDAO(ctrl)
			d.EXPECT().IncrRead(gomock.Any(), "article", int64(2)).Return(tc.daoErr)
			err := NewInteractiveRepository(d).IncrRead(context.Background(), "article", 2)
			assert.ErrorIs(t, err, tc.daoErr)
		})
	}
}

func TestInteractiveRepositoryBatchGet(t *testing.T) {
	testCases := []struct {
		name   string
		rows   []dao.Interactive
		daoErr error
		want   map[int64]domain.Interactive
	}{
		{
			name: "查询成功",
			rows: []dao.Interactive{
				{Biz: "article", BizId: 2, ReadCnt: 4, LikeCnt: 3, CollectCnt: 5},
			},
			want: map[int64]domain.Interactive{
				2: {Biz: "article", BizId: 2, ReadCnt: 4, LikeCnt: 3, CollectCnt: 5},
			},
		},
		{name: "查询失败", daoErr: errors.New("database unavailable")},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			d := daomocks.NewMockInteractiveDAO(ctrl)
			d.EXPECT().BatchGet(gomock.Any(), "article", []int64{2, 3}).Return(tc.rows, tc.daoErr)
			got, err := NewInteractiveRepository(
				d,
			).BatchGet(context.Background(), "article", []int64{2, 3})
			assert.Equal(t, tc.want, got)
			assert.ErrorIs(t, err, tc.daoErr)
		})
	}
}

func TestInteractiveRepositoryIncrCollect(t *testing.T) {
	ctrl := gomock.NewController(t)
	d := daomocks.NewMockInteractiveDAO(ctrl)
	d.EXPECT().InsertCollectInfo(gomock.Any(), "article", int64(2), int64(1)).Return(true, nil)
	assert.NoError(
		t,
		NewInteractiveRepository(d).IncrCollect(context.Background(), "article", 2, 1),
	)
}

func TestInteractiveRepositoryDecrCollect(t *testing.T) {
	ctrl := gomock.NewController(t)
	d := daomocks.NewMockInteractiveDAO(ctrl)
	d.EXPECT().DeleteCollectInfo(gomock.Any(), "article", int64(2), int64(1)).Return(true, nil)
	assert.NoError(
		t,
		NewInteractiveRepository(d).DecrCollect(context.Background(), "article", 2, 1),
	)
}

func TestInteractiveRepositoryCollected(t *testing.T) {
	ctrl := gomock.NewController(t)
	d := daomocks.NewMockInteractiveDAO(ctrl)
	d.EXPECT().Collected(gomock.Any(), "article", int64(2), int64(1)).Return(true, nil)
	got, err := NewInteractiveRepository(d).Collected(context.Background(), "article", 2, 1)
	assert.NoError(t, err)
	assert.True(t, got)
}

func TestInteractiveRepositoryListCollected(t *testing.T) {
	ctrl := gomock.NewController(t)
	d := daomocks.NewMockInteractiveDAO(ctrl)
	d.EXPECT().
		ListCollected(gomock.Any(), "article", int64(1), 0, 10).
		Return([]dao.PublishArticle{{Id: 2, Title: "收藏文章", AuthorId: 3, AuthorNickname: "云端旅人", Status: 2}}, nil)
	got, err := NewInteractiveRepository(d).ListCollected(context.Background(), "article", 1, 0, 10)
	assert.NoError(t, err)
	assert.Equal(
		t,
		[]domain.PublishArticle{
			{
				Id:     2,
				Title:  "收藏文章",
				Author: domain.Author{Id: 3, Nickname: "云端旅人"},
				Status: domain.ArticleStatusPublished,
			},
		},
		got,
	)
}

func TestInteractiveRepositoryCountCollected(t *testing.T) {
	ctrl := gomock.NewController(t)
	d := daomocks.NewMockInteractiveDAO(ctrl)
	d.EXPECT().CountCollected(gomock.Any(), "article", int64(1)).Return(int64(6), nil)
	got, err := NewInteractiveRepository(d).CountCollected(context.Background(), "article", 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(6), got)
}
