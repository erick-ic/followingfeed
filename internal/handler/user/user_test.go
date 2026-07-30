package user

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"followingfeed/internal/domain"
	ijwt "followingfeed/internal/handler/jwt"
	jwtmocks "followingfeed/internal/handler/jwt/mocks"
	"followingfeed/internal/service"
	svcmocks "followingfeed/internal/service/mocks"
	"followingfeed/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

//go:generate mockgen -source=../service/user.go -package=svcmocks -destination=../service/mocks/user.mock.go
//go:generate mockgen -source=../jwt/types.go -package=jwtmocks -destination=../jwt/mocks/jwt.mock.go
func TestUserHandlerSignUp(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testCases := []struct {
		name       string
		reqBody    string
		mock       func(*gomock.Controller) service.UserService
		expectCode int
		expectBody string
	}{
		{
			name:    "注册成功",
			reqBody: `{"nickname":"云端旅人","email":"alice@example.com","password":"Passw0rd!","confirm_password":"Passw0rd!"}`,
			mock: func(ctrl *gomock.Controller) service.UserService {
				svc := svcmocks.NewMockUserService(ctrl)
				svc.EXPECT().Create(gomock.Any(), domain.User{
					Nickname: "云端旅人",
					Email:    "alice@example.com",
					Password: "Passw0rd!"},
				).Return(nil)
				return svc
			},
			expectCode: http.StatusOK,
			expectBody: `{"code":0,"msg":"注册成功～","data":null}`,
		},
		{
			name:       "邮箱格式错误",
			reqBody:    `{"nickname":"云端旅人","email":"alice","password":"Passw0rd!","confirm_password":"Passw0rd!"}`,
			mock:       func(ctrl *gomock.Controller) service.UserService { return svcmocks.NewMockUserService(ctrl) },
			expectCode: http.StatusOK,
			expectBody: `{"code":4,"msg":"邮箱格式错误!","data":null}`,
		},
		{
			name:       "两次密码不一致",
			reqBody:    `{"nickname":"云端旅人","email":"alice@example.com","password":"Passw0rd!","confirm_password":"Different1!"}`,
			mock:       func(ctrl *gomock.Controller) service.UserService { return svcmocks.NewMockUserService(ctrl) },
			expectCode: http.StatusOK,
			expectBody: `{"code":4,"msg":"两次输入的密码不一致!","data":null}`,
		},
		{
			name:       "密码格式错误",
			reqBody:    `{"nickname":"云端旅人","email":"alice@example.com","password":"12345678","confirm_password":"12345678"}`,
			mock:       func(ctrl *gomock.Controller) service.UserService { return svcmocks.NewMockUserService(ctrl) },
			expectCode: http.StatusOK,
			expectBody: `{"code":5,"msg":"密码必须大于8位，包含数字、特殊字符!","data":null}`,
		},
		{
			name:    "邮箱重复",
			reqBody: `{"nickname":"云端旅人","email":"alice@example.com","password":"Passw0rd!","confirm_password":"Passw0rd!"}`,
			mock: func(ctrl *gomock.Controller) service.UserService {
				svc := svcmocks.NewMockUserService(ctrl)
				svc.EXPECT().Create(gomock.Any(), domain.User{
					Nickname: "云端旅人",
					Email:    "alice@example.com",
					Password: "Passw0rd!"},
				).Return(service.ErrUserDuplicated)
				return svc
			},
			expectCode: http.StatusOK,
			expectBody: `{"code":5,"msg":"邮箱重复，请换一个!","data":null}`,
		},
		{
			name:    "系统异常",
			reqBody: `{"nickname":"云端旅人","email":"alice@example.com","password":"Passw0rd!","confirm_password":"Passw0rd!"}`,
			mock: func(ctrl *gomock.Controller) service.UserService {
				svc := svcmocks.NewMockUserService(ctrl)
				svc.EXPECT().Create(gomock.Any(), domain.User{
					Nickname: "云端旅人",
					Email:    "alice@example.com",
					Password: "Passw0rd!"},
				).Return(errors.New("database unavailable"))
				return svc
			},
			expectCode: http.StatusOK,
			expectBody: `{"code":5,"msg":"系统错误!","data":null}`,
		},
		{
			name:    "昵称格式错误",
			reqBody: `{"nickname":"云","email":"alice@example.com","password":"Passw0rd!","confirm_password":"Passw0rd!"}`,
			mock: func(ctrl *gomock.Controller) service.UserService {
				svc := svcmocks.NewMockUserService(ctrl)
				svc.EXPECT().Create(gomock.Any(), domain.User{
					Nickname: "云",
					Email:    "alice@example.com",
					Password: "Passw0rd!",
				}).Return(service.ErrInvalidNickname)
				return svc
			},
			expectCode: http.StatusOK,
			expectBody: `{"code":4,"msg":"昵称需为2至6位中文、英文字母或数字!","data":null}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := tc.mock(ctrl)
			server := gin.New()
			NewUserHandler(svc, nil, &logger.NopLogger{}, nil).RegisterUsersRouters(server)

			req := httptest.NewRequest(
				http.MethodPost, "/api/v1/users/signup",
				bytes.NewBufferString(tc.reqBody),
			)
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, req)

			assert.Equal(t, tc.expectCode, resp.Code)
			assert.Equal(t, tc.expectBody, resp.Body.String())
		})
	}
}

func TestUserHandlerLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testCases := []struct {
		name       string
		serviceErr error
		jwtErr     error
		expectCode int
		expectBody string
	}{
		{name: "登录成功", expectCode: http.StatusOK, expectBody: `{"code":0,"msg":"登录成功～","data":null}`},
		{name: "账号或密码错误", serviceErr: service.ErrInvalidUserPassword, expectCode: http.StatusOK, expectBody: `{"code":5,"msg":"账号/邮箱或密码错误！","data":null}`},
		{name: "用户不存在", serviceErr: gorm.ErrRecordNotFound, expectCode: http.StatusOK, expectBody: `{"code":5,"msg":"用户不存在！","data":null}`},
		{name: "系统异常", serviceErr: errors.New("database unavailable"), expectCode: http.StatusOK, expectBody: `{"code":5,"msg":"系统错误！","data":null}`},
		{name: "生成登录令牌失败", jwtErr: errors.New("redis unavailable"), expectCode: http.StatusOK, expectBody: `{"code":5,"msg":"系统错误！","data":null}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := svcmocks.NewMockUserService(ctrl)
			jwt := jwtmocks.NewMockJWTHandler(ctrl)
			u := domain.User{Id: 1, Email: "alice@example.com"}
			svc.EXPECT().Login(gomock.Any(), "alice@example.com", "Passw0rd!").Return(u, tc.serviceErr)
			if tc.serviceErr == nil {
				jwt.EXPECT().SetLoginToken(gomock.Any(), int64(1)).Return(tc.jwtErr)
			}

			server := gin.New()
			NewUserHandler(svc, nil, &logger.NopLogger{}, jwt).RegisterUsersRouters(server)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/users/login", bytes.NewBufferString(`{"email":"alice@example.com","password":"Passw0rd!"}`))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, req)

			assert.Equal(t, tc.expectCode, resp.Code)
			assert.Equal(t, tc.expectBody, resp.Body.String())
		})
	}
}

