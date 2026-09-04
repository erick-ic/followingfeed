package follow

import (
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

//go:generate mockgen -source=../../service/follow.go -package=svcmocks -destination=../../service/mocks/follow.mock.go
func newFollowServer(svc service.FollowService, authenticated bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	server := gin.New()
	if authenticated {
		server.Use(func(ctx *gin.Context) {
			ctx.Set("claims", &ijwt.UserClaims{Uid: 1})
			ctx.Next()
		})
	}
	NewHandler(svc, &logger.NopLogger{}).RegisterRouters(server)
	return server
}

func TestFollowHandlerFollow(t *testing.T) {
	testCases := []struct {
		name       string
		path       string
		auth       bool
		mock       func(*gomock.Controller) service.FollowService
		expectCode int
		expectBody string
	}{
		{
			name: "关注成功",
			path: "/api/v1/users/2/follow",
			auth: true,
			mock: func(ctrl *gomock.Controller) service.FollowService {
				svc := svcmocks.NewMockFollowService(ctrl)
				svc.EXPECT().Follow(gomock.Any(), int64(1), int64(2)).Return(nil)
				return svc
			},
			expectCode: http.StatusOK,
			expectBody: `{"code":0,"msg":"关注成功","data":null}`,
		},
		{
			name: "不能关注自己",
			path: "/api/v1/users/1/follow",
			auth: true,
			mock: func(ctrl *gomock.Controller) service.FollowService {
				svc := svcmocks.NewMockFollowService(ctrl)
				svc.EXPECT().
					Follow(gomock.Any(), int64(1), int64(1)).
					Return(service.ErrCannotFollowSelf)
				return svc
			},
			expectCode: http.StatusBadRequest,
			expectBody: `{"code":4,"msg":"不能关注自己","data":null}`,
		},
		{
			name: "重复关注",
			path: "/api/v1/users/2/follow",
			auth: true,
			mock: func(ctrl *gomock.Controller) service.FollowService {
				svc := svcmocks.NewMockFollowService(ctrl)
				svc.EXPECT().
					Follow(gomock.Any(), int64(1), int64(2)).
					Return(service.ErrFollowDuplicated)
				return svc
			},
			expectCode: http.StatusConflict,
			expectBody: `{"code":4,"msg":"已经关注该用户","data":null}`,
		},
		{
			name: "目标用户不存在",
			path: "/api/v1/users/2/follow",
			auth: true,
			mock: func(ctrl *gomock.Controller) service.FollowService {
				svc := svcmocks.NewMockFollowService(ctrl)
				svc.EXPECT().
					Follow(gomock.Any(), int64(1), int64(2)).
					Return(service.ErrTargetUserNotFound)
				return svc
			},
			expectCode: http.StatusNotFound,
			expectBody: `{"code":4,"msg":"目标用户不存在","data":null}`,
		},
		{
			name: "服务异常",
			path: "/api/v1/users/2/follow",
			auth: true,
			mock: func(ctrl *gomock.Controller) service.FollowService {
				svc := svcmocks.NewMockFollowService(ctrl)
				svc.EXPECT().
					Follow(gomock.Any(), int64(1), int64(2)).
					Return(errors.New("database unavailable"))
				return svc
			},
			expectCode: http.StatusInternalServerError,
			expectBody: `{"code":4,"msg":"关注失败","data":null}`,
		},
		{
			name:       "用户ID错误",
			path:       "/api/v1/users/nope/follow",
			auth:       true,
			mock:       func(ctrl *gomock.Controller) service.FollowService { return svcmocks.NewMockFollowService(ctrl) },
			expectCode: http.StatusBadRequest,
			expectBody: `{"code":4,"msg":"用户 id 参数错误","data":null}`,
		},
		{
			name:       "未登录",
			path:       "/api/v1/users/2/follow",
			mock:       func(ctrl *gomock.Controller) service.FollowService { return svcmocks.NewMockFollowService(ctrl) },
			expectCode: http.StatusUnauthorized,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			resp := httptest.NewRecorder()
			newFollowServer(
				tc.mock(ctrl),
				tc.auth,
			).ServeHTTP(resp, httptest.NewRequest(http.MethodPost, tc.path, nil))
			assert.Equal(t, tc.expectCode, resp.Code)
			if tc.expectBody != "" {
				assert.JSONEq(t, tc.expectBody, resp.Body.String())
			}
		})
	}
}

