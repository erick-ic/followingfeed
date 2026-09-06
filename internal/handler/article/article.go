package article

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
	"strings"
	"unicode/utf8"

	"github.com/ecodeclub/ekit/slice"
	"github.com/gin-gonic/gin"
)

const articleBiz = "article"

const (
	maxArticleTitleRunes   = 200
	maxArticleContentBytes = 60 * 1024
)

type ArticleHandler struct {
	svc            service.ArticleService
	interactiveSvc service.InteractiveService
	l              logger.LoggerV1
}

// NewArticleHandler 创建文章处理器，并注入文章服务、互动服务和日志组件。
func NewArticleHandler(
	svc service.ArticleService,
	l logger.LoggerV1,
	interactiveSvc service.InteractiveService,
) *ArticleHandler {
	return &ArticleHandler{
		svc:            svc,
		interactiveSvc: interactiveSvc,
		l:              l,
	}
}

// RegisterArticlesRouters 注册文章制作库、公开文章、关注流和作者文章列表路由。
func (ah *ArticleHandler) RegisterArticlesRouters(server *gin.Engine) {
	// 制作库：
	group := server.Group("/api/v1/articles")
	// POST /edit：JWT；JSON 为 ArticleReq，新建 id=0，编辑 id>0。
	group.POST("/edit", ginx.WrapperReqWithToken[ArticleReq, *ijwt.UserClaims](ah.l, ah.Edit))
	// POST /publish：JWT；JSON 为 ArticleReq，同步制作库和公开库。
	group.POST("/publish", ginx.WrapperReqWithToken[ArticleReq, *ijwt.UserClaims](ah.l, ah.Publish))
	// POST /withdraw：JWT；JSON {"id":文章ID}，状态恢复为草稿。
	group.POST(
		"/withdraw",
		ginx.WrapperReqWithToken[ArticleWithdrawReq, *ijwt.UserClaims](ah.l, ah.Withdraw),
	)
	// POST /delete：JWT；JSON {"id":文章ID}，制作库与公开库同步软删除。
	group.POST(
		"/delete",
		ginx.WrapperReqWithToken[ArticleDeleteReq, *ijwt.UserClaims](ah.l, ah.Delete),
	)
	// GET /list：JWT；Query page、pageSize，返回作者全部状态及状态统计。
	group.GET("/list", ginx.WrapperReqWithToken[ListReq, *ijwt.UserClaims](ah.l, ah.ArticleList))
	// GET /detail/:id：JWT；仅文章作者可读取制作库正文。
	group.GET("/detail/:id", ginx.WrapperWithToken[*ijwt.UserClaims](ah.l, ah.Detail))

	// 线上库：
	pubGroup := server.Group("/api/v1/pub")
	// GET /pub/list：公开；Query page、pageSize，只返回已发布文章摘要。
	pubGroup.GET("/list", ginx.WrapperReq[ListReq](ah.l, ah.PubArticleList))
	// GET /pub/detail/:id：公开；返回已发布文章正文并记录阅读量。
	pubGroup.GET("/detail/:id", ginx.Wrapper(ah.l, ah.PubArticleDetail))

	// GET /feed：JWT；Query page、pageSize，返回当前用户关注作者的文章。
	server.GET("/api/v1/feed", ginx.WrapperReqWithToken[ListReq, *ijwt.UserClaims](ah.l, ah.Feed))
	// GET /pub/users/:id/articles：公开；路径 id 为作者 ID，支持分页。
	server.GET(
		"/api/v1/pub/users/:id/articles",
		ginx.WrapperReq[ListReq](ah.l, ah.UserPublishedArticles),
	)
}

