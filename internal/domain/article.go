package domain

// Article 是制作库模型，保存作者自己的草稿、已发布和私密文章。
type Article struct {
	Id        int64         // 文章唯一 ID；新建且尚未持久化时可以为 0。
	Title     string        // 文章标题。
	Content   string        // 文章完整正文；列表展示应使用 Abstract 生成摘要。
	Author    Author        // 文章作者的公开身份摘要。
	Status    ArticleStatus // 文章当前状态，决定作者和其他用户的可见范围。
	CreatedAt int64         // 创建时间，Unix 毫秒时间戳。
	UpdatedAt int64         // 内容或状态最后更新时间，Unix 毫秒时间戳。
	DeletedAt int64         // 软删除时间，Unix 毫秒时间戳；0 表示未删除。
}

// Abstract 用于列表接口展示，避免传输完整内容
func (a *Article) Abstract() string {
	cs := []rune(a.Content)
	if len(cs) <= 100 {
		return a.Content
	}
	return string(cs[:100])
}

// Author 是嵌入文章读取模型的作者公开信息，不包含账号凭据等私有字段。
type Author struct {
	Id       int64  // 作者用户 ID。
	Nickname string // 作者公开昵称；关联用户不存在时可能为空。
}

const (
	ArticleStatusUnknown     ArticleStatus = iota // 未知状态（默认值，不应出现）
	ArticleStatusUnPublished                      // 未发表状态（草稿）
	ArticleStatusPublished                        // 已发表状态（对外可见）
	ArticleStatusPrivate                          // 私密状态（仅作者可见）
)

type ArticleStatus uint8

// ToUint8 将文章状态转换为uint8类型，用于数据库存储和 JSON 响应
func (as ArticleStatus) ToUint8() uint8 {
	return uint8(as)
}

// PublishArticle 是公开读取模型；公开查询仍需过滤已发布且未删除的数据。
type PublishArticle Article
