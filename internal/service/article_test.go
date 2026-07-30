package service

import (
	"context"
	"errors"
	"testing"

	"followingfeed/internal/domain"
	"followingfeed/internal/repository"
	repomocks "followingfeed/internal/repository/mocks"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

//go:generate mockgen -source=../repository/article.go -package=repomocks -destination=../repository/mocks/article.mock.go

func TestArticleServiceSave(t *testing.T) {
	testCases := []struct {
		name     string
		article  domain.Article
		mock     func(*gomock.Controller) repository.ArticleRepository
		expectID int64
		wantErr  error
	}{
		{
			name:    "创建草稿",
			article: domain.Article{Title: "标题", Content: "正文", Author: domain.Author{Id: 1}},
			mock: func(ctrl *gomock.Controller) repository.ArticleRepository {
				repo := repomocks.NewMockArticleRepository(ctrl)
				repo.EXPECT().Create(gomock.Any(), domain.Article{
					Title: "标题", Content: "正文", Author: domain.Author{Id: 1},
					Status: domain.ArticleStatusUnPublished,
				}).Return(int64(9), nil)
				return repo
			},
			expectID: 9,
		},
		{
			name:    "更新草稿",
			article: domain.Article{Id: 8, Title: "标题", Content: "正文", Author: domain.Author{Id: 1}},
			mock: func(ctrl *gomock.Controller) repository.ArticleRepository {
				repo := repomocks.NewMockArticleRepository(ctrl)
				repo.EXPECT().Update(gomock.Any(), domain.Article{
					Id: 8, Title: "标题", Content: "正文", Author: domain.Author{Id: 1},
					Status: domain.ArticleStatusUnPublished,
				}).Return(nil)
				return repo
			},
			expectID: 8,
		},
		{
			name:    "创建失败",
			article: domain.Article{Title: "标题", Author: domain.Author{Id: 1}},
			mock: func(ctrl *gomock.Controller) repository.ArticleRepository {
				repo := repomocks.NewMockArticleRepository(ctrl)
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(int64(0), errDatabaseUnavailable)
				return repo
			},
			wantErr: errDatabaseUnavailable,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			id, err := NewArticleServiceImpl(tc.mock(ctrl)).Save(context.Background(), tc.article)
			assert.Equal(t, tc.expectID, id)
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestArticleServicePublish(t *testing.T) {
	testCases := []struct {
		name     string
		repoErr  error
		expectID int64
	}{
		{name: "发布成功", expectID: 10},
		{name: "发布失败", repoErr: errDatabaseUnavailable},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := repomocks.NewMockArticleRepository(ctrl)
			repo.EXPECT().Sync(gomock.Any(), domain.Article{
				Title: "标题", Author: domain.Author{Id: 1}, Status: domain.ArticleStatusPublished,
			}).Return(tc.expectID, tc.repoErr)

			id, err := NewArticleServiceImpl(repo).Publish(
				context.Background(),
				domain.Article{Title: "标题", Author: domain.Author{Id: 1}},
			)
			assert.Equal(t, tc.expectID, id)
			assert.ErrorIs(t, err, tc.repoErr)
		})
	}
}

func TestArticleServiceWithdraw(t *testing.T) {
	testCases := []struct {
		name     string
		repoErr  error
		expectID int64
	}{
		{name: "撤回成功", expectID: 3},
		{name: "撤回失败", repoErr: errDatabaseUnavailable},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := repomocks.NewMockArticleRepository(ctrl)
			repo.EXPECT().SyncStatus(
				gomock.Any(), int64(3), int64(1), domain.ArticleStatusUnPublished,
			).Return(tc.expectID, tc.repoErr)

			id, err := NewArticleServiceImpl(repo).Withdraw(context.Background(), 3, 1)
			assert.Equal(t, tc.expectID, id)
			assert.ErrorIs(t, err, tc.repoErr)
		})
	}
}

func TestArticleServiceDelete(t *testing.T) {
	testCases := []struct {
		name     string
		repoErr  error
		expectID int64
	}{
		{name: "删除成功", expectID: 4},
		{name: "删除失败", repoErr: errDatabaseUnavailable},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := repomocks.NewMockArticleRepository(ctrl)
			repo.EXPECT().SoftDelete(gomock.Any(), int64(4), int64(1)).
				Return(tc.expectID, tc.repoErr)

			id, err := NewArticleServiceImpl(repo).Delete(context.Background(), 4, 1)
			assert.Equal(t, tc.expectID, id)
			assert.ErrorIs(t, err, tc.repoErr)
		})
	}
}

func TestArticleServiceList(t *testing.T) {
	testCases := []struct {
		name    string
		repoErr error
		want    []domain.Article
	}{
		{name: "查询成功", want: []domain.Article{{Id: 1, Title: "标题"}}},
		{name: "查询失败", repoErr: errDatabaseUnavailable},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := repomocks.NewMockArticleRepository(ctrl)
			repo.EXPECT().List(gomock.Any(), int64(1), 5, 10).Return(tc.want, tc.repoErr)

			got, err := NewArticleServiceImpl(repo).List(context.Background(), 1, 5, 10)
			assert.Equal(t, tc.want, got)
			assert.ErrorIs(t, err, tc.repoErr)
		})
	}
}

func TestArticleServiceGetById(t *testing.T) {
	testCases := []struct {
		name    string
		repoErr error
		want    domain.Article
	}{
		{name: "查询成功", want: domain.Article{Id: 1, Title: "标题"}},
		{name: "查询失败", repoErr: errDatabaseUnavailable},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := repomocks.NewMockArticleRepository(ctrl)
			repo.EXPECT().GetById(gomock.Any(), int64(1)).Return(tc.want, tc.repoErr)

			got, err := NewArticleServiceImpl(repo).GetById(context.Background(), 1)
			assert.Equal(t, tc.want, got)
			assert.ErrorIs(t, err, tc.repoErr)
		})
	}
}

func TestArticleServicePubList(t *testing.T) {
	testCases := []struct {
		name    string
		repoErr error
		want    []domain.PublishArticle
	}{
		{name: "查询成功", want: []domain.PublishArticle{{Id: 1, Title: "标题"}}},
		{name: "查询失败", repoErr: errors.New("database unavailable")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := repomocks.NewMockArticleRepository(ctrl)
			repo.EXPECT().PubList(gomock.Any(), 0, 10).Return(tc.want, tc.repoErr)

			got, err := NewArticleServiceImpl(repo).PubList(context.Background(), 0, 10)
			assert.Equal(t, tc.want, got)
			assert.ErrorIs(t, err, tc.repoErr)
		})
	}
}

func TestArticleServiceGetByPubId(t *testing.T) {
	testCases := []struct {
		name    string
		repoErr error
		want    domain.Article
	}{
		{name: "查询成功", want: domain.Article{Id: 1, Title: "标题"}},
		{name: "查询失败", repoErr: errDatabaseUnavailable},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := repomocks.NewMockArticleRepository(ctrl)
			repo.EXPECT().GetByPubId(gomock.Any(), int64(1)).Return(tc.want, tc.repoErr)

			got, err := NewArticleServiceImpl(repo).GetByPubId(context.Background(), 1)
			assert.Equal(t, tc.want, got)
			assert.ErrorIs(t, err, tc.repoErr)
		})
	}
}
