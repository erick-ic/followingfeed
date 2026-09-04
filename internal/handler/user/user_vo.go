package user

// SignUpReq 是注册接口的 JSON 请求。
type SignUpReq struct {
	Nickname        string `json:"nickname"`         // 必填，2 至 6 位中文、英文字母或数字
	Email           string `json:"email"`            // 必填，作为登录账号
	Password        string `json:"password"`         // 至少 8 位，包含字母、数字和特殊字符
	ConfirmPassword string `json:"confirm_password"` // 必须与 password 一致
}

// LoginReq 是邮箱密码登录的 JSON 请求。
type LoginReq struct {
	Email    string `json:"email"`    // 注册邮箱
	Password string `json:"password"` // 明文仅用于本次 TLS 请求，服务端使用 bcrypt 校验
}

// UserVO 是当前登录用户的个人资料响应，包含本人可见邮箱和文章互动汇总。
type UserVO struct {
	Id                  int64  `json:"id,omitempty"`                  // 用户唯一 ID
	Nickname            string `json:"nickname,omitempty"`            // 公开展示昵称
	Email               string `json:"email,omitempty"`               // 仅本人资料接口返回
	CreatedAt           int64  `json:"createdAt,omitempty"`           // 注册时间，Unix 毫秒
	UpdatedAt           int64  `json:"updatedAt,omitempty"`           // 资料更新时间，Unix 毫秒
	ArticleLikeCount    int64  `json:"articleLikeCount,omitempty"`    // 本人已发布文章获赞总数
	ArticleReadCount    int64  `json:"articleReadCount,omitempty"`    // 本人已发布文章阅读总数
	ArticleCollectCount int64  `json:"articleCollectCount,omitempty"` // 本人已发布文章收藏总数
}

// PublicUserVO 是作者主页响应，不包含邮箱等私有字段。
type PublicUserVO struct {
	Id             int64  `json:"id"`             // 作者用户 ID
	Nickname       string `json:"nickname"`       // 作者公开昵称
	CreatedAt      int64  `json:"createdAt"`      // 注册时间，Unix 毫秒
	FollowingCount int64  `json:"followingCount"` // 作者关注人数
	FollowersCount int64  `json:"followersCount"` // 作者粉丝人数
	ArticleCount   int64  `json:"articleCount"`   // 已发布且未删除文章数
}
