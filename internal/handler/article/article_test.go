package article

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
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
		serviceErr error
		expectBody string
	}{
		{name: "编辑成功", expectBody: `{"code":0,"msg":"编辑成功～","data":11}`},
		{name: "系统异常", serviceErr: errors.New("database unavailable"), expectBody: `{"code":5,"msg":"系统错误！","data":null}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := svcmocks.NewMockArticleService(ctrl)
			svc.EXPECT().Save(gomock.Any(), domain.Article{
				Title: "标题", Content: "正文", Author: domain.Author{Id: 1},
			}).Return(int64(11), tc.serviceErr)

			server := gin.New()
			server.Use(func(ctx *gin.Context) {
				ctx.Set("claims", &ijwt.UserClaims{Uid: 1})
			})
			NewArticleHandler(svc, &logger.NopLogger{}).RegisterArticlesRouters(server)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/articles/edit", bytes.NewBufferString(`{"title":"标题","content":"正文"}`))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, req)

			assert.Equal(t, http.StatusOK, resp.Code)
			assert.Equal(t, tc.expectBody, resp.Body.String())
		})
	}
}

func TestArticleHandlerPublish(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testCases := []struct {
		name       string
		serviceErr error
		expectBody string
	}{
		{name: "发表成功", expectBody: `{"code":0,"msg":"发表成功～","data":12}`},
		{name: "系统异常", serviceErr: errors.New("database unavailable"), expectBody: `{"code":5,"msg":"系统错误！","data":null}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := svcmocks.NewMockArticleService(ctrl)
			svc.EXPECT().Publish(gomock.Any(), domain.Article{
				Title: "标题", Content: "正文", Author: domain.Author{Id: 1},
			}).Return(int64(12), tc.serviceErr)

			server := gin.New()
			server.Use(func(ctx *gin.Context) {
				ctx.Set("claims", &ijwt.UserClaims{Uid: 1})
			})
			NewArticleHandler(svc, &logger.NopLogger{}).RegisterArticlesRouters(server)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/articles/publish", bytes.NewBufferString(`{"title":"标题","content":"正文"}`))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, req)

			assert.Equal(t, http.StatusOK, resp.Code)
			assert.Equal(t, tc.expectBody, resp.Body.String())
		})
	}
}

