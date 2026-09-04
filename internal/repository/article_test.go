package repository

import (
	"context"
	"errors"
	"testing"

	"followingfeed/internal/domain"
	cachemocks "followingfeed/internal/repository/cache/mocks"
	"followingfeed/internal/repository/dao"
	daomocks "followingfeed/internal/repository/dao/mocks"
	"followingfeed/pkg/logger"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

//go:generate mockgen -source=article.go -package=repomocks -destination=mocks/article.mock.go
//go:generate mockgen -source=dao/article.go -package=daomocks -destination=dao/mocks/article.mock.go
//go:generate mockgen -source=cache/article.go -package=cachemocks -destination=cache/mocks/article.mock.go

func TestArticleRepositoryGetById(t *testing.T) {
	testCases := []struct {
		name       string
		daoArticle dao.Article
		daoErr     error
		want       domain.Article
	}{
		{
			name:       "查询成功",
			daoArticle: dao.Article{Id: 1, Title: "标题", Content: "正文", AuthorId: 2, Status: 2},
			want: domain.Article{
				Id: 1, Title: "标题", Content: "正文", Author: domain.Author{Id: 2},
				Status: domain.ArticleStatusPublished,
			},
		},
		{name: "查询失败", daoErr: errors.New("database unavailable")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			daoMock := daomocks.NewMockArticleDAO(ctrl)
			daoMock.EXPECT().
				GetById(gomock.Any(), int64(1), int64(2)).
				Return(tc.daoArticle, tc.daoErr)

			got, err := NewArticleRepositoryImpl(daoMock, nil, &logger.NopLogger{}).
				GetById(context.Background(), 1, 2)
			assert.Equal(t, tc.want, got)
			assert.ErrorIs(t, err, tc.daoErr)
		})
	}
}

func TestArticleRepositoryCountByStatus(t *testing.T) {
	testCases := []struct {
		name   string
		rows   []dao.ArticleStatusCount
		daoErr error
		want   map[domain.ArticleStatus]int64
	}{
		{
			name: "统计成功",
			rows: []dao.ArticleStatusCount{{Status: 1, Count: 2}, {Status: 2, Count: 3}},
			want: map[domain.ArticleStatus]int64{
				domain.ArticleStatusUnPublished: 2,
				domain.ArticleStatusPublished:   3,
			},
		},
		{name: "统计失败", daoErr: errors.New("database unavailable")},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			daoMock := daomocks.NewMockArticleDAO(ctrl)
			daoMock.EXPECT().CountByAuthorStatus(gomock.Any(), int64(1)).Return(tc.rows, tc.daoErr)
			got, err := NewArticleRepositoryImpl(
				daoMock,
				nil,
				&logger.NopLogger{},
			).CountByStatus(context.Background(), 1)
			assert.Equal(t, tc.want, got)
			assert.ErrorIs(t, err, tc.daoErr)
		})
	}
}

func TestArticleRepositoryCount(t *testing.T) {
	ctrl := gomock.NewController(t)
	daoMock := daomocks.NewMockArticleDAO(ctrl)
	daoMock.EXPECT().CountByAuthor(gomock.Any(), int64(1)).Return(int64(5), nil)
	got, err := NewArticleRepositoryImpl(
		daoMock,
		nil,
		&logger.NopLogger{},
	).Count(context.Background(), 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), got)
}

func TestArticleRepositoryGetByPubId(t *testing.T) {
	testCases := []struct {
		name       string
		daoArticle dao.PublishArticle
		daoErr     error
		want       domain.Article
	}{
		{
			name: "查询成功",
			daoArticle: dao.PublishArticle{
				Id: 1, Title: "标题", AuthorId: 2, AuthorNickname: "云端旅人", Status: 2,
			},
			want: domain.Article{
				Id: 1, Title: "标题", Author: domain.Author{Id: 2, Nickname: "云端旅人"},
				Status: domain.ArticleStatusPublished,
			},
		},
		{name: "查询失败", daoErr: errors.New("database unavailable")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			daoMock := daomocks.NewMockArticleDAO(ctrl)
			daoMock.EXPECT().GetByPubId(gomock.Any(), int64(1)).Return(tc.daoArticle, tc.daoErr)

			got, err := NewArticleRepositoryImpl(daoMock, nil, &logger.NopLogger{}).
				GetByPubId(context.Background(), 1)
			assert.Equal(t, tc.want, got)
			assert.ErrorIs(t, err, tc.daoErr)
		})
	}
}

