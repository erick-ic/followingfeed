package article

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"followingfeed/internal/domain"
	ijwt "followingfeed/internal/handler/jwt"
	"followingfeed/internal/service"
	svcmocks "followingfeed/internal/service/mocks"
	"followingfeed/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

//go:generate mockgen -source=../../service/article.go -package=svcmocks -destination=../../service/mocks/article.mock.go

func TestArticleHandlerEdit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testCases := []struct {
		name       string
		expectErr  error
		expectBody string
	}{
		{
			name:       "编辑成功",
			expectBody: `{"code":0,"msg":"编辑成功～","data":11}`},
		{
			name:       "系统异常",
			expectErr:  errors.New("database unavailable"),
			expectBody: `{"code":5,"msg":"系统错误！","data":null}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := svcmocks.NewMockArticleService(ctrl)
			svc.EXPECT().Save(gomock.Any(), domain.Article{
				Title: "标题", Content: "正文", Author: domain.Author{Id: 1},
			}).Return(int64(11), tc.expectErr)

			server := gin.New()
			server.Use(func(ctx *gin.Context) {
				ctx.Set("claims", &ijwt.UserClaims{Uid: 1})
			})
			NewArticleHandler(
				svc,
				&logger.NopLogger{},
				svcmocks.NewMockInteractiveService(ctrl),
			).RegisterArticlesRouters(server)
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/articles/edit",
				bytes.NewBufferString(`{"title":"标题","content":"正文"}`),
			)
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, req)

			assert.Equal(t, expectedHTTPStatus(tc.expectBody), resp.Code)
			assert.Equal(t, tc.expectBody, resp.Body.String())
		})
	}
}

func TestArticleHandlerPublish(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testCases := []struct {
		name       string
		expectErr  error
		expectBody string
	}{
		{
			name:       "发表成功",
			expectBody: `{"code":0,"msg":"发表成功～","data":12}`},
		{
			name:       "系统异常",
			expectErr:  errors.New("database unavailable"),
			expectBody: `{"code":5,"msg":"系统错误！","data":null}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := svcmocks.NewMockArticleService(ctrl)
			svc.EXPECT().Publish(gomock.Any(), domain.Article{
				Title: "标题", Content: "正文", Author: domain.Author{Id: 1},
			}).Return(int64(12), tc.expectErr)

			server := gin.New()
			server.Use(func(ctx *gin.Context) {
				ctx.Set("claims", &ijwt.UserClaims{Uid: 1})
			})
			NewArticleHandler(
				svc,
				&logger.NopLogger{},
				svcmocks.NewMockInteractiveService(ctrl),
			).RegisterArticlesRouters(server)
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/articles/publish",
				bytes.NewBufferString(`{"title":"标题","content":"正文"}`),
			)
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, req)

			assert.Equal(t, expectedHTTPStatus(tc.expectBody), resp.Code)
			assert.Equal(t, tc.expectBody, resp.Body.String())
		})
	}
}

func TestArticleHandlerWithdraw(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testCases := []struct {
		name       string
		expectErr  error
		expectBody string
	}{
		{
			name:       "撤回成功",
			expectBody: `{"code":0,"msg":"文章撤回成功～","data":13}`},
		{
			name:       "系统异常",
			expectErr:  errors.New("database unavailable"),
			expectBody: `{"code":5,"msg":"系统错误！","data":null}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := svcmocks.NewMockArticleService(ctrl)
			svc.EXPECT().Withdraw(gomock.Any(), int64(13), int64(1)).Return(int64(13), tc.expectErr)

			server := gin.New()
			server.Use(func(ctx *gin.Context) {
				ctx.Set("claims", &ijwt.UserClaims{Uid: 1})
			})
			NewArticleHandler(
				svc,
				&logger.NopLogger{},
				svcmocks.NewMockInteractiveService(ctrl),
			).RegisterArticlesRouters(server)
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/articles/withdraw",
				bytes.NewBufferString(`{"id":13}`),
			)
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, req)

			assert.Equal(t, expectedHTTPStatus(tc.expectBody), resp.Code)
			assert.Equal(t, tc.expectBody, resp.Body.String())
		})
	}
}

func TestArticleHandlerDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testCases := []struct {
		name       string
		expectErr  error
		expectBody string
	}{
		{
			name:       "删除成功",
			expectBody: `{"code":0,"msg":"删除文章成功～","data":14}`},
		{
			name:       "删除失败",
			expectErr:  errors.New("database unavailable"),
			expectBody: `{"code":5,"msg":"删除文章失败！","data":null}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := svcmocks.NewMockArticleService(ctrl)
			svc.EXPECT().Delete(gomock.Any(), int64(14), int64(1)).Return(int64(14), tc.expectErr)

			server := gin.New()
			server.Use(func(ctx *gin.Context) {
				ctx.Set("claims", &ijwt.UserClaims{Uid: 1})
			})
			NewArticleHandler(
				svc,
				&logger.NopLogger{},
				svcmocks.NewMockInteractiveService(ctrl),
			).RegisterArticlesRouters(server)
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/articles/delete",
				bytes.NewBufferString(`{"id":14}`),
			)
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, req)

			assert.Equal(t, expectedHTTPStatus(tc.expectBody), resp.Code)
			assert.Equal(t, tc.expectBody, resp.Body.String())
		})
	}
}

