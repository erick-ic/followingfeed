package user

import (
	"errors"
	"fmt"
	"followingfeed/internal/domain"
	"followingfeed/internal/handler"
	ijwt "followingfeed/internal/handler/jwt"
	"followingfeed/internal/service"
	"followingfeed/pkg/ginx"
	"followingfeed/pkg/logger"
	"net/http"
	"strconv"
	"strings"

	regexp "github.com/dlclark/regexp2"
	"github.com/gin-gonic/gin"
)

const (
	emailRegexPattern    = "^\\w+([-+.]\\w+)*@\\w+([-.]\\w+)*\\.\\w+([-.]\\w+)*$"
	passwordRegexPattern = `^(?=.*[A-Za-z])(?=.*\d)(?=.*[$@$!%*#?&])[A-Za-z\d$@$!%*#?&]{8,}$`
	maxEmailBytes        = 254
	maxPasswordBytes     = 72
)

type UserHandler struct {
	emailRegex    *regexp.Regexp
	passwordRegex *regexp.Regexp

	svc              service.UserService
	publicProfileSvc service.PublicProfileService
	myProfileSvc     service.MyProfileService
	l                logger.LoggerV1
	ijwt.JWTHandler
}

func NewUserHandler(
	svc service.UserService,
	publicProfileSvc service.PublicProfileService,
	myProfileSvc service.MyProfileService,
	l logger.LoggerV1,
	jwtHandler ijwt.JWTHandler,
) *UserHandler {
	emailExp := regexp.MustCompile(emailRegexPattern, regexp.None)
	passwordExp := regexp.MustCompile(passwordRegexPattern, regexp.None)
	return &UserHandler{
		emailRegex:       emailExp,
		passwordRegex:    passwordExp,
		svc:              svc,
		publicProfileSvc: publicProfileSvc,
		myProfileSvc:     myProfileSvc,
		l:                l,
		JWTHandler:       jwtHandler,
	}
}

func (uh *UserHandler) RegisterUsersRouters(server *gin.Engine) {
	group := server.Group("/api/v1/users")
	group.POST("/signup", ginx.WrapperReq[SignUpReq](uh.l, uh.SignUp))
	group.POST("/login", ginx.WrapperReq[LoginReq](uh.l, uh.Login))

	// profile、logout 使用访问令牌；refreshToken 从 HttpOnly Cookie 读取刷新令牌。
	group.GET("/profile", ginx.WrapperWithToken[*ijwt.UserClaims](uh.l, uh.Profile))
	group.POST("/refreshToken", uh.RefreshToken)
	group.POST("/logout", ginx.WrapperWithToken[*ijwt.UserClaims](uh.l, uh.Logout))

	// GET /pub/users/:id/profile：公开；返回作者及关注、粉丝、已发布文章统计。
	server.GET("/api/v1/pub/users/:id/profile", ginx.Wrapper(uh.l, uh.PublicProfile))
}

func (uh *UserHandler) PublicProfile(ctx *gin.Context) (handler.Result, error) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return handler.Result{
			Code: 4,
			Msg:  "用户 id 参数错误",
		}, nil
	}
	profile, err := uh.publicProfileSvc.Get(ctx.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			return handler.Result{Code: 4, Msg: "用户不存在", HTTPStatus: http.StatusNotFound}, nil
		}
		return handler.Result{
				Code: 5,
				Msg:  "查询用户公开资料失败",
			},
			fmt.Errorf("查询公开用户资料失败，用户 ID=%d：%w", id, err)
	}
	return handler.Result{
		Code: 0,
		Msg:  "success",
		Data: PublicUserVO{
			Id:             profile.User.Id,
			Nickname:       profile.User.Nickname,
			CreatedAt:      profile.User.CreatedAt,
			FollowingCount: profile.FollowingCount,
			FollowersCount: profile.FollowersCount,
			ArticleCount:   profile.ArticleCount,
		}}, nil
}

func (uh *UserHandler) Logout(ctx *gin.Context, claims *ijwt.UserClaims) (handler.Result, error) {
	err := uh.ClearToken(ctx, claims.Ssid)
	if err != nil {
		ctx.Set("security_event", "auth.logout_failed")
		return handler.Result{
			Code: 5,
			Msg:  "退出登录失败！",
		}, fmt.Errorf("清除登录会话失败，用户 ID=%d：%w", claims.Uid, err)
	}
	ctx.Set("security_event", "auth.logout_succeeded")

	return handler.Result{
		Code: 0,
		Msg:  "退出登录成功～",
	}, nil
}