func TestArticleRepositoryPubList(t *testing.T) {
	testCases := []struct {
		name    string
		daoData []dao.PublishArticle
		daoErr  error
		want    []domain.PublishArticle
	}{
		{
			name: "查询成功",
			daoData: []dao.PublishArticle{{
				Id: 1, Title: "标题", AuthorId: 2, AuthorNickname: "云端旅人", Status: 2,
			}},
			want: []domain.PublishArticle{{
				Id: 1, Title: "标题", Author: domain.Author{Id: 2, Nickname: "云端旅人"},
				Status: domain.ArticleStatusPublished,
			}},
		},
		{name: "查询失败", daoErr: errors.New("database unavailable")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			daoMock := daomocks.NewMockArticleDAO(ctrl)
			daoMock.EXPECT().GetPublished(gomock.Any(), 0, 10).Return(tc.daoData, tc.daoErr)

			got, err := NewArticleRepositoryImpl(daoMock, nil, &logger.NopLogger{}).
				PubList(context.Background(), 0, 10)
			assert.Equal(t, tc.want, got)
			assert.ErrorIs(t, err, tc.daoErr)
		})
	}
}

func TestArticleRepositoryPublishedCache(t *testing.T) {
	t.Run("公开首页缓存命中", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		daoMock := daomocks.NewMockArticleDAO(ctrl)
		cacheMock := cachemocks.NewMockArticleCache(ctrl)
		want := []domain.PublishArticle{{Id: 1, Title: "缓存文章"}}
		cacheMock.EXPECT().GetPublishedFirstPage(gomock.Any()).Return(want, nil)

		got, err := NewArticleRepositoryImpl(daoMock, cacheMock, &logger.NopLogger{}).
			PubList(context.Background(), 0, articleFirstPageSize)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("公开首页缓存未命中并回源", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		daoMock := daomocks.NewMockArticleDAO(ctrl)
		cacheMock := cachemocks.NewMockArticleCache(ctrl)
		cacheMock.EXPECT().GetPublishedFirstPage(gomock.Any()).Return(nil, redis.Nil).Times(2)
		daoMock.EXPECT().GetPublished(gomock.Any(), 0, articleFirstPageSize).
			Return([]dao.PublishArticle{{Id: 2, Title: "数据库文章", AuthorId: 1}}, nil)
		want := []domain.PublishArticle{{Id: 2, Title: "数据库文章", Author: domain.Author{Id: 1}}}
		cacheMock.EXPECT().SetPublishedFirstPage(gomock.Any(), want).Return(nil)

		got, err := NewArticleRepositoryImpl(daoMock, cacheMock, &logger.NopLogger{}).
			PubList(context.Background(), 0, articleFirstPageSize)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("已发布总数缓存命中", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		daoMock := daomocks.NewMockArticleDAO(ctrl)
		cacheMock := cachemocks.NewMockArticleCache(ctrl)
		cacheMock.EXPECT().GetPublishedCount(gomock.Any()).Return(int64(7), nil)

		count, err := NewArticleRepositoryImpl(daoMock, cacheMock, &logger.NopLogger{}).
			CountPub(context.Background())
		require.NoError(t, err)
		assert.Equal(t, int64(7), count)
	})
}

func TestArticleRepositoryList(t *testing.T) {
	testCases := []struct {
		name    string
		offset  int
		limit   int
		mock    func(*gomock.Controller) (*daomocks.MockArticleDAO, *cachemocks.MockArticleCache)
		want    []domain.Article
		wantErr error
	}{
		{
			name:  "缓存未命中并回源",
			limit: 10,
			mock: func(ctrl *gomock.Controller) (*daomocks.MockArticleDAO, *cachemocks.MockArticleCache) {
				daoMock := daomocks.NewMockArticleDAO(ctrl)
				cacheMock := cachemocks.NewMockArticleCache(ctrl)
				cacheMock.EXPECT().
					GetFirstPage(gomock.Any(), int64(1)).
					Return(nil, redis.Nil).
					Times(2)
				daoMock.EXPECT().GetByAuthor(gomock.Any(), int64(1), 0, 10).
					Return([]dao.Article{{Id: 2, Title: "数据库文章", AuthorId: 1}}, nil)
				cacheMock.EXPECT().SetFirstPage(
					gomock.Any(), int64(1),
					[]domain.Article{{Id: 2, Title: "数据库文章", Author: domain.Author{Id: 1}}},
				).Return(nil)
				return daoMock, cacheMock
			},
			want: []domain.Article{{Id: 2, Title: "数据库文章", Author: domain.Author{Id: 1}}},
		},
		{
			name:  "首页缓存命中",
			limit: 10,
			mock: func(ctrl *gomock.Controller) (*daomocks.MockArticleDAO, *cachemocks.MockArticleCache) {
				daoMock := daomocks.NewMockArticleDAO(ctrl)
				cacheMock := cachemocks.NewMockArticleCache(ctrl)
				cacheMock.EXPECT().GetFirstPage(gomock.Any(), int64(1)).
					Return([]domain.Article{{Id: 1, Title: "缓存文章"}}, nil)
				return daoMock, cacheMock
			},
			want: []domain.Article{{Id: 1, Title: "缓存文章"}},
		},
		{
			name:   "非首页直接查询数据库",
			offset: 10,
			limit:  10,
			mock: func(ctrl *gomock.Controller) (*daomocks.MockArticleDAO, *cachemocks.MockArticleCache) {
				daoMock := daomocks.NewMockArticleDAO(ctrl)
				cacheMock := cachemocks.NewMockArticleCache(ctrl)
				daoMock.EXPECT().GetByAuthor(gomock.Any(), int64(1), 10, 10).
					Return([]dao.Article{{Id: 3, Title: "第二页"}}, nil)
				return daoMock, cacheMock
			},
			want: []domain.Article{{Id: 3, Title: "第二页"}},
		},
		{
			name:  "数据库异常",
			limit: 10,
			mock: func(ctrl *gomock.Controller) (*daomocks.MockArticleDAO, *cachemocks.MockArticleCache) {
				daoMock := daomocks.NewMockArticleDAO(ctrl)
				cacheMock := cachemocks.NewMockArticleCache(ctrl)
				cacheMock.EXPECT().
					GetFirstPage(gomock.Any(), int64(1)).
					Return(nil, redis.Nil).
					Times(2)
				daoMock.EXPECT().GetByAuthor(gomock.Any(), int64(1), 0, 10).
					Return(nil, errors.New("database unavailable"))
				return daoMock, cacheMock
			},
			wantErr: errors.New("database unavailable"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			daoMock, cacheMock := tc.mock(ctrl)
			got, err := NewArticleRepositoryImpl(daoMock, cacheMock, &logger.NopLogger{}).
				List(context.Background(), 1, tc.offset, tc.limit)
			assert.Equal(t, tc.want, got)
			if tc.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.wantErr.Error())
			}
		})
	}
}

