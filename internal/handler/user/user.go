package user

import (
	"errors"
	"followingfeed/internal/domain"
	"followingfeed/internal/handler"
	ijwt "followingfeed/internal/handler/jwt"
	"followingfeed/internal/service"
	"followingfeed/pkg/ginx"
	"followingfeed/pkg/logger"
	"net/http"

	regexp "github.com/dlclark/regexp2"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/redis/go-redis/v9"
)

const (
	emailRegexPattern    = "^\\w+([-+.]\\w+)*@\\w+([-.]\\w+)*\\.\\w+([-.]\\w+)*$"
	passwordRegexPattern = `^(?=.*[A-Za-z])(?=.*\d)(?=.*[$@$!%*#?&])[A-Za-z\d$@$!%*#?&]{8,}$`
)

type UserHandler struct {
	emailRegex    *regexp.Regexp
	passwordRegex *regexp.Regexp

	svc service.UserService
	cmd redis.Cmdable
	l   logger.LoggerV1
	ijwt.JWTHandler
}

func NewUserHandler(
	svc service.UserService,
	cmd redis.Cmdable,
	l logger.LoggerV1,
	jwtHandler ijwt.JWTHandler,
) *UserHandler {
	emailExp := regexp.MustCompile(emailRegexPattern, regexp.None)
	passwordExp := regexp.MustCompile(passwordRegexPattern, regexp.None)
	return &UserHandler{
		emailRegex:    emailExp,
		passwordRegex: passwordExp,
		svc:           svc,
		cmd:           cmd,
		l:             l,
		JWTHandler:    jwtHandler,
	}
}

func (uh *UserHandler) RegisterUsersRouters(server *gin.Engine) {
	group := server.Group("/api/v1/users")
	group.POST("/signup", ginx.WrapperReq[SignUpReq](uh.l, uh.SignUp))
	group.POST("/login", ginx.WrapperReq[LoginReq](uh.l, uh.Login))
	group.GET("/profile", uh.Profile)
	group.POST("/refreshToken", uh.RefreshToken)
	group.POST("/logout", uh.Logout)
}

func (uh *UserHandler) Logout(ctx *gin.Context) {
	err := uh.ClearToken(ctx)
	if err != nil {
		ctx.JSON(http.StatusOK, handler.Result{
			Code: 5,
			Msg:  "退出登录失败！",
		})
		return
	}

	ctx.JSON(http.StatusOK, handler.Result{
		Code: 0,
		Msg:  "退出登录成功～",
	})
}

func (uh *UserHandler) RefreshToken(ctx *gin.Context) {
	refreshToken := uh.ExtractToken(ctx)

	rc, err := uh.ParseRefreshToken(refreshToken)
	if err != nil {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	//查看ssid是否有效
	err = uh.CheckSession(ctx, rc.Ssid)
	if err != nil {
		//Redis问题或已退出登录
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	//更新token
	err = uh.SetJWTToken(ctx, rc.Uid, rc.Ssid)
	if err != nil {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	ctx.JSON(http.StatusOK, handler.Result{
		Code: 0,
		Msg:  "token刷新成功～",
	})
}

func (uh *UserHandler) Profile(ctx *gin.Context) {
	v, ok := ctx.Get("claims")
	if !ok {
		ctx.String(http.StatusOK, "系统错误!")
		return
	}

	claims, ok := v.(*ijwt.UserClaims)
	if !ok {
		ctx.String(http.StatusOK, "系统错误!")
		return
	}

	res, err := uh.svc.Profile(ctx, claims.Uid)
	if err != nil {
		ctx.String(http.StatusInternalServerError, "获取用户信息失败！")
		return
	}

	data := UserVO{
		Id:        res.Id,
		Nickname:  res.Nickname,
		Email:     res.Email,
		CreatedAt: res.CreatedAt,
		UpdatedAt: res.UpdatedAt,
	}

	ctx.JSON(http.StatusOK, handler.Result{
		Code: 0,
		Data: data,
	})
}

func (uh *UserHandler) Login(ctx *gin.Context, req LoginReq) (handler.Result, error) {
	u, err := uh.svc.Login(ctx, req.Email, req.Password)

	if errors.Is(err, service.ErrInvalidUserPassword) {
		return handler.Result{
			Code: 5,
			Msg:  "账号/邮箱或密码错误！",
		}, nil
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return handler.Result{
			Code: 5,
			Msg:  "用户不存在！",
		}, nil
	}

	if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "系统错误！",
		}, nil
	}

	er := uh.SetLoginToken(ctx, u.Id)
	if er != nil {
		return handler.Result{
			Code: 5,
			Msg:  "系统错误！",
		}, nil
	}

	return handler.Result{
		Code: 0,
		Msg:  "登录成功～",
	}, nil
}

func (uh *UserHandler) SignUp(ctx *gin.Context, req SignUpReq) (handler.Result, error) {
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
		}, err
	}
	//两次输入的密码不一致
	if req.ConfirmPassword != req.Password {
		return handler.Result{
			Code: 4,
			Msg:  "两次输入的密码不一致!",
		}, err
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
			Code: 5,
			Msg:  "密码必须大于8位，包含数字、特殊字符!",
		}, err
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
		}, er
	}

	if errors.Is(er, service.ErrUserDuplicated) {
		return handler.Result{
			Code: 5,
			Msg:  "邮箱重复，请换一个!",
		}, er
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
