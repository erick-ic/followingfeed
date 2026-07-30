package domain

type Article struct {
	Id        int64
	Title     string
	Content   string
	Author    Author
	Status    ArticleStatus
	CreatedAt int64
	UpdatedAt int64
	DeletedAt int64
}

// Abstract 用于列表接口展示，避免传输完整内容
func (a *Article) Abstract() string {
	cs := []rune(a.Content)
	if len(cs) < 100 {
		return a.Content
	}
	return string(cs[:100])
}

type Author struct {
	Id       int64
	Nickname string
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

type PublishArticle Article
