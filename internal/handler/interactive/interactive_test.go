package interactive

import (
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

//go:generate mockgen -source=../../service/interactive.go -package=svcmocks -destination=../../service/mocks/interactive.mock.go
func TestInteractiveHandlerPublicBatchStatus(t *testing.T) {
	testCases := []struct {
		name     string
		body     string
		setup    func(*svcmocks.MockInteractiveService)
		wantCode int
		wantBody string
	}{
		{
			name:     "批量查询成功并补齐无记录文章的零值",
			body:     `{"ids":[2,3]}`,
			wantCode: http.StatusOK,
			setup: func(svc *svcmocks.MockInteractiveService) {
				svc.EXPECT().
					BatchGet(gomock.Any(), "article", []int64{2, 3}).
					Return(map[int64]domain.Interactive{
						2: {Biz: "article", BizId: 2, LikeCnt: 3, ReadCnt: 4, CollectCnt: 5},
					}, nil)
			},
			wantBody: `{"code":0,"msg":"success","data":{"2":{"likeCount":3,"readCount":4,"collectCount":5},"3":{"likeCount":0,"readCount":0,"collectCount":0}}}`,
		},
		{
			name:     "缺少文章ID",
			body:     `{}`,
			wantCode: http.StatusBadRequest,
			wantBody: `{"code":4,"msg":"文章 id 列表参数错误","data":null}`,
		},
		{
			name:     "文章ID均无效",
			body:     `{"ids":[0,-1]}`,
			wantCode: http.StatusBadRequest,
			wantBody: `{"code":4,"msg":"文章 id 列表参数错误","data":null}`,
		},
		{
			name: "服务查询失败", body: `{"ids":[2]}`,
			wantCode: http.StatusInternalServerError,
			setup: func(svc *svcmocks.MockInteractiveService) {
				svc.EXPECT().
					BatchGet(gomock.Any(), "article", []int64{2}).
					Return(nil, errors.New("database unavailable"))
			},
			wantBody: `{"code":5,"msg":"查询互动数据失败","data":null}`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := svcmocks.NewMockInteractiveService(ctrl)
			if tc.setup != nil {
				tc.setup(svc)
			}
			server := gin.New()
			NewHandler(svc, &logger.NopLogger{}).Register(server)
			resp := httptest.NewRecorder()
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/pub/articles/interactions",
				strings.NewReader(tc.body),
			)
			req.Header.Set("Content-Type", "application/json")
			server.ServeHTTP(resp, req)
			assert.Equal(t, tc.wantCode, resp.Code)
			assert.JSONEq(t, tc.wantBody, resp.Body.String())
		})
	}
}

func newInteractiveServer(svc *svcmocks.MockInteractiveService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	server := gin.New()
	server.Use(func(ctx *gin.Context) {
		ctx.Set("claims", &ijwt.UserClaims{Uid: 1})
		ctx.Next()
	})
	NewHandler(svc, &logger.NopLogger{}).Register(server)
	return server
}

func TestInteractiveHandlerLike(t *testing.T) {
	testInteractiveMutation(
		t,
		"like",
		"点赞成功",
		"点赞失败",
		func(svc *svcmocks.MockInteractiveService, id int64, err error) {
			svc.EXPECT().Like(gomock.Any(), "article", id, int64(1)).Return(err)
		},
	)
}

func TestInteractiveHandlerUnlike(t *testing.T) {
	testInteractiveMutation(
		t,
		"unlike",
		"取消点赞成功",
		"取消点赞失败",
		func(svc *svcmocks.MockInteractiveService, id int64, err error) {
			svc.EXPECT().CancelLike(gomock.Any(), "article", id, int64(1)).Return(err)
		},
	)
}

func TestInteractiveHandlerCollect(t *testing.T) {
	testInteractiveMutation(
		t,
		"collect",
		"收藏成功",
		"收藏失败",
		func(svc *svcmocks.MockInteractiveService, id int64, err error) {
			svc.EXPECT().Collect(gomock.Any(), "article", id, int64(1)).Return(err)
		},
	)
}

