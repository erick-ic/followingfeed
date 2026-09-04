package interactive

import (
	"errors"
	"followingfeed/internal/domain"
	"followingfeed/internal/handler"
	ijwt "followingfeed/internal/handler/jwt"
	"followingfeed/internal/service"
	"followingfeed/pkg/ginx"
	"followingfeed/pkg/logger"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

const articleBiz = "article"

type Handler struct {
	svc service.InteractiveService
	l   logger.LoggerV1
}

// NewHandler 创建互动处理器，并注入互动服务和日志组件。
func NewHandler(svc service.InteractiveService, l logger.LoggerV1) *Handler {
	return &Handler{
		svc: svc,
		l:   l,
	}
}

// Register 注册文章互动和当前用户收藏列表路由。
func (h *Handler) Register(server *gin.Engine) {
	group := server.Group("/api/v1/pub/articles")
	group.POST("/interactions", ginx.WrapperReq[BatchInteractionReq](h.l, h.PublicBatchStatus))
	group.GET("/:id/interactions", ginx.Wrapper(h.l, h.PublicInteractions))
	group.GET("/:id/interactions/status", ginx.Wrapper(h.l, h.UserInteractionStatus))
	group.POST("/:id/like", ginx.WrapperWithToken[*ijwt.UserClaims](h.l, h.Like))
	group.POST("/:id/unlike", ginx.WrapperWithToken[*ijwt.UserClaims](h.l, h.CancelLike))
	group.POST("/:id/collect", ginx.WrapperWithToken[*ijwt.UserClaims](h.l, h.Collect))
	group.POST("/:id/uncollect", ginx.WrapperWithToken[*ijwt.UserClaims](h.l, h.CancelCollect))
	server.GET(
		"/api/v1/users/me/collections",
		ginx.WrapperReqWithToken[ListReq, *ijwt.UserClaims](h.l, h.Collections),
	)
}

// Collections 分页查询当前登录用户收藏且仍处于公开状态的文章。
func (h *Handler) Collections(
	ctx *gin.Context,
	req ListReq,
	uc *ijwt.UserClaims,
) (handler.Result, error) {
	if errMsg := validatePagination(req); errMsg != "" {
		return handler.Result{
			Code: 4,
			Msg:  errMsg,
		}, nil
	}
	offset := (req.Page - 1) * req.PageSize
	items, err := h.svc.ListCollected(ctx, articleBiz, uc.Uid, offset, req.PageSize)
	if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "查询收藏失败",
		}, err
	}

	total, err := h.svc.CountCollected(ctx, articleBiz, uc.Uid)
	if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "查询收藏失败",
		}, err
	}
	data := make([]CollectedArticleVO, len(items))
	for i, item := range items {
		article := domain.Article(item)
		data[i] = CollectedArticleVO{
			Id:             item.Id,
			Title:          item.Title,
			Abstract:       article.Abstract(),
			AuthorId:       item.Author.Id,
			AuthorNickname: item.Author.Nickname,
			Status:         item.Status.ToUint8(),
			CreatedAt:      item.CreatedAt,
			UpdatedAt:      item.UpdatedAt,
		}
	}
	return handler.Result{
		Code: 0,
		Msg:  "success",
		Data: handler.NewPageResult(data, req.Page, req.PageSize, total),
	}, nil
}

// CancelCollect 取消收藏
func (h *Handler) CancelCollect(ctx *gin.Context, uc *ijwt.UserClaims) (handler.Result, error) {
	id, ok := articleID(ctx)
	if !ok {
		return handler.Result{
			Code: 4,
			Msg:  "文章 id 参数错误",
		}, nil
	}
	if err := h.svc.CancelCollect(ctx, articleBiz, id, uc.Uid); err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "取消收藏失败",
		}, err
	}
	return handler.Result{
		Code: 0,
		Msg:  "取消收藏成功",
	}, nil
}

// Collect 收藏
func (h *Handler) Collect(ctx *gin.Context, uc *ijwt.UserClaims) (handler.Result, error) {
	id, ok := articleID(ctx)
	if !ok {
		return handler.Result{
			Code: 4,
			Msg:  "文章 id 参数错误",
		}, nil
	}
	if err := h.svc.Collect(ctx, articleBiz, id, uc.Uid); errors.Is(
		err,
		service.ErrInteractiveTargetNotFound,
	) {
		return handler.Result{Code: 4, Msg: "文章不存在或不可操作", HTTPStatus: http.StatusNotFound}, nil
	} else if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "收藏失败",
		}, err
	}
	return handler.Result{
		Code: 0,
		Msg:  "收藏成功",
	}, nil
}