func TestArticleHandlerArticleList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testCases := []struct {
		name       string
		query      string
		mock       func(*gomock.Controller) service.ArticleService
		expectBody string
	}{
		{
			name:  "查询成功",
			query: "?page=2&pageSize=5",
			mock: func(ctrl *gomock.Controller) service.ArticleService {
				svc := svcmocks.NewMockArticleService(ctrl)
				svc.EXPECT().List(gomock.Any(), int64(1), 5, 5).Return([]domain.Article{{
					Id: 7, Title: "标题", Content: "正文", Status: domain.ArticleStatusPublished,
				}}, nil)
				svc.EXPECT().Count(gomock.Any(), int64(1)).Return(int64(1), nil)
				svc.EXPECT().
					CountByStatus(gomock.Any(), int64(1)).
					Return(map[domain.ArticleStatus]int64{
						domain.ArticleStatusUnPublished: 2,
						domain.ArticleStatusPublished:   3,
					}, nil)
				return svc
			},
			expectBody: `{"code":0,"msg":"success","data":{"list":{"items":[{"id":7,"title":"标题","abstract":"正文","status":2,"createdAt":0,"updatedAt":0}],"page":2,"pageSize":5,"total":1,"totalPages":1},"summary":{"draft":2,"published":3}}}`,
		},
		{
			name:  "分页参数错误",
			query: "?page=0&pageSize=5",
			mock: func(ctrl *gomock.Controller) service.ArticleService {
				return svcmocks.NewMockArticleService(ctrl)
			},
			expectBody: `{"code":4,"msg":"page 和 pageSize 必须大于 0","data":null}`,
		},
		{
			name:  "系统异常",
			query: "?page=1&pageSize=10",
			mock: func(ctrl *gomock.Controller) service.ArticleService {
				svc := svcmocks.NewMockArticleService(ctrl)
				svc.EXPECT().
					List(gomock.Any(), int64(1), 0, 10).
					Return(nil, errors.New("database unavailable"))
				return svc
			},
			expectBody: `{"code":5,"msg":"系统错误！","data":null}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			server := gin.New()
			server.Use(func(ctx *gin.Context) {
				ctx.Set("claims", &ijwt.UserClaims{Uid: 1})
			})
			NewArticleHandler(
				tc.mock(ctrl),
				&logger.NopLogger{},
				svcmocks.NewMockInteractiveService(ctrl),
			).RegisterArticlesRouters(server)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/articles/list"+tc.query, nil)
			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, req)

			assert.Equal(t, expectedHTTPStatus(tc.expectBody), resp.Code)
			assert.Equal(t, tc.expectBody, resp.Body.String())
		})
	}
}

func TestArticleHandlerUserPublishedArticles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testCases := []struct {
		name       string
		path       string
		mock       func(*gomock.Controller) service.ArticleService
		expectCode int
		expectBody string
	}{
		{
			name: "查询成功",
			path: "/api/v1/pub/users/2/articles?page=2&pageSize=5",
			mock: func(ctrl *gomock.Controller) service.ArticleService {
				svc := svcmocks.NewMockArticleService(ctrl)
				svc.EXPECT().
					PubListByAuthor(gomock.Any(), int64(2), 5, 5).
					Return([]domain.PublishArticle{{
						Id: 7, Title: "标题", Content: "正文", Status: domain.ArticleStatusPublished,
						Author: domain.Author{Id: 2, Nickname: "云端旅人"},
					}}, nil)
				svc.EXPECT().CountPublishedByAuthor(gomock.Any(), int64(2)).Return(int64(6), nil)
				return svc
			},
			expectCode: http.StatusOK,
			expectBody: `{"code":0,"msg":"success","data":{"items":[{"id":7,"title":"标题","abstract":"正文","authorId":2,"authorNickname":"云端旅人","status":2,"createdAt":0,"updatedAt":0}],"page":2,"pageSize":5,"total":6,"totalPages":2}}`,
		},
		{
			name:       "作者ID错误",
			path:       "/api/v1/pub/users/invalid/articles?page=1&pageSize=5",
			mock:       func(ctrl *gomock.Controller) service.ArticleService { return svcmocks.NewMockArticleService(ctrl) },
			expectCode: http.StatusBadRequest,
			expectBody: `{"code":4,"msg":"用户 id 参数错误","data":null}`,
		},
		{
			name:       "分页参数错误",
			path:       "/api/v1/pub/users/2/articles?page=0&pageSize=5",
			mock:       func(ctrl *gomock.Controller) service.ArticleService { return svcmocks.NewMockArticleService(ctrl) },
			expectCode: http.StatusBadRequest,
			expectBody: `{"code":4,"msg":"page 和 pageSize 必须大于 0","data":null}`,
		},
		{
			name: "查询失败",
			path: "/api/v1/pub/users/2/articles?page=1&pageSize=5",
			mock: func(ctrl *gomock.Controller) service.ArticleService {
				svc := svcmocks.NewMockArticleService(ctrl)
				svc.EXPECT().
					PubListByAuthor(gomock.Any(), int64(2), 0, 5).
					Return(nil, errors.New("database unavailable"))
				return svc
			},
			expectCode: http.StatusInternalServerError,
			expectBody: `{"code":5,"msg":"查询用户文章失败","data":null}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			server := gin.New()
			NewArticleHandler(
				tc.mock(ctrl),
				&logger.NopLogger{},
				svcmocks.NewMockInteractiveService(ctrl),
			).RegisterArticlesRouters(server)
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, req)
			assert.Equal(t, tc.expectCode, resp.Code)
			assert.Equal(t, tc.expectBody, resp.Body.String())
		})
	}
}

