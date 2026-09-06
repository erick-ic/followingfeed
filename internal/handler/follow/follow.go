package follow

import (
	"errors"
	"followingfeed/internal/handler"
	ijwt "followingfeed/internal/handler/jwt"
	"followingfeed/internal/service"
	"followingfeed/pkg/ginx"
	"followingfeed/pkg/logger"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc service.FollowService
	l   logger.LoggerV1
}

func NewHandler(svc service.FollowService, l logger.LoggerV1) *Handler {
	return &Handler{
		svc: svc,
		l:   l,
	}
}

func (h *Handler) RegisterRouters(server *gin.Engine) {
	group := server.Group("/api/v1/users")
	// 路径 id 均为目标用户 ID；以下接口需要 JWT。
	group.POST("/:id/follow", ginx.WrapperWithToken[*ijwt.UserClaims](h.l, h.Follow))
	group.POST("/:id/unfollow", ginx.WrapperWithToken[*ijwt.UserClaims](h.l, h.Unfollow))
	group.GET("/:id/following/status", ginx.WrapperWithToken[*ijwt.UserClaims](h.l, h.Status))

	// 关注/粉丝列表 Query：page、pageSize（1..50），仅本人可查看。
	group.GET(
		"/:id/following",
		ginx.WrapperReqWithToken[ListReq, *ijwt.UserClaims](h.l, h.ListFollowing),
	)
	group.GET(
		"/:id/followers",
		ginx.WrapperReqWithToken[ListReq, *ijwt.UserClaims](h.l, h.ListFollowers),
	)
}

// ListFollowers 分页查询当前登录用户的粉丝列表。
func (h *Handler) ListFollowers(
	ctx *gin.Context,
	req ListReq,
	claims *ijwt.UserClaims,
) (handler.Result, error) {
	target, ok := targetID(ctx)
	if !ok {
		return handler.Result{
			Code: 4,
			Msg:  "用户 id 参数错误",
		}, nil
	}
	if claims.Uid != target {
		return handler.Result{
			Code:       4,
			Msg:        "无权查看其他用户的粉丝列表",
			HTTPStatus: http.StatusForbidden,
		}, nil
	}
	if errMsg := validatePagination(req); errMsg != "" {
		return handler.Result{
			Code: 4,
			Msg:  errMsg,
		}, nil
	}

	offset := (req.Page - 1) * req.PageSize
	items, err := h.svc.ListFollowersPage(ctx, target, offset, req.PageSize)
	if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "查询粉丝列表失败",
		}, err
	}

	total, err := h.svc.CountFollowers(ctx, target)
	if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "查询粉丝列表失败",
		}, err
	}

	data := make([]FollowVO, len(items))
	for i, item := range items {
		data[i] = newFollowVO(item)
	}
	return handler.Result{
		Code: 0,
		Msg:  "success",
		Data: handler.NewPageResult(data, req.Page, req.PageSize, total),
	}, nil
}

// ListFollowing 分页查询当前登录用户的关注列表。
func (h *Handler) ListFollowing(
	ctx *gin.Context,
	req ListReq,
	claims *ijwt.UserClaims,
) (handler.Result, error) {
	target, ok := targetID(ctx)
	if !ok {
		return handler.Result{
			Code: 4,
			Msg:  "用户 id 参数错误",
		}, nil
	}
	if claims.Uid != target {
		return handler.Result{
			Code:       4,
			Msg:        "无权查看其他用户的关注列表",
			HTTPStatus: http.StatusForbidden,
		}, nil
	}
	if errMsg := validatePagination(req); errMsg != "" {
		return handler.Result{
			Code: 4,
			Msg:  errMsg,
		}, nil
	}

	offset := (req.Page - 1) * req.PageSize
	items, err := h.svc.ListFollowingPage(ctx, target, offset, req.PageSize)
	if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "查询关注列表失败",
		}, err
	}

	total, err := h.svc.CountFollowing(ctx, target)
	if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "查询关注列表失败",
		}, err
	}

	data := make([]FollowVO, len(items))
	for i, item := range items {
		data[i] = newFollowVO(item)
	}
	return handler.Result{
		Code: 0,
		Msg:  "success",
		Data: handler.NewPageResult(data, req.Page, req.PageSize, total),
	}, nil
}

// Status 查询当前登录用户是否已关注路径参数指定的用户。
func (h *Handler) Status(ctx *gin.Context, claims *ijwt.UserClaims) (handler.Result, error) {
	target, ok := targetID(ctx)
	if !ok {
		return handler.Result{
			Code: 4,
			Msg:  "用户 id 参数错误",
		}, nil
	}
	following, err := h.svc.IsFollowing(ctx, claims.Uid, target)
	if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "查询关注状态失败",
		}, err
	}
	return handler.Result{
		Code: 0,
		Msg:  "success",
		Data: map[string]bool{"following": following},
	}, nil
}

// Unfollow 让当前登录用户取消关注路径参数指定的用户。
func (h *Handler) Unfollow(ctx *gin.Context, claims *ijwt.UserClaims) (handler.Result, error) {
	target, ok := targetID(ctx)
	if !ok {
		return handler.Result{
			Code: 4,
			Msg:  "用户 id 参数错误",
		}, nil
	}
	if err := h.svc.UnFollow(ctx, claims.Uid, target); err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "取消关注失败",
		}, err
	}
	return handler.Result{
		Code: 0,
		Msg:  "取消关注成功",
	}, nil
}

// Follow 让当前登录用户关注路径参数指定的用户。
func (h *Handler) Follow(ctx *gin.Context, claims *ijwt.UserClaims) (handler.Result, error) {
	target, ok := targetID(ctx)
	if !ok {
		return handler.Result{
			Code: 4,
			Msg:  "用户 id 参数错误",
		}, nil
	}
	if err := h.svc.Follow(ctx, claims.Uid, target); errors.Is(err, service.ErrCannotFollowSelf) {
		return handler.Result{
			Code: 4,
			Msg:  "不能关注自己",
		}, nil
	} else if errors.Is(err, service.ErrFollowDuplicated) {
		return handler.Result{
			Code:       4,
			Msg:        "已经关注该用户",
			HTTPStatus: http.StatusConflict,
		}, nil
	} else if errors.Is(err, service.ErrTargetUserNotFound) {
		return handler.Result{
			Code:       4,
			Msg:        "目标用户不存在",
			HTTPStatus: http.StatusNotFound,
		}, nil
	} else if err != nil {
		return handler.Result{
			Code: 4,
			Msg:  "关注失败",
		}, err
	}
	return handler.Result{
		Code: 0,
		Msg:  "关注成功",
	}, nil
}

// validatePagination 校验列表分页参数范围。
func validatePagination(req ListReq) string {
	return handler.ValidatePagination(req.Page, req.PageSize)
}

func targetID(ctx *gin.Context) (int64, bool) {
	target, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	return target, err == nil && target > 0
}