// UserPublishedArticles 分页查询指定作者公开且尚未删除的已发布文章。
func (ah *ArticleHandler) UserPublishedArticles(
	ctx *gin.Context,
	req ListReq,
) (handler.Result, error) {
	authorID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || authorID <= 0 {
		return handler.Result{
			Code: 4,
			Msg:  "用户 id 参数错误",
		}, nil
	}
	if msg := validatePagination(req); msg != "" {
		return handler.Result{
			Code: 4,
			Msg:  msg,
		}, nil
	}

	offset := (req.Page - 1) * req.PageSize
	articles, err := ah.svc.PubListByAuthor(ctx, authorID, offset, req.PageSize)
	if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "查询用户文章失败",
		}, err
	}

	total, err := ah.svc.CountPublishedByAuthor(ctx, authorID)
	if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "查询用户文章失败",
		}, err
	}
	data := slice.Map[domain.PublishArticle, ArticleVO](
		articles,
		func(_ int, src domain.PublishArticle) ArticleVO {
			article := domain.Article(src)
			return ArticleVO{
				Id:             src.Id,
				Title:          src.Title,
				Abstract:       article.Abstract(),
				AuthorId:       src.Author.Id,
				AuthorNickname: src.Author.Nickname,
				Status:         src.Status.ToUint8(),
				CreatedAt:      src.CreatedAt,
				UpdatedAt:      src.UpdatedAt,
			}
		})
	return handler.Result{
		Code: 0,
		Msg:  "success",
		Data: handler.NewPageResult(data, req.Page, req.PageSize, total),
	}, nil
}

// Feed 分页查询当前登录用户所关注作者发布的文章。
func (ah *ArticleHandler) Feed(
	ctx *gin.Context,
	req ListReq,
	claims *ijwt.UserClaims,
) (handler.Result, error) {
	if errMsg := validatePagination(req); errMsg != "" {
		return handler.Result{
			Code: 4,
			Msg:  errMsg,
		}, nil
	}

	offset := (req.Page - 1) * req.PageSize
	res, err := ah.svc.Feed(ctx, claims.Uid, offset, req.PageSize)
	if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "关注动态加载失败",
		}, err
	}

	total, err := ah.svc.CountFeed(ctx, claims.Uid)
	if err != nil {
		return handler.Result{Code: 5, Msg: "关注动态加载失败"}, err
	}

	data := slice.Map[domain.PublishArticle, ArticleVO](
		res,
		func(_ int, src domain.PublishArticle) ArticleVO {
			article := domain.Article(src)
			return ArticleVO{
				Id:             src.Id,
				Title:          src.Title,
				Abstract:       article.Abstract(),
				AuthorId:       src.Author.Id,
				AuthorNickname: src.Author.Nickname,
				Status:         src.Status.ToUint8(),
				CreatedAt:      src.CreatedAt,
				UpdatedAt:      src.UpdatedAt,
			}
		})
	return handler.Result{
		Code: 0,
		Msg:  "success",
		Data: handler.NewPageResult(data, req.Page, req.PageSize, total),
	}, nil
}

// PubArticleDetail 查询公开文章完整正文，并记录一次阅读行为。
// 点赞数、阅读数和收藏数由互动接口统一提供，不混入文章详情响应。
func (ah *ArticleHandler) PubArticleDetail(ctx *gin.Context) (handler.Result, error) {
	idStr := ctx.Param("id")
	articleId, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return handler.Result{
			Code: 4,
			Msg:  "id 参数错误！",
		}, nil
	}

	// 获取线上库文章详情。互动数据由 InteractiveHandler 单独提供。
	res, err := ah.svc.GetByPubId(ctx, articleId)
	if errors.Is(err, service.ErrArticleNotFound) {
		return handler.Result{Code: 4, Msg: "文章不存在", HTTPStatus: http.StatusNotFound}, nil
	}
	if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "系统错误",
		}, err
	}
	// 后续替换成 Kafka 异步消费
	if err := ah.interactiveSvc.RecordRead(ctx, articleBiz, articleId); err != nil {
		logger.FromContext(ctx, ah.l).Warn(
			"记录文章阅读量失败",
			logger.Int64("article_id", articleId),
			logger.Error(err),
		)
	}

	// 文章接口只返回文章数据；阅读、点赞和收藏数据由互动接口统一提供。
	vo := ArticleVO{
		Id:             res.Id,
		Title:          res.Title,
		Content:        res.Content,
		Status:         res.Status.ToUint8(),
		AuthorId:       res.Author.Id,
		AuthorNickname: res.Author.Nickname,
		CreatedAt:      res.CreatedAt,
		UpdatedAt:      res.UpdatedAt,
	}
	return handler.Result{
		Code: 0,
		Msg:  "success",
		Data: vo,
	}, nil
}

