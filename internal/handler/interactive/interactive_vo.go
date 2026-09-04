package interactive

// BatchInteractionReq 是批量查询文章互动数据的 JSON 请求，最多传 50 个 ID。
type BatchInteractionReq struct {
	IDs []int64 `json:"ids"` // 待查询的文章 ID，必须包含 1 至 50 个正整数
}

// ListReq 是收藏列表的分页参数，GET 请求从 Query 绑定。
type ListReq struct {
	Page     int `form:"page"     json:"page"`
	PageSize int `form:"pageSize" json:"pageSize"`
}

// InteractionCountVO 是文章公开互动计数。
type InteractionCountVO struct {
	LikeCount    int64 `json:"likeCount"`    // 当前文章累计点赞数
	ReadCount    int64 `json:"readCount"`    // 当前文章累计阅读数
	CollectCount int64 `json:"collectCount"` // 当前文章累计收藏数
}

// InteractionStatusVO 返回公开计数，并在登录时带上当前用户的互动状态。
type InteractionStatusVO struct {
	Liked        bool  `json:"liked"`        // 当前登录用户是否已点赞
	LikeCount    int64 `json:"likeCount"`    // 当前文章累计点赞数
	ReadCount    int64 `json:"readCount"`    // 当前文章累计阅读数
	Collected    bool  `json:"collected"`    // 当前登录用户是否已收藏
	CollectCount int64 `json:"collectCount"` // 当前文章累计收藏数
}

// CollectedArticleVO 是收藏列表中的文章摘要。
type CollectedArticleVO struct {
	Id             int64  `json:"id"`                       // 文章唯一 ID
	Title          string `json:"title"`                    // 文章标题
	Abstract       string `json:"abstract"`                 // 正文前 100 个 Unicode 字符
	AuthorId       int64  `json:"authorId"`                 // 作者用户 ID
	AuthorNickname string `json:"authorNickname,omitempty"` // 作者公开昵称
	Status         uint8  `json:"status"`                   // 文章状态，收藏列表固定为 2（已发布）
	CreatedAt      int64  `json:"createdAt"`                // 发布时间，Unix 毫秒时间戳
	UpdatedAt      int64  `json:"updatedAt"`                // 更新时间，Unix 毫秒时间戳
}