func TestArticleHandlerWithdraw(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testCases := []struct {
		name       string
		serviceErr error
		expectBody string
	}{
		{name: "撤回成功", expectBody: `{"code":0,"msg":"文章撤回成功～","data":13}`},
		{name: "系统异常", serviceErr: errors.New("database unavailable"), expectBody: `{"code":5,"msg":"系统错误！","data":null}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := svcmocks.NewMockArticleService(ctrl)
			svc.EXPECT().Withdraw(gomock.Any(), int64(13), int64(1)).Return(int64(13), tc.serviceErr)

			server := gin.New()
			server.Use(func(ctx *gin.Context) {
				ctx.Set("claims", &ijwt.UserClaims{Uid: 1})
			})
			NewArticleHandler(svc, &logger.NopLogger{}).RegisterArticlesRouters(server)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/articles/withdraw", bytes.NewBufferString(`{"id":13}`))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, req)

			assert.Equal(t, http.StatusOK, resp.Code)
			assert.Equal(t, tc.expectBody, resp.Body.String())
		})
	}
}

func TestArticleHandlerDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testCases := []struct {
		name       string
		serviceErr error
		expectBody string
	}{
		{name: "删除成功", expectBody: `{"code":0,"msg":"删除文章成功～","data":14}`},
		{name: "删除失败", serviceErr: errors.New("database unavailable"), expectBody: `{"code":5,"msg":"删除文章失败！","data":null}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := svcmocks.NewMockArticleService(ctrl)
			svc.EXPECT().Delete(gomock.Any(), int64(14), int64(1)).Return(int64(14), tc.serviceErr)

			server := gin.New()
			server.Use(func(ctx *gin.Context) {
				ctx.Set("claims", &ijwt.UserClaims{Uid: 1})
			})
			NewArticleHandler(svc, &logger.NopLogger{}).RegisterArticlesRouters(server)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/articles/delete", bytes.NewBufferString(`{"id":14}`))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, req)

			assert.Equal(t, http.StatusOK, resp.Code)
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
			name: "查询成功", query: "?page=2&pageSize=5",
			mock: func(ctrl *gomock.Controller) service.ArticleService {
				svc := svcmocks.NewMockArticleService(ctrl)
				svc.EXPECT().List(gomock.Any(), int64(1), 5, 5).Return([]domain.Article{{
					Id: 7, Title: "标题", Content: "正文", Status: domain.ArticleStatusPublished,
				}}, nil)
				return svc
			},
			expectBody: `{"code":0,"msg":"success","data":[{"id":7,"title":"标题","abstract":"正文","status":2,"created_at":"1970-01-01 08:00:00","updated_at":"1970-01-01 08:00:00"}]}`,
		},
		{
			name: "分页参数错误", query: "?page=0&pageSize=5",
			mock: func(ctrl *gomock.Controller) service.ArticleService {
				return svcmocks.NewMockArticleService(ctrl)
			},
			expectBody: `{"code":4,"msg":"page 和 pageSize 必须大于 0","data":null}`,
		},
		{
			name: "系统异常", query: "?page=1&pageSize=10",
			mock: func(ctrl *gomock.Controller) service.ArticleService {
				svc := svcmocks.NewMockArticleService(ctrl)
				svc.EXPECT().List(gomock.Any(), int64(1), 0, 10).Return(nil, errors.New("database unavailable"))
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
			NewArticleHandler(tc.mock(ctrl), &logger.NopLogger{}).RegisterArticlesRouters(server)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/articles/list"+tc.query, nil)
			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, req)

			assert.Equal(t, http.StatusOK, resp.Code)
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
			name: "查询成功", query: "?page=2&pageSize=5",
			mock: func(ctrl *gomock.Controller) service.ArticleService {
				svc := svcmocks.NewMockArticleService(ctrl)
				svc.EXPECT().PubList(gomock.Any(), 5, 5).Return([]domain.PublishArticle{{
					Id: 7, Title: "AI 代理", Content: "文章内容", Status: domain.ArticleStatusPublished,
					Author: domain.Author{Id: 3, Nickname: "云端旅人"},
				}}, nil)
				return svc
			},
			expectBody: `{"code":0,"msg":"success","data":[{"id":7,"title":"AI 代理","abstract":"文章内容","authorId":3,"authorNickname":"云端旅人","status":2,"created_at":"1970-01-01 08:00:00","updated_at":"1970-01-01 08:00:00"}]}`,
		},
		{
			name: "分页参数错误", query: "?page=0&pageSize=10",
			mock: func(ctrl *gomock.Controller) service.ArticleService {
				return svcmocks.NewMockArticleService(ctrl)
			},
			expectBody: `{"code":4,"msg":"page 和 pageSize 必须大于 0","data":null}`,
		},
		{
			name: "系统异常", query: "?page=1&pageSize=10",
			mock: func(ctrl *gomock.Controller) service.ArticleService {
				svc := svcmocks.NewMockArticleService(ctrl)
				svc.EXPECT().PubList(gomock.Any(), 0, 10).Return(nil, errors.New("database unavailable"))
				return svc
			},
			expectBody: `{"code":5,"msg":"系统错误！","data":null}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			server := gin.New()
			NewArticleHandler(tc.mock(ctrl), &logger.NopLogger{}).RegisterArticlesRouters(server)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/pub/list"+tc.query, nil)
			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, req)

			assert.Equal(t, http.StatusOK, resp.Code)
			assert.Equal(t, tc.expectBody, resp.Body.String())
		})
	}
}

func TestArticleHandlerDetailRejectsAnotherUsersDraft(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	svc := svcmocks.NewMockArticleService(ctrl)
	svc.EXPECT().GetById(gomock.Any(), int64(7)).Return(domain.Article{
		Id: 7, Title: "私有草稿", Content: "不应泄露", Author: domain.Author{Id: 2},
	}, nil)

	server := gin.New()
	server.Use(func(ctx *gin.Context) {
		ctx.Set("claims", &ijwt.UserClaims{Uid: 1})
	})
	NewArticleHandler(svc, &logger.NopLogger{}).RegisterArticlesRouters(server)
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
	NewArticleHandler(svcmocks.NewMockArticleService(ctrl), &logger.NopLogger{}).RegisterArticlesRouters(server)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles/list?page=1&pageSize=51", nil)
	resp := httptest.NewRecorder()
	server.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, `{"code":4,"msg":"pageSize 不能超过 50","data":null}`, resp.Body.String())
}