func TestFollowHandlerUnfollow(t *testing.T) {
	testCases := []struct {
		name       string
		serviceErr error
		expectCode int
		expectBody string
	}{
		{
			name:       "取消成功",
			expectCode: http.StatusOK,
			expectBody: `{"code":0,"msg":"取消关注成功","data":null}`,
		},
		{
			name:       "取消失败",
			serviceErr: errors.New("database unavailable"),
			expectCode: http.StatusInternalServerError,
			expectBody: `{"code":5,"msg":"取消关注失败","data":null}`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := svcmocks.NewMockFollowService(ctrl)
			svc.EXPECT().UnFollow(gomock.Any(), int64(1), int64(2)).Return(tc.serviceErr)
			resp := httptest.NewRecorder()
			newFollowServer(
				svc,
				true,
			).ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/v1/users/2/unfollow", nil))
			assert.Equal(t, tc.expectCode, resp.Code)
			assert.JSONEq(t, tc.expectBody, resp.Body.String())
		})
	}
}

func TestFollowHandlerStatus(t *testing.T) {
	testCases := []struct {
		name       string
		following  bool
		serviceErr error
		expectCode int
		expectBody string
	}{
		{
			name:       "已关注",
			following:  true,
			expectCode: http.StatusOK,
			expectBody: `{"code":0,"msg":"success","data":{"following":true}}`,
		},
		{
			name:       "未关注",
			expectCode: http.StatusOK,
			expectBody: `{"code":0,"msg":"success","data":{"following":false}}`,
		},
		{
			name:       "查询失败",
			serviceErr: errors.New("database unavailable"),
			expectCode: http.StatusInternalServerError,
			expectBody: `{"code":5,"msg":"查询关注状态失败","data":null}`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := svcmocks.NewMockFollowService(ctrl)
			svc.EXPECT().
				IsFollowing(gomock.Any(), int64(1), int64(2)).
				Return(tc.following, tc.serviceErr)
			resp := httptest.NewRecorder()
			newFollowServer(
				svc,
				true,
			).ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/users/2/following/status", nil))
			assert.Equal(t, tc.expectCode, resp.Code)
			assert.JSONEq(t, tc.expectBody, resp.Body.String())
		})
	}
}

func TestFollowHandlerListFollowing(t *testing.T) {
	testFollowListEndpoint(t, true)
}

func TestFollowHandlerListFollowers(t *testing.T) {
	testFollowListEndpoint(t, false)
}

func testFollowListEndpoint(t *testing.T, following bool) {
	path := "/api/v1/users/1/followers?page=2&pageSize=5"
	if following {
		path = "/api/v1/users/1/following?page=2&pageSize=5"
	}
	testCases := []struct {
		name       string
		listErr    error
		countErr   error
		expectCode int
		expectBody string
	}{
		{
			name:       "查询成功",
			expectCode: http.StatusOK,
			expectBody: `{"code":0,"msg":"success","data":{"items":[{"id":3,"followerId":1,"followingId":2,"nickname":"云端旅人","createdAt":100,"updatedAt":200}],"page":2,"pageSize":5,"total":6,"totalPages":2}}`,
		},
		{
			name:       "列表查询失败",
			listErr:    errors.New("database unavailable"),
			expectCode: http.StatusInternalServerError,
			expectBody: `{"code":5,"msg":"查询关注列表失败","data":null}`,
		},
		{
			name:       "总数查询失败",
			countErr:   errors.New("database unavailable"),
			expectCode: http.StatusInternalServerError,
			expectBody: `{"code":5,"msg":"查询关注列表失败","data":null}`,
		},
	}
	for _, tc := range testCases {
		if !following && (tc.listErr != nil || tc.countErr != nil) {
			tc.expectBody = `{"code":5,"msg":"查询粉丝列表失败","data":null}`
		}
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := svcmocks.NewMockFollowService(ctrl)
			items := []domain.Follow{
				{
					Id:          3,
					FollowerId:  1,
					FollowingId: 2,
					Nickname:    "云端旅人",
					CreatedAt:   100,
					UpdatedAt:   200,
				},
			}
			if following {
				svc.EXPECT().
					ListFollowingPage(gomock.Any(), int64(1), 5, 5).
					Return(items, tc.listErr)
				if tc.listErr == nil {
					svc.EXPECT().
						CountFollowing(gomock.Any(), int64(1)).
						Return(int64(6), tc.countErr)
				}
			} else {
				svc.EXPECT().ListFollowersPage(gomock.Any(), int64(1), 5, 5).Return(items, tc.listErr)
				if tc.listErr == nil {
					svc.EXPECT().CountFollowers(gomock.Any(), int64(1)).Return(int64(6), tc.countErr)
				}
			}
			resp := httptest.NewRecorder()
			newFollowServer(
				svc,
				true,
			).ServeHTTP(resp, httptest.NewRequest(http.MethodGet, path, nil))
			assert.Equal(t, tc.expectCode, resp.Code)
			assert.JSONEq(t, tc.expectBody, resp.Body.String())
		})
	}
}