func (uh *UserHandler) RefreshToken(ctx *gin.Context) {
	refreshToken := uh.ExtractRefreshToken(ctx)

	rc, err := uh.ParseRefreshToken(refreshToken)
	if err != nil {
		uh.ClearRefreshToken(ctx)
		ctx.Set("security_event", "auth.token_refresh_failed")
		ctx.Set("auth_failure_reason", "invalid_token")
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	//查看ssid是否有效
	err = uh.CheckSession(ctx.Request.Context(), rc.Ssid)
	if err != nil {
		if errors.Is(err, ijwt.ErrSessionRevoked) {
			uh.ClearRefreshToken(ctx)
			ctx.Set("security_event", "auth.token_refresh_failed")
			ctx.Set("auth_failure_reason", "revoked")
			ctx.Set("auth_user_id", rc.Uid)
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		ctx.Set("security_event", "auth.token_refresh_failed")
		ctx.Set("auth_failure_reason", "dependency_error")
		ctx.Set("auth_user_id", rc.Uid)
		_ = ctx.Error(fmt.Errorf("检查刷新令牌会话失败：%w", err))
		ctx.JSON(http.StatusServiceUnavailable, handler.Result{Code: 5, Msg: "认证服务暂时不可用"})
		return
	}

	//更新token
	err = uh.SetJWTToken(ctx, rc.Uid, rc.Ssid)
	if err != nil {
		ctx.Set("security_event", "auth.token_refresh_failed")
		ctx.Set("auth_failure_reason", "token_issue")
		ctx.Set("auth_user_id", rc.Uid)
		_ = ctx.Error(fmt.Errorf("签发新的访问令牌失败：%w", err))
		ctx.JSON(http.StatusInternalServerError, handler.Result{Code: 5, Msg: "系统错误！"})
		return
	}
	ctx.Set("security_event", "auth.token_refresh_succeeded")
	ctx.Set("auth_user_id", rc.Uid)
	ctx.JSON(http.StatusOK, handler.Result{
		Code: 0,
		Msg:  "token刷新成功～",
	})
}

func (uh *UserHandler) Profile(ctx *gin.Context, claims *ijwt.UserClaims) (handler.Result, error) {
	profile, err := uh.myProfileSvc.Get(ctx.Request.Context(), claims.Uid)
	if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "获取用户信息失败！",
		}, fmt.Errorf("查询当前用户资料失败，用户 ID=%d：%w", claims.Uid, err)
	}

	data := UserVO{
		Id:                  profile.User.Id,
		Nickname:            profile.User.Nickname,
		Email:               profile.User.Email,
		CreatedAt:           profile.User.CreatedAt,
		UpdatedAt:           profile.User.UpdatedAt,
		ArticleLikeCount:    profile.ArticleLikeCount,
		ArticleReadCount:    profile.ArticleReadCount,
		ArticleCollectCount: profile.ArticleCollectCount,
	}

	return handler.Result{
		Code: 0,
		Data: data,
	}, nil
}

func (uh *UserHandler) Login(ctx *gin.Context, req LoginReq) (handler.Result, error) {
	u, err := uh.svc.Login(ctx.Request.Context(), req.Email, req.Password)

	if errors.Is(err, service.ErrInvalidUserPassword) {
		ctx.Set("security_event", "auth.login_failed")
		ctx.Set("auth_failure_reason", "invalid_credentials")
		return handler.Result{
			Code:       4,
			Msg:        "账号/邮箱或密码错误！",
			HTTPStatus: http.StatusUnauthorized,
		}, nil
	}

	if err != nil {
		ctx.Set("security_event", "auth.login_failed")
		ctx.Set("auth_failure_reason", "dependency_error")
		return handler.Result{
			Code: 5,
			Msg:  "系统错误！",
		}, fmt.Errorf("用户登录失败：%w", err)
	}

	er := uh.SetLoginToken(ctx, u.Id)
	if er != nil {
		ctx.Set("security_event", "auth.login_failed")
		ctx.Set("auth_failure_reason", "token_issue")
		ctx.Set("auth_user_id", u.Id)
		return handler.Result{
			Code: 5,
			Msg:  "系统错误！",
		}, fmt.Errorf("设置登录令牌失败，用户 ID=%d：%w", u.Id, er)
	}
	ctx.Set("security_event", "auth.login_succeeded")
	ctx.Set("auth_user_id", u.Id)

	return handler.Result{
		Code: 0,
		Msg:  "登录成功～",
	}, nil
}

func (uh *UserHandler) SignUp(ctx *gin.Context, req SignUpReq) (handler.Result, error) {
	req.Email = strings.TrimSpace(req.Email)
	if len(req.Email) > maxEmailBytes {
		return handler.Result{Code: 4, Msg: "邮箱长度不能超过 254 个字符!"}, nil
	}
	if len(req.Password) > maxPasswordBytes || len(req.ConfirmPassword) > maxPasswordBytes {
		return handler.Result{Code: 4, Msg: "密码长度不能超过 72 个字符!"}, nil
	}
	isEmail, err := uh.emailRegex.MatchString(req.Email)
	//邮箱正则匹配失败
	if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "系统错误!",
		}, err
	}
	//邮箱格式校验不匹配
	if !isEmail {
		return handler.Result{
			Code: 4,
			Msg:  "邮箱格式错误!",
		}, nil
	}
	//两次输入的密码不一致
	if req.ConfirmPassword != req.Password {
		return handler.Result{
			Code: 4,
			Msg:  "两次输入的密码不一致!",
		}, nil
	}
	//密码正则匹配失败
	isPassword, err := uh.passwordRegex.MatchString(req.Password)
	if err != nil {
		//写入日志
		return handler.Result{
			Code: 5,
			Msg:  "系统错误!",
		}, err
	}
	//密码格式校验不匹配
	if !isPassword {
		return handler.Result{
			Code: 4,
			Msg:  "密码必须大于8位，包含数字、特殊字符!",
		}, nil
	}

	er := uh.svc.Create(ctx.Request.Context(), domain.User{
		Nickname: req.Nickname,
		Email:    req.Email,
		Password: req.Password,
	})

	if errors.Is(er, service.ErrInvalidNickname) {
		return handler.Result{
			Code: 4,
			Msg:  "昵称需为2至6位中文、英文字母或数字!",
		}, nil
	}

	if errors.Is(er, service.ErrUserDuplicated) {
		return handler.Result{
			Code:       4,
			Msg:        "邮箱重复，请换一个!",
			HTTPStatus: http.StatusConflict,
		}, nil
	}

	if er != nil {
		return handler.Result{
			Code: 5,
			Msg:  "系统错误!",
		}, er
	}

	return handler.Result{
		Code: 0,
		Msg:  "注册成功～",
	}, nil
}
