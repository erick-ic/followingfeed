package article

import (
	"followingfeed/internal/domain"
	"followingfeed/internal/handler"
	ijwt "followingfeed/internal/handler/jwt"
	"followingfeed/internal/service"
	"followingfeed/pkg/ginx"
	"followingfeed/pkg/logger"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ecodeclub/ekit/slice"
	"github.com/gin-gonic/gin"
)

const (
	maxPageSize            = 50
	maxArticleTitleRunes   = 200
	maxArticleContentBytes = 60 * 1024
)

type ArticleHandler struct {
	svc service.ArticleService
	l   logger.LoggerV1
	biz string
}

func NewArticleHandler(svc service.ArticleService, l logger.LoggerV1) *ArticleHandler {
	return &ArticleHandler{
		svc: svc,
		l:   l,
		biz: "article",
	}
}

func (ah *ArticleHandler) RegisterArticlesRouters(server *gin.Engine) {
	group := server.Group("/api/v1/articles")
	// 编辑/保存文章草稿
	group.POST(
		"/edit",
		//中间件放入 Gin Context 的实际类型是指针，因此用指针
		ginx.WrapperReqWithToken[ArticleReq, *ijwt.UserClaims](ah.l, ah.Edit),
	)
	// 发表文章（同步到制作库和线上库）
	group.POST("/publish", ginx.WrapperReqWithToken[ArticleReq, *ijwt.UserClaims](ah.l, ah.Publish))
	// 撤回文章（状态改为未发表）
	group.POST(
		"/withdraw",
		ginx.WrapperReqWithToken[ArticleWithdrawReq, *ijwt.UserClaims](ah.l, ah.Withdraw),
	)
	// 软删除文章（同步删除制作库和线上库的可见性）
	group.POST(
		"/delete",
		ginx.WrapperReqWithToken[ArticleDeleteReq, *ijwt.UserClaims](ah.l, ah.Delete),
	)
	// 制作库列表
	group.GET(
		"/list",
		ginx.WrapperReqWithToken[ListReq, *ijwt.UserClaims](ah.l, ah.ArticleList),
	)
	// 制作库详情
	group.GET("/detail/:id", ah.Detail)

	pubGroup := server.Group("/api/v1/pub")
	// 线上库列表
	pubGroup.GET(
		"/list",
		ginx.WrapperReq[ListReq](ah.l, ah.PubArticleList),
	)
	// 线上库文章详情
	pubGroup.GET("/detail/:id", ah.PubArticleDetail)
}

func (ah *ArticleHandler) PubArticleDetail(ctx *gin.Context) {
	idStr := ctx.Param("id")
	articleId, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusOK, handler.Result{
			Code: 4,
			Msg:  "id 参数错误！",
		})
		ah.l.Warn("查询文章失败，id格式不对！",
			logger.String("articleId", idStr),
			logger.Error(err),
		)
		return
	}

	//c, ok := ctx.Get("claims")
	//if !ok {
	//	ctx.JSON(http.StatusInternalServerError, handler.Result{
	//		Code: 5,
	//		Msg:  "系统错误！",
	//	})
	//	ah.l.Warn("查询文章失败，系统错误！",
	//		logger.Int64("articleId", articleId),
	//		logger.Error(err),
	//	)
	//	return
	//}
	//claims, ok := c.(*ijwt.UserClaims)
	//if !ok {
	//	ctx.JSON(http.StatusInternalServerError, handler.Result{
	//		Code: 5,
	//		Msg:  "系统错误！",
	//	})
	//	ah.l.Warn("查询文章失败，claims 类型错误！",
	//		logger.Int64("userId", claims.Uid),
	//		logger.Error(err),
	//	)
	//	return
	//}

	// 获取文章详情（从线上库），Service 层会异步发送阅读事件到 Kafka
	//res, err := ah.svc.GetByPubId(ctx, articleId, claims.Uid)
	res, err := ah.svc.GetByPubId(ctx, articleId)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, handler.Result{
			Code: 5,
			Msg:  "系统错误",
		})
		ah.l.Error("查询文章失败",
			logger.Int64("articleId", articleId),
			logger.Error(err),
		)
		return
	}

	//// 获取互动数据（阅读数/点赞数/收藏数/是否点赞/是否收藏）
	//iter, err := ah.interSvc.Get(ctx, &interactivev1.GetRequest{
	//	Biz:   ah.biz,
	//	BizId: res.Id,
	//	Uid:   claims.Uid,
	//})
	//if err != nil {
	//	ctx.JSON(http.StatusInternalServerError, handler.Result{
	//		Code: 5,
	//		Msg:  "系统错误",
	//	})
	//	ah.l.Error("查询文章失败",
	//		logger.Int64("articleId", articleId),
	//		logger.Error(err),
	//	)
	//	return
	//}

	// 组装 VO 返回前端
	vo := ArticleVO{
		Id:      res.Id,
		Title:   res.Title,
		Content: res.Content,
		//ReadCnt:    iter.Inter.GetReadCnt(),
		//LikeCnt:    iter.Inter.GetLikeCnt(),
		//CollectCnt: iter.Inter.GetCollectCnt(),
		//Liked:      iter.Inter.GetLiked(),
		//Collected:  iter.Inter.GetCollected(),

		Status:         res.Status.ToUint8(),
		AuthorId:       res.Author.Id,
		AuthorNickname: res.Author.Nickname,
		CreatedAt:      time.UnixMilli(res.CreatedAt).Format(time.DateTime),
		UpdatedAt:      time.UnixMilli(res.UpdatedAt).Format(time.DateTime),
	}
	ctx.JSON(http.StatusOK, handler.Result{
		Code: 0,
		Msg:  "success",
		Data: vo,
	})
}

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
		}, nil
	}

	// 将领域模型列表转换为 VO 列表
	// 列表接口不返回 content（数据量大），只返回摘要 Abstract
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
				//Content: src.Content,
				CreatedAt: time.UnixMilli(src.CreatedAt).Format(time.DateTime),
				UpdatedAt: time.UnixMilli(src.UpdatedAt).Format(time.DateTime),
			}
		})
	return handler.Result{
		Code: 0,
		Msg:  "success",
		Data: data,
	}, nil
}