func TestArticleRepositoryCreate(t *testing.T) {
	testCases := []struct {
		name     string
		daoErr   error
		expectID int64
	}{
		{name: "创建成功", expectID: 8},
		{name: "创建失败", daoErr: errors.New("database unavailable")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			daoMock := daomocks.NewMockArticleDAO(ctrl)
			cacheMock := cachemocks.NewMockArticleCache(ctrl)
			daoMock.EXPECT().Insert(gomock.Any(), dao.Article{
				Title: "标题", Content: "正文", AuthorId: 1, Status: 1,
			}).Return(tc.expectID, tc.daoErr)
			if tc.daoErr == nil {
				cacheMock.EXPECT().DelFirstPage(gomock.Any(), int64(1)).Return(nil)
			}

			id, err := NewArticleRepositoryImpl(daoMock, cacheMock, &logger.NopLogger{}).
				Create(context.Background(), domain.Article{
					Title: "标题", Content: "正文", Author: domain.Author{Id: 1},
					Status: domain.ArticleStatusUnPublished,
				})
			assert.Equal(t, tc.expectID, id)
			assert.ErrorIs(t, err, tc.daoErr)
		})
	}
}

func TestArticleRepositoryUpdate(t *testing.T) {
	testCases := []struct {
		name   string
		daoErr error
	}{
		{name: "更新成功"},
		{name: "更新失败", daoErr: errors.New("database unavailable")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			daoMock := daomocks.NewMockArticleDAO(ctrl)
			cacheMock := cachemocks.NewMockArticleCache(ctrl)
			daoMock.EXPECT().UpdateByArticleId(gomock.Any(), dao.Article{
				Id: 8, Title: "标题", Content: "正文", AuthorId: 1, Status: 1,
			}).Return(tc.daoErr)
			if tc.daoErr == nil {
				cacheMock.EXPECT().DelFirstPage(gomock.Any(), int64(1)).Return(nil)
			}

			err := NewArticleRepositoryImpl(daoMock, cacheMock, &logger.NopLogger{}).
				Update(context.Background(), domain.Article{
					Id: 8, Title: "标题", Content: "正文", Author: domain.Author{Id: 1},
					Status: domain.ArticleStatusUnPublished,
				})
			assert.ErrorIs(t, err, tc.daoErr)
		})
	}
}

