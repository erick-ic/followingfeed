package service

import (
	"context"
	"errors"
	"testing"

	"followingfeed/internal/domain"
	"followingfeed/internal/repository"
	cachemocks "followingfeed/internal/repository/cache/mocks"
	repomocks "followingfeed/internal/repository/mocks"
	"followingfeed/pkg/logger"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

//go:generate mockgen -source=../repository/article.go -package=repomocks -destination=../repository/mocks/article.mock.go

func newArticleServiceForTest(repo repository.ArticleRepository) ArticleService {
	return NewArticleServiceImpl(repo, noopUserProfileCacheInvalidator{}, &logger.NopLogger{})
}

func TestArticleServicePublishInvalidatesProfileCaches(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := repomocks.NewMockArticleRepository(ctrl)
	invalidator := cachemocks.NewMockUserProfileCacheInvalidator(ctrl)
	article := domain.Article{Author: domain.Author{Id: 1}}
	repo.EXPECT().Sync(gomock.Any(), gomock.Any()).Return(int64(10), nil)
	invalidator.EXPECT().DelPublicProfiles(gomock.Any(), int64(1)).Return(nil)
	invalidator.EXPECT().DelArticleStats(gomock.Any(), int64(1)).Return(nil)

	id, err := NewArticleServiceImpl(repo, invalidator, &logger.NopLogger{}).
		Publish(context.Background(), article)

	assert.NoError(t, err)
	assert.Equal(t, int64(10), id)
}

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
			name: "更新草稿",
			article: domain.Article{
				Id:      8,
				Title:   "标题",
				Content: "正文",
				Author:  domain.Author{Id: 1},
			},
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
				repo.EXPECT().
					Create(gomock.Any(), gomock.Any()).
					Return(int64(0), errDatabaseUnavailable)
				return repo
			},
			wantErr: errDatabaseUnavailable,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			id, err := newArticleServiceForTest(
				tc.mock(ctrl),
			).Save(context.Background(), tc.article)
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

			id, err := newArticleServiceForTest(repo).Publish(
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

			id, err := newArticleServiceForTest(repo).Withdraw(context.Background(), 3, 1)
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

			id, err := newArticleServiceForTest(repo).Delete(context.Background(), 4, 1)
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

			got, err := newArticleServiceForTest(repo).List(context.Background(), 1, 5, 10)
			assert.Equal(t, tc.want, got)
			assert.ErrorIs(t, err, tc.repoErr)
		})
	}
}

func TestArticleServiceCountByStatus(t *testing.T) {
	testCases := []struct {
		name    string
		want    map[domain.ArticleStatus]int64
		repoErr error
	}{
		{
			name: "统计成功",
			want: map[domain.ArticleStatus]int64{
				domain.ArticleStatusUnPublished: 2,
				domain.ArticleStatusPublished:   3,
			},
		},
		{name: "统计失败", repoErr: errDatabaseUnavailable},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := repomocks.NewMockArticleRepository(ctrl)
			repo.EXPECT().CountByStatus(gomock.Any(), int64(1)).Return(tc.want, tc.repoErr)
			got, err := newArticleServiceForTest(repo).CountByStatus(context.Background(), 1)
			assert.Equal(t, tc.want, got)
			assert.ErrorIs(t, err, tc.repoErr)
		})
	}
}

func TestArticleServiceCount(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := repomocks.NewMockArticleRepository(ctrl)
	repo.EXPECT().Count(gomock.Any(), int64(1)).Return(int64(5), nil)
	got, err := newArticleServiceForTest(repo).Count(context.Background(), 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), got)
}

func TestArticleServiceFeed(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := repomocks.NewMockArticleRepository(ctrl)
	want := []domain.PublishArticle{{Id: 8, Title: "关注文章"}}
	repo.EXPECT().Feed(gomock.Any(), int64(1), 10, 10).Return(want, nil)
	got, err := newArticleServiceForTest(repo).Feed(context.Background(), 1, 10, 10)
	assert.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestArticleServicePubListByAuthor(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := repomocks.NewMockArticleRepository(ctrl)
	want := []domain.PublishArticle{{Id: 9, Author: domain.Author{Id: 2}}}
	repo.EXPECT().PubListByAuthor(gomock.Any(), int64(2), 0, 20).Return(want, nil)
	got, err := newArticleServiceForTest(repo).PubListByAuthor(context.Background(), 2, 0, 20)
	assert.NoError(t, err)
	assert.Equal(t, want, got)
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
			repo.EXPECT().GetById(gomock.Any(), int64(1), int64(2)).Return(tc.want, tc.repoErr)

			got, err := newArticleServiceForTest(repo).GetById(context.Background(), 1, 2)
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

			got, err := newArticleServiceForTest(repo).PubList(context.Background(), 0, 10)
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

			got, err := newArticleServiceForTest(repo).GetByPubId(context.Background(), 1)
			assert.Equal(t, tc.want, got)
			assert.ErrorIs(t, err, tc.repoErr)
		})
	}
}