func (ah *ArticleHandler) Detail(ctx *gin.Context) {
	value, ok := ctx.Get("claims")
	if !ok {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	claims, ok := value.(*ijwt.UserClaims)
	if !ok || claims == nil {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	idStr := ctx.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, handler.Result{
			Code: 4,
			Msg:  "id 参数错误！",
		})
		ah.l.Warn("查询文章失败，id 错误！",
			logger.String("id", idStr),
			logger.Error(err),
		)
		return
	}
	res, er := ah.svc.GetById(ctx, id)
	if er != nil {
		ctx.JSON(http.StatusInternalServerError, handler.Result{
			Code: 5,
			Msg:  "系统错误",
		})
		ah.l.Error("查询文章失败",
			logger.Int64("id", id),
			logger.Error(er),
		)
		return
	}
	if res.Author.Id != claims.Uid {
		ctx.JSON(http.StatusForbidden, handler.Result{Code: 4, Msg: "无权查看此文章"})
		return
	}
	vo := ArticleVO{
		Id:        res.Id,
		Title:     res.Title,
		Content:   res.Content,
		Status:    res.Status.ToUint8(),
		AuthorId:  res.Author.Id,
		CreatedAt: time.UnixMilli(res.CreatedAt).Format(time.DateTime),
		UpdatedAt: time.UnixMilli(res.UpdatedAt).Format(time.DateTime),
	}
	ctx.JSON(http.StatusOK, handler.Result{
		Code: 0,
		Msg:  "success",
		Data: vo,
	})
}

func (ah *ArticleHandler) ArticleList(ctx *gin.Context, req ListReq, uc *ijwt.UserClaims) (handler.Result, error) {
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
		}, nil
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
				//Content: src.Content,
				CreatedAt: time.UnixMilli(src.CreatedAt).Format(time.DateTime),
				UpdatedAt: time.UnixMilli(src.UpdatedAt).Format(time.DateTime),
			}
		})
	return handler.Result{
		Code: 0,
		Msg:  "success",
		Data: data,
	}, nil
}

func (ah *ArticleHandler) Withdraw(ctx *gin.Context, req ArticleWithdrawReq, uc *ijwt.UserClaims) (handler.Result, error) {
	id, err := ah.svc.Withdraw(ctx, req.Id, uc.Uid)
	if err != nil {
		ah.l.Error("撤回帖子失败", logger.Error(err))
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

func (ah *ArticleHandler) Delete(ctx *gin.Context, req ArticleDeleteReq, uc *ijwt.UserClaims) (handler.Result, error) {
	id, err := ah.svc.Delete(ctx, req.Id, uc.Uid)
	if err != nil {
		ah.l.Error("删除文章失败", logger.Error(err))
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

func (ah *ArticleHandler) Publish(ctx *gin.Context, req ArticleReq, uc *ijwt.UserClaims) (handler.Result, error) {
	if errMsg := validateArticleReq(&req); errMsg != "" {
		return handler.Result{Code: 4, Msg: errMsg}, nil
	}
	id, err := ah.svc.Publish(ctx, req.toDomain(uc.Uid))
	if err != nil {
		ah.l.Error("发表帖子失败", logger.Error(err))
		return handler.Result{
			Code: 5,
			Msg:  "系统错误！",
		}, nil
	}
	return handler.Result{
		Code: 0,
		Msg:  "发表成功～",
		Data: id,
	}, nil
}

func (ah *ArticleHandler) Edit(ctx *gin.Context, req ArticleReq, uc *ijwt.UserClaims) (handler.Result, error) {
	if errMsg := validateArticleReq(&req); errMsg != "" {
		return handler.Result{Code: 4, Msg: errMsg}, nil
	}
	id, err := ah.svc.Save(ctx, req.toDomain(uc.Uid))
	if err != nil {
		ah.l.Error("修改帖子失败", logger.Error(err))
		return handler.Result{
			Code: 5,
			Msg:  "系统错误！",
		}, nil
	}
	return handler.Result{
		Code: 0,
		Msg:  "编辑成功～",
		Data: id,
	}, nil
}

func validatePagination(req ListReq) string {
	if req.Page < 1 || req.PageSize < 1 {
		return "page 和 pageSize 必须大于 0"
	}
	if req.PageSize > maxPageSize {
		return "pageSize 不能超过 50"
	}
	return ""
}

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