func TestArticleRepositorySync(t *testing.T) {
	testCases := []struct {
		name     string
		daoErr   error
		expectID int64
	}{
		{name: "同步成功", expectID: 8},
		{name: "同步失败", daoErr: errors.New("database unavailable")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			daoMock := daomocks.NewMockArticleDAO(ctrl)
			cacheMock := cachemocks.NewMockArticleCache(ctrl)
			daoMock.EXPECT().Sync(gomock.Any(), dao.Article{
				Id: 8, Title: "标题", AuthorId: 1, Status: 2,
			}).Return(tc.expectID, tc.daoErr)
			if tc.daoErr == nil {
				cacheMock.EXPECT().DelFirstPage(gomock.Any(), int64(1)).Return(nil)
				cacheMock.EXPECT().DelPublished(gomock.Any()).Return(nil)
			}

			id, err := NewArticleRepositoryImpl(daoMock, cacheMock, &logger.NopLogger{}).
				Sync(context.Background(), domain.Article{
					Id: 8, Title: "标题", Author: domain.Author{Id: 1},
					Status: domain.ArticleStatusPublished,
				})
			assert.Equal(t, tc.expectID, id)
			assert.ErrorIs(t, err, tc.daoErr)
		})
	}
}

func TestArticleRepositorySyncStatus(t *testing.T) {
	testCases := []struct {
		name     string
		daoErr   error
		expectID int64
	}{
		{name: "同步成功", expectID: 8},
		{name: "同步失败", daoErr: errors.New("database unavailable")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			daoMock := daomocks.NewMockArticleDAO(ctrl)
			cacheMock := cachemocks.NewMockArticleCache(ctrl)
			daoMock.EXPECT().SyncStatus(
				gomock.Any(), int64(8), int64(1), domain.ArticleStatusUnPublished,
			).Return(tc.expectID, tc.daoErr)
			if tc.daoErr == nil {
				cacheMock.EXPECT().DelFirstPage(gomock.Any(), int64(1)).Return(nil)
				cacheMock.EXPECT().DelPublished(gomock.Any()).Return(nil)
			}

			id, err := NewArticleRepositoryImpl(daoMock, cacheMock, &logger.NopLogger{}).
				SyncStatus(context.Background(), 8, 1, domain.ArticleStatusUnPublished)
			assert.Equal(t, tc.expectID, id)
			assert.ErrorIs(t, err, tc.daoErr)
		})
	}
}

func TestArticleRepositorySoftDelete(t *testing.T) {
	testCases := []struct {
		name     string
		daoErr   error
		expectID int64
	}{
		{name: "删除成功", expectID: 8},
		{name: "删除失败", daoErr: errors.New("database unavailable")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			daoMock := daomocks.NewMockArticleDAO(ctrl)
			cacheMock := cachemocks.NewMockArticleCache(ctrl)
			daoMock.EXPECT().SoftDelete(gomock.Any(), int64(8), int64(1)).
				Return(tc.expectID, tc.daoErr)
			if tc.daoErr == nil {
				cacheMock.EXPECT().DelFirstPage(gomock.Any(), int64(1)).Return(nil)
				cacheMock.EXPECT().DelPublished(gomock.Any()).Return(nil)
			}

			id, err := NewArticleRepositoryImpl(daoMock, cacheMock, &logger.NopLogger{}).
				SoftDelete(context.Background(), 8, 1)
			assert.Equal(t, tc.expectID, id)
			assert.ErrorIs(t, err, tc.daoErr)
		})
	}
}
