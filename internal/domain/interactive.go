package domain

// Interactive 聚合一种业务资源的阅读、点赞和收藏数据。
// Liked、Collected 是当前登录用户的状态，不属于全局计数。
type Interactive struct {
	Biz        string // 业务资源类型，例如 article。
	BizId      int64  // 业务资源 ID，与 Biz 共同定位互动对象。
	ReadCnt    int64  // 该资源累计阅读次数。
	LikeCnt    int64  // 该资源当前有效点赞数。
	CollectCnt int64  // 该资源当前有效收藏数。

	Liked      bool   // 当前登录用户是否已点赞；匿名请求为 false。
	Collected  bool   // 当前登录用户是否已收藏；匿名请求为 false。
}