func TestInteractiveHandlerRejectsUnavailableArticle(t *testing.T) {
	for _, action := range []string{"like", "collect"} {
		t.Run(action, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := svcmocks.NewMockInteractiveService(ctrl)
			if action == "like" {
				svc.EXPECT().
					Like(gomock.Any(), "article", int64(2), int64(1)).
					Return(service.ErrInteractiveTargetNotFound)
			} else {
				svc.EXPECT().Collect(gomock.Any(), "article", int64(2), int64(1)).Return(service.ErrInteractiveTargetNotFound)
			}
			resp := httptest.NewRecorder()
			path := "/api/v1/pub/articles/2/" + action
			newInteractiveServer(
				svc,
			).ServeHTTP(resp, httptest.NewRequest(http.MethodPost, path, nil))
			assert.Equal(t, http.StatusNotFound, resp.Code)
			assert.JSONEq(t, `{"code":4,"msg":"文章不存在或不可操作","data":null}`, resp.Body.String())
		})
	}
}

func TestInteractiveHandlerUncollect(t *testing.T) {
	testInteractiveMutation(
		t,
		"uncollect",
		"取消收藏成功",
		"取消收藏失败",
		func(svc *svcmocks.MockInteractiveService, id int64, err error) {
			svc.EXPECT().CancelCollect(gomock.Any(), "article", id, int64(1)).Return(err)
		},
	)
}

func testInteractiveMutation(
	t *testing.T,
	action, successMessage, failureMessage string,
	expectCall func(*svcmocks.MockInteractiveService, int64, error),
) {
	t.Helper()
	testCases := []struct {
		name       string
		id         string
		serviceErr error
		wantBody   string
	}{
		{name: "请求成功", id: "2", wantBody: `{"code":0,"msg":"` + successMessage + `","data":null}`},
		{name: "文章ID错误", id: "nope", wantBody: `{"code":4,"msg":"文章 id 参数错误","data":null}`},
		{
			name:       "服务异常",
			id:         "2",
			serviceErr: errors.New("database unavailable"),
			wantBody:   `{"code":5,"msg":"` + failureMessage + `","data":null}`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := svcmocks.NewMockInteractiveService(ctrl)
			if tc.id == "2" {
				expectCall(svc, 2, tc.serviceErr)
			}
			resp := httptest.NewRecorder()
			path := "/api/v1/pub/articles/" + tc.id + "/" + action
			newInteractiveServer(
				svc,
			).ServeHTTP(resp, httptest.NewRequest(http.MethodPost, path, nil))
			wantCode := http.StatusOK
			if tc.id != "2" {
				wantCode = http.StatusBadRequest
			} else if tc.serviceErr != nil {
				wantCode = http.StatusInternalServerError
			}
			assert.Equal(t, wantCode, resp.Code)
			assert.JSONEq(t, tc.wantBody, resp.Body.String())
		})
	}
}

func TestInteractiveHandlerUserInteractionStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := svcmocks.NewMockInteractiveService(ctrl)
	svc.EXPECT().Get(gomock.Any(), "article", int64(2), int64(1)).Return(domain.Interactive{
		Biz: "article", BizId: 2, LikeCnt: 3, Liked: true,
	}, nil)
	resp := httptest.NewRecorder()
	newInteractiveServer(
		svc,
	).ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/pub/articles/2/interactions/status", nil))
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.JSONEq(
		t,
		`{"code":0,"msg":"success","data":{"liked":true,"likeCount":3,"readCount":0,"collected":false,"collectCount":0}}`,
		resp.Body.String(),
	)
}