func TestUserHandlerLogout(t *testing.T) {
	testCases := []struct {
		name       string
		jwtErr     error
		expectBody string
	}{
		{name: "退出成功", expectBody: `{"code":0,"msg":"退出登录成功～","data":null}`},
		{name: "退出失败", jwtErr: errors.New("redis unavailable"), expectBody: `{"code":5,"msg":"退出登录失败！","data":null}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			jwt := jwtmocks.NewMockJWTHandler(ctrl)
			jwt.EXPECT().ClearToken(gomock.Any()).Return(tc.jwtErr)
			server := gin.New()
			NewUserHandler(nil, nil, &logger.NopLogger{}, jwt).RegisterUsersRouters(server)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/users/logout", nil)
			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, req)

			assert.Equal(t, http.StatusOK, resp.Code)
			assert.Equal(t, tc.expectBody, resp.Body.String())
		})
	}
}

func TestUserHandlerRefreshToken(t *testing.T) {
	const validTokenString = "valid-refresh-token"

	testCases := []struct {
		name        string
		token       string
		sessionErr  error
		setTokenErr error
		expectCode  int
		expectBody  string
	}{
		{name: "刷新成功", token: validTokenString, expectCode: http.StatusOK, expectBody: `{"code":0,"msg":"token刷新成功～","data":null}`},
		{name: "Token 无效", token: "invalid-token", expectCode: http.StatusUnauthorized},
		{name: "会话无效", token: validTokenString, sessionErr: errors.New("session revoked"), expectCode: http.StatusUnauthorized},
		{name: "生成新 Token 失败", token: validTokenString, setTokenErr: errors.New("redis unavailable"), expectCode: http.StatusUnauthorized},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			jwtHandler := jwtmocks.NewMockJWTHandler(ctrl)
			jwtHandler.EXPECT().ExtractToken(gomock.Any()).Return(tc.token)
			if tc.token == validTokenString {
				jwtHandler.EXPECT().ParseRefreshToken(validTokenString).Return(
					&ijwt.RefreshClaims{Uid: 1, Ssid: "ssid-1"}, nil,
				)
				jwtHandler.EXPECT().CheckSession(gomock.Any(), "ssid-1").Return(tc.sessionErr)
				if tc.sessionErr == nil {
					jwtHandler.EXPECT().SetJWTToken(gomock.Any(), int64(1), "ssid-1").Return(tc.setTokenErr)
				}
			} else {
				jwtHandler.EXPECT().ParseRefreshToken(tc.token).Return(nil, errors.New("invalid token"))
			}

			server := gin.New()
			NewUserHandler(nil, nil, &logger.NopLogger{}, jwtHandler).RegisterUsersRouters(server)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/users/refreshToken", nil)
			req.Header.Set("Authorization", "Bearer "+tc.token)
			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, req)

			assert.Equal(t, tc.expectCode, resp.Code)
			if tc.expectBody != "" {
				assert.Equal(t, tc.expectBody, resp.Body.String())
			}
		})
	}
}

func TestUserHandlerProfile(t *testing.T) {
	testCases := []struct {
		name       string
		serviceErr error
		expectCode int
		expectBody string
	}{
		{name: "获取成功", expectCode: http.StatusOK, expectBody: `{"code":0,"msg":"","data":{"id":1,"nickname":"云端旅人","email":"alice@example.com","created_at":100,"updated_at":200}}`},
		{name: "获取失败", serviceErr: errors.New("database unavailable"), expectCode: http.StatusInternalServerError, expectBody: "获取用户信息失败！"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := svcmocks.NewMockUserService(ctrl)
			svc.EXPECT().Profile(gomock.Any(), int64(1)).Return(domain.User{
				Id: 1, Nickname: "云端旅人", Email: "alice@example.com",
				CreatedAt: 100, UpdatedAt: 200,
			}, tc.serviceErr)

			server := gin.New()
			server.Use(func(ctx *gin.Context) {
				ctx.Set("claims", &ijwt.UserClaims{Uid: 1})
			})
			NewUserHandler(svc, nil, &logger.NopLogger{}, nil).RegisterUsersRouters(server)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/users/profile", nil)
			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, req)

			assert.Equal(t, tc.expectCode, resp.Code)
			assert.Equal(t, tc.expectBody, resp.Body.String())
		})
	}
}
