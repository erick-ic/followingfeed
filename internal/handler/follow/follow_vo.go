package follow

import "followingfeed/internal/domain"

// ListReq 是关注/粉丝列表的分页参数，GET 请求从 Query 绑定。
type ListReq struct {
	Page     int `form:"page"     json:"page"`
	PageSize int `form:"pageSize" json:"pageSize"`
}

// FollowVO 是关注/粉丝列表项，统一使用前端约定的 camelCase 字段。
type FollowVO struct {
	Id          int64  `json:"id"`                 // 关注关系唯一 ID
	FollowerId  int64  `json:"followerId"`         // 发起关注的用户 ID
	FollowingId int64  `json:"followingId"`        // 被关注的用户 ID
	Nickname    string `json:"nickname,omitempty"` // 列表对端用户的公开昵称
	CreatedAt   int64  `json:"createdAt"`          // 建立关注关系的时间，Unix 毫秒
	UpdatedAt   int64  `json:"updatedAt"`          // 关注关系更新时间，Unix 毫秒
}

func newFollowVO(item domain.Follow) FollowVO {
	return FollowVO{
		Id:          item.Id,
		FollowerId:  item.FollowerId,
		FollowingId: item.FollowingId,
		Nickname:    item.Nickname,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}