func TestInteractiveHandlerAnonymousInteractionStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := svcmocks.NewMockInteractiveService(ctrl)
	svc.EXPECT().Get(gomock.Any(), "article", int64(2), int64(0)).Return(domain.Interactive{
		Biz: "article", BizId: 2, LikeCnt: 3, ReadCnt: 4, CollectCnt: 5,
	}, nil)
	resp := httptest.NewRecorder()
	server := gin.New()
	NewHandler(svc, &logger.NopLogger{}).Register(server)
	server.ServeHTTP(
		resp,
		httptest.NewRequest(http.MethodGet, "/api/v1/pub/articles/2/interactions/status", nil),
	)
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.JSONEq(
		t,
		`{"code":0,"msg":"success","data":{"liked":false,"likeCount":3,"readCount":4,"collected":false,"collectCount":5}}`,
		resp.Body.String(),
	)
}

func TestInteractiveHandlerPublicInteractions(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := svcmocks.NewMockInteractiveService(ctrl)
	svc.EXPECT().Get(gomock.Any(), "article", int64(2), int64(0)).Return(domain.Interactive{
		Biz: "article", BizId: 2, LikeCnt: 3,
	}, nil)
	server := gin.New()
	NewHandler(svc, &logger.NopLogger{}).Register(server)
	resp := httptest.NewRecorder()
	server.ServeHTTP(
		resp,
		httptest.NewRequest(http.MethodGet, "/api/v1/pub/articles/2/interactions", nil),
	)
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.JSONEq(
		t,
		`{"code":0,"msg":"success","data":{"likeCount":3,"readCount":0,"collectCount":0}}`,
		resp.Body.String(),
	)
}

func TestInteractiveHandlerCollections(t *testing.T) {
	testCases := []struct {
		name       string
		path       string
		listErr    error
		countErr   error
		expectCode int
		expectBody string
	}{
		{
			name:       "查询成功",
			path:       "/api/v1/users/me/collections?page=2&pageSize=5",
			expectCode: http.StatusOK,
			expectBody: `{"code":0,"msg":"success","data":{"items":[{"id":2,"title":"收藏文章","abstract":"正文","authorId":3,"authorNickname":"云端旅人","status":2,"createdAt":0,"updatedAt":0}],"page":2,"pageSize":5,"total":6,"totalPages":2}}`,
		},
		{
			name:       "分页参数错误",
			path:       "/api/v1/users/me/collections?page=0&pageSize=5",
			expectCode: http.StatusBadRequest,
			expectBody: `{"code":4,"msg":"page 和 pageSize 必须大于 0","data":null}`,
		},
		{
			name:       "列表查询失败",
			path:       "/api/v1/users/me/collections?page=1&pageSize=5",
			listErr:    errors.New("database unavailable"),
			expectCode: http.StatusInternalServerError,
			expectBody: `{"code":5,"msg":"查询收藏失败","data":null}`,
		},
		{
			name:       "总数查询失败",
			path:       "/api/v1/users/me/collections?page=1&pageSize=5",
			countErr:   errors.New("database unavailable"),
			expectCode: http.StatusInternalServerError,
			expectBody: `{"code":5,"msg":"查询收藏失败","data":null}`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := svcmocks.NewMockInteractiveService(ctrl)
			if tc.name != "分页参数错误" {
				items := []domain.PublishArticle{
					{
						Id:      2,
						Title:   "收藏文章",
						Content: "正文",
						Status:  domain.ArticleStatusPublished,
						Author:  domain.Author{Id: 3, Nickname: "云端旅人"},
					},
				}
				offset := 0
				if tc.name == "查询成功" {
					offset = 5
				}
				svc.EXPECT().
					ListCollected(gomock.Any(), "article", int64(1), offset, 5).
					Return(items, tc.listErr)
				if tc.listErr == nil {
					svc.EXPECT().
						CountCollected(gomock.Any(), "article", int64(1)).
						Return(int64(6), tc.countErr)
				}
			}
			resp := httptest.NewRecorder()
			newInteractiveServer(
				svc,
			).ServeHTTP(resp, httptest.NewRequest(http.MethodGet, tc.path, nil))
			assert.Equal(t, tc.expectCode, resp.Code)
			assert.JSONEq(t, tc.expectBody, resp.Body.String())
		})
	}
}
