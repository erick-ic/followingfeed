package domain

// Follow 表示 followerId 关注 followingId；同一关系由数据库唯一约束保证幂等。
type Follow struct {
	Id          int64  // 关注关系唯一 ID。
	FollowerId  int64  // 发起关注的用户 ID。
	FollowingId int64  // 被关注的用户 ID。
	Nickname    string // 关注或粉丝列表中对端用户的公开昵称。
	CreatedAt   int64  // 关注关系建立时间，Unix 毫秒时间戳。
	UpdatedAt   int64  // 关注关系最后更新时间，Unix 毫秒时间戳。
}