// PubArticleList 分页查询所有公开且尚未删除的已发布文章，列表只返回摘要。
func (ah *ArticleHandler) PubArticleList(ctx *gin.Context, req ListReq) (handler.Result, error) {
	if errMsg := validatePagination(req); errMsg != "" {
		return handler.Result{
			Code: 4,
			Msg:  errMsg,
		}, nil
	}

	// 客户端页码从 1 开始，数据库查询使用从 0 开始的 offset。
	offset := (req.Page - 1) * req.PageSize
	res, err := ah.svc.PubList(ctx, offset, req.PageSize)
	if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "系统错误！",
		}, err
	}
	total, err := ah.svc.CountPub(ctx)
	if err != nil {
		return handler.Result{Code: 5, Msg: "系统错误！"}, err
	}

	// 将领域模型列表转换为 VO 列表
	data := slice.Map[domain.PublishArticle, ArticleVO](
		res,
		func(idx int, src domain.PublishArticle) ArticleVO {
			article := domain.Article(src)
			return ArticleVO{
				Id:             src.Id,
				Title:          src.Title,
				Abstract:       article.Abstract(),
				Status:         src.Status.ToUint8(),
				AuthorId:       src.Author.Id,
				AuthorNickname: src.Author.Nickname,
				//列表不需要返回content
				CreatedAt: src.CreatedAt,
				UpdatedAt: src.UpdatedAt,
			}
		})
	return handler.Result{
		Code: 0,
		Msg:  "success",
		Data: handler.NewPageResult(data, req.Page, req.PageSize, total),
	}, nil
}

// Detail 查询当前登录用户自己的制作库文章详情，用于编辑页面回显。
func (ah *ArticleHandler) Detail(ctx *gin.Context, uc *ijwt.UserClaims) (handler.Result, error) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return handler.Result{
			Code: 4,
			Msg:  "id 参数错误！",
		}, nil
	}
	res, err := ah.svc.GetById(ctx, id, uc.Uid)
	if errors.Is(err, service.ErrArticleNotFound) {
		return handler.Result{Code: 4, Msg: "文章不存在", HTTPStatus: http.StatusNotFound}, nil
	}
	if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "系统错误",
		}, err
	}
	// DAO 已按作者 ID 限制查询；这里保留校验作为纵深防御。
	if res.Author.Id != uc.Uid {
		return handler.Result{Code: 4, Msg: "无权查看此文章", HTTPStatus: http.StatusForbidden}, nil
	}
	vo := ArticleVO{
		Id:        res.Id,
		Title:     res.Title,
		Content:   res.Content,
		Status:    res.Status.ToUint8(),
		AuthorId:  res.Author.Id,
		CreatedAt: res.CreatedAt,
		UpdatedAt: res.UpdatedAt,
	}
	return handler.Result{
		Code: 0,
		Msg:  "success",
		Data: vo,
	}, nil
}