func TestArticleHandlerPubList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testCases := []struct {
		name       string
		query      string
		mock       func(*gomock.Controller) service.ArticleService
		expectBody string
	}{
		{
			name:  "查询成功",
			query: "?page=2&pageSize=5",
			mock: func(ctrl *gomock.Controller) service.ArticleService {
				svc := svcmocks.NewMockArticleService(ctrl)
				svc.EXPECT().PubList(gomock.Any(), 5, 5).Return([]domain.PublishArticle{{
					Id: 7, Title: "AI 代理", Content: "文章内容", Status: domain.ArticleStatusPublished,
					Author: domain.Author{Id: 3, Nickname: "云端旅人"},
				}}, nil)
				svc.EXPECT().CountPub(gomock.Any()).Return(int64(1), nil)
				return svc
			},
			expectBody: `{"code":0,"msg":"success","data":{"items":[{"id":7,"title":"AI 代理","abstract":"文章内容","authorId":3,"authorNickname":"云端旅人","status":2,"createdAt":0,"updatedAt":0}],"page":2,"pageSize":5,"total":1,"totalPages":1}}`,
		},
		{
			name:  "分页参数错误",
			query: "?page=0&pageSize=10",
			mock: func(ctrl *gomock.Controller) service.ArticleService {
				return svcmocks.NewMockArticleService(ctrl)
			},
			expectBody: `{"code":4,"msg":"page 和 pageSize 必须大于 0","data":null}`,
		},
		{
			name:  "系统异常",
			query: "?page=1&pageSize=10",
			mock: func(ctrl *gomock.Controller) service.ArticleService {
				svc := svcmocks.NewMockArticleService(ctrl)
				svc.EXPECT().
					PubList(gomock.Any(), 0, 10).
					Return(nil, errors.New("database unavailable"))
				return svc
			},
			expectBody: `{"code":5,"msg":"系统错误！","data":null}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			server := gin.New()
			NewArticleHandler(
				tc.mock(ctrl),
				&logger.NopLogger{},
				svcmocks.NewMockInteractiveService(ctrl),
			).RegisterArticlesRouters(server)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/pub/list"+tc.query, nil)
			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, req)

			assert.Equal(t, expectedHTTPStatus(tc.expectBody), resp.Code)
			assert.Equal(t, tc.expectBody, resp.Body.String())
		})
	}
}

func TestArticleHandlerDetailRejectsAnotherUsersDraft(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	svc := svcmocks.NewMockArticleService(ctrl)
	svc.EXPECT().GetById(gomock.Any(), int64(7), int64(1)).Return(domain.Article{
		Id: 7, Title: "私有草稿", Content: "不应泄露", Author: domain.Author{Id: 2},
	}, nil)

	server := gin.New()
	server.Use(func(ctx *gin.Context) {
		ctx.Set("claims", &ijwt.UserClaims{Uid: 1})
	})
	NewArticleHandler(
		svc,
		&logger.NopLogger{},
		svcmocks.NewMockInteractiveService(ctrl),
	).RegisterArticlesRouters(server)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles/detail/7", nil)
	resp := httptest.NewRecorder()
	server.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusForbidden, resp.Code)
	assert.Equal(t, `{"code":4,"msg":"无权查看此文章","data":null}`, resp.Body.String())
}

func TestArticleHandlerRejectsOversizedPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	server := gin.New()
	server.Use(func(ctx *gin.Context) {
		ctx.Set("claims", &ijwt.UserClaims{Uid: 1})
	})
	NewArticleHandler(
		svcmocks.NewMockArticleService(ctrl),
		&logger.NopLogger{},
		svcmocks.NewMockInteractiveService(ctrl),
	).RegisterArticlesRouters(server)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles/list?page=1&pageSize=51", nil)
	resp := httptest.NewRecorder()
	server.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Equal(t, `{"code":4,"msg":"pageSize 不能超过 50","data":null}`, resp.Body.String())
}

func expectedHTTPStatus(body string) int {
	if strings.Contains(body, `"code":5`) {
		return http.StatusInternalServerError
	}
	if strings.Contains(body, `"code":4`) {
		return http.StatusBadRequest
	}
	return http.StatusOK
}
