package article

import "followingfeed/internal/domain"

type ArticleReq struct {
	Id      int64  `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

type ListReq struct {
	// GET 请求通过 URL Query 参数传递，例如：?page=1&pageSize=10。
	Page     int `form:"page" json:"page"`         // 页码，从 1 开始
	PageSize int `form:"pageSize" json:"pageSize"` // 每页数量
}

type ArticleWithdrawReq struct {
	Id int64 `json:"id"`
}

type ArticleDeleteReq struct {
	Id int64 `json:"id"`
}

type ArticleVO struct {
	Id             int64  `json:"id,omitempty"`
	Title          string `json:"title,omitempty"`
	Abstract       string `json:"abstract,omitempty"` // 摘要，列表接口返回（前100字）
	Content        string `json:"content,omitempty"`  // 完整内容，详情接口返回
	AuthorId       int64  `json:"authorId,omitempty"`
	AuthorNickname string `json:"authorNickname,omitempty"`
	Status         uint8  `json:"status,omitempty"`     // 文章状态：0未知/1未发表/2已发表/3私密
	CreatedAt      string `json:"created_at,omitempty"` // 创建时间，格式：2006-01-02 15:04:05
	UpdatedAt      string `json:"updated_at,omitempty"` // 更新时间，格式：2006-01-02 15:04:05
}

// toDomain 将请求结构体转换为领域模型，注入作者ID
// 作者ID 从 JWT claims 中获取，确保数据归属正确
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