// ArticleList 分页查询当前登录用户的全部未删除文章，并返回草稿和已发布数量汇总。
func (ah *ArticleHandler) ArticleList(
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

	// 客户端页码从 1 开始，数据库查询使用从 0 开始的 offset。
	offset := (req.Page - 1) * req.PageSize
	res, err := ah.svc.List(ctx, uc.Uid, offset, req.PageSize)
	if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "系统错误！",
		}, err
	}
	total, err := ah.svc.Count(ctx, uc.Uid)
	if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "系统错误！",
		}, err
	}
	statusCounts, err := ah.svc.CountByStatus(ctx, uc.Uid)
	if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "系统错误！",
		}, err
	}

	// 将领域模型列表转换为 VO 列表
	// 列表接口不返回 content（数据量大），只返回摘要 Abstract
	data := slice.Map[domain.Article, ArticleVO](
		res,
		func(idx int, src domain.Article) ArticleVO {
			return ArticleVO{
				Id:       src.Id,
				Title:    src.Title,
				Abstract: src.Abstract(),
				Status:   src.Status.ToUint8(),
				//列表不需要返回content
				CreatedAt: src.CreatedAt,
				UpdatedAt: src.UpdatedAt,
			}
		})
	return handler.Result{
		Code: 0,
		Msg:  "success",
		Data: MyArticleListResult{
			List: handler.NewPageResult(data, req.Page, req.PageSize, total),
			Summary: ArticleStatusSummary{
				Draft:     statusCounts[domain.ArticleStatusUnPublished],
				Published: statusCounts[domain.ArticleStatusPublished],
			},
		},
	}, nil
}

// Delete 软删除当前用户拥有的文章，并同步处理线上库中的对应文章。
func (ah *ArticleHandler) Delete(
	ctx *gin.Context,
	req ArticleDeleteReq,
	uc *ijwt.UserClaims,
) (handler.Result, error) {
	id, err := ah.svc.Delete(ctx, req.Id, uc.Uid)
	if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "删除文章失败！",
		}, err
	}
	return handler.Result{
		Code: 0,
		Msg:  "删除文章成功～",
		Data: id,
	}, nil
}

// Withdraw 将当前用户已发布的文章撤回为草稿，并同步更新线上库状态。
func (ah *ArticleHandler) Withdraw(
	ctx *gin.Context,
	req ArticleWithdrawReq,
	uc *ijwt.UserClaims,
) (handler.Result, error) {
	id, err := ah.svc.Withdraw(ctx, req.Id, uc.Uid)
	if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "系统错误！",
		}, err
	}
	return handler.Result{
		Code: 0,
		Msg:  "文章撤回成功～",
		Data: id,
	}, nil
}

// Publish 校验文章内容后发布文章，并同步保存到制作库和线上库。
func (ah *ArticleHandler) Publish(
	ctx *gin.Context,
	req ArticleReq,
	uc *ijwt.UserClaims,
) (handler.Result, error) {
	if errMsg := validateArticleReq(&req); errMsg != "" {
		return handler.Result{
			Code: 4,
			Msg:  errMsg,
		}, nil
	}

	id, err := ah.svc.Publish(ctx, req.toDomain(uc.Uid))
	if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "系统错误！",
		}, err
	}
	return handler.Result{
		Code: 0,
		Msg:  "发表成功～",
		Data: id,
	}, nil
}

// Edit 创建或编辑当前用户的制作库文章：请求 id 为 0 时创建，非 0 时更新。
func (ah *ArticleHandler) Edit(
	ctx *gin.Context,
	req ArticleReq,
	uc *ijwt.UserClaims,
) (handler.Result, error) {
	if errMsg := validateArticleReq(&req); errMsg != "" {
		return handler.Result{Code: 4, Msg: errMsg}, nil
	}

	id, err := ah.svc.Save(ctx, req.toDomain(uc.Uid))
	if err != nil {
		return handler.Result{
			Code: 5,
			Msg:  "系统错误！",
		}, err
	}
	return handler.Result{
		Code: 0,
		Msg:  "编辑成功～",
		Data: id,
	}, nil
}

// validatePagination 校验列表分页参数范围。
func validatePagination(req ListReq) string {
	return handler.ValidatePagination(req.Page, req.PageSize)
}

// validateArticleReq 校验文章标题和正文的必填项及长度限制。
func validateArticleReq(req *ArticleReq) string {
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		return "文章标题不能为空"
	}
	if utf8.RuneCountInString(req.Title) > maxArticleTitleRunes {
		return "文章标题不能超过 200 个字符"
	}
	if strings.TrimSpace(req.Content) == "" {
		return "文章正文不能为空"
	}
	if len(req.Content) > maxArticleContentBytes {
		return "文章正文不能超过 60KB"
	}
	return ""
}