// CancelLike 取消点赞
func (h *Handler) CancelLike(ctx *gin.Context, uc *ijwt.UserClaims) (handler.Result, error) {
	id, ok := articleID(ctx)
	if !ok {
		return handler.Result{
			Code: 4,
			Msg:  "文章 id 参数错误",
		}, nil
	}
	if err := h.svc.CancelLike(ctx, articleBiz, id, uc.Uid); err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "取消点赞失败",
		}, err
	}
	return handler.Result{
		Code: 0,
		Msg:  "取消点赞成功",
	}, nil
}

// Like 点赞
func (h *Handler) Like(ctx *gin.Context, uc *ijwt.UserClaims) (handler.Result, error) {
	id, ok := articleID(ctx)
	if !ok {
		return handler.Result{
			Code: 4,
			Msg:  "文章 id 参数错误",
		}, nil
	}
	if err := h.svc.Like(ctx, articleBiz, id, uc.Uid); errors.Is(
		err,
		service.ErrInteractiveTargetNotFound,
	) {
		return handler.Result{Code: 4, Msg: "文章不存在或不可操作", HTTPStatus: http.StatusNotFound}, nil
	} else if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "点赞失败",
		}, err
	}
	return handler.Result{
		Code: 0,
		Msg:  "点赞成功",
	}, nil
}

// UserInteractionStatus 查询文章互动计数；登录后同时返回当前用户的点赞、收藏状态。
func (h *Handler) UserInteractionStatus(ctx *gin.Context) (handler.Result, error) {
	id, ok := articleID(ctx)
	if !ok {
		return handler.Result{
			Code: 4,
			Msg:  "文章 id 参数错误",
		}, nil
	}
	var uid int64
	if value, exists := ctx.Get("claims"); exists {
		claims, ok := value.(*ijwt.UserClaims)
		if !ok {
			return handler.Result{Code: 4, Msg: "登录信息无效", HTTPStatus: http.StatusUnauthorized}, nil
		}
		uid = claims.Uid
	}
	inter, err := h.svc.Get(ctx, articleBiz, id, uid)
	if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "查询互动状态失败",
		}, err
	}
	return handler.Result{
		Code: 0,
		Msg:  "success",
		Data: InteractionStatusVO{
			Liked:        inter.Liked,
			LikeCount:    inter.LikeCnt,
			ReadCount:    inter.ReadCnt,
			Collected:    inter.Collected,
			CollectCount: inter.CollectCnt,
		},
	}, nil
}

// PublicInteractions 查询指定文章的公开互动计数，不要求用户登录。
func (h *Handler) PublicInteractions(ctx *gin.Context) (handler.Result, error) {
	id, ok := articleID(ctx)
	if !ok {
		return handler.Result{
			Code: 4,
			Msg:  "文章 id 参数错误",
		}, nil
	}
	inter, err := h.svc.Get(ctx, articleBiz, id, 0)
	if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "查询互动数据失败",
		}, err
	}
	return handler.Result{
		Code: 0,
		Msg:  "success",
		Data: newInteractionCountVO(inter),
	}, nil
}

// PublicBatchStatus 批量查询文章的公开点赞、阅读和收藏计数。
func (h *Handler) PublicBatchStatus(
	ctx *gin.Context,
	req BatchInteractionReq,
) (handler.Result, error) {
	if len(req.IDs) == 0 || len(req.IDs) > 50 {
		return handler.Result{
			Code: 4,
			Msg:  "文章 id 列表参数错误",
		}, nil
	}
	ids := make([]int64, 0, len(req.IDs))
	for _, id := range req.IDs {
		if id > 0 {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return handler.Result{
			Code: 4,
			Msg:  "文章 id 列表参数错误",
		}, nil
	}
	items, err := h.svc.BatchGet(ctx, articleBiz, ids)
	if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "查询互动数据失败",
		}, err
	}
	data := make(map[string]InteractionCountVO, len(ids))
	for _, id := range ids {
		data[strconv.FormatInt(id, 10)] = newInteractionCountVO(items[id])
	}
	return handler.Result{
		Code: 0,
		Msg:  "success",
		Data: data,
	}, nil
}

func newInteractionCountVO(inter domain.Interactive) InteractionCountVO {
	return InteractionCountVO{
		LikeCount:    inter.LikeCnt,
		ReadCount:    inter.ReadCnt,
		CollectCount: inter.CollectCnt,
	}
}

func articleID(ctx *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	return id, err == nil && id > 0
}

func validatePagination(req ListReq) string {
	if req.Page < 1 || req.PageSize < 1 {
		return "page 和 pageSize 必须大于 0"
	}
	if req.PageSize > 50 {
		return "pageSize 不能超过 50"
	}
	return ""
}
