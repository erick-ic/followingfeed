package article

import (
	"followingfeed/internal/domain"
	"followingfeed/internal/handler"
)

// ArticleReq 是保存草稿和发布文章共用的 JSON 请求。
type ArticleReq struct {
	Id      int64  `json:"id"`      // 新建传 0，编辑或发布已有文章传文章 ID
	Title   string `json:"title"`   // 必填，最多 200 个 Unicode 字符
	Content string `json:"content"` // 必填，UTF-8 内容最多 60 KiB
}

// ListReq 是文章列表的分页参数，GET 请求从 Query 绑定。
type ListReq struct {
	// GET 请求通过 URL Query 参数传递，例如：?page=1&pageSize=10。
	Page     int `form:"page"     json:"page"`     // 页码，从 1 开始
	PageSize int `form:"pageSize" json:"pageSize"` // 每页数量
}

// ArticleStatusSummary 是当前用户全部未删除文章的状态统计，不受分页影响。
type ArticleStatusSummary struct {
	Draft     int64 `json:"draft"`     // 草稿数量
	Published int64 `json:"published"` // 已发布数量
}

// MyArticleListResult 组合当前用户的文章分页数据和状态统计。
type MyArticleListResult struct {
	List    handler.PageResult[ArticleVO] `json:"list"`
	Summary ArticleStatusSummary          `json:"summary"`
}

// ArticleWithdrawReq 是撤回文章的 JSON 请求。
type ArticleWithdrawReq struct {
	Id int64 `json:"id"` // 当前用户拥有的已发布文章 ID
}

// ArticleDeleteReq 是软删除文章的 JSON 请求。
type ArticleDeleteReq struct {
	Id int64 `json:"id"` // 当前用户拥有的文章 ID
}

// ArticleVO 是文章接口响应；列表返回摘要，详情返回完整正文。
type ArticleVO struct {
	Id             int64  `json:"id,omitempty"`             // 文章唯一 ID
	Title          string `json:"title,omitempty"`          // 文章标题
	Abstract       string `json:"abstract,omitempty"`       // 摘要，列表接口返回（前100字）
	Content        string `json:"content,omitempty"`        // 完整内容，详情接口返回
	AuthorId       int64  `json:"authorId,omitempty"`       // 作者用户 ID
	AuthorNickname string `json:"authorNickname,omitempty"` // 作者公开昵称
	Status         uint8  `json:"status,omitempty"`         // 文章状态：0未知/1未发表/2已发表/3私密
	CreatedAt      int64  `json:"createdAt"`                // 创建时间，Unix 毫秒时间戳
	UpdatedAt      int64  `json:"updatedAt"`                // 更新时间，Unix 毫秒时间戳
}

// toDomain 将请求转换为领域模型；作者 ID 只取 JWT，不信任前端传值。
func (ar *ArticleReq) toDomain(uid int64) domain.Article {
	return domain.Article{
		Id:      ar.Id,
		Title:   ar.Title,
		Content: ar.Content,
		Author: domain.Author{
			Id: uid,
		},
	}
}
