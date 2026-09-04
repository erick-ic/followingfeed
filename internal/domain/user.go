package domain

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// User 表示系统用户；Password 仅供认证流程使用，不得进入公开响应或日志。
type User struct {
	Id        int64  // 用户唯一 ID；注册且尚未持久化时可以为 0。
	Nickname  string // 面向其他用户展示的昵称，允许重复。
	Email     string // 登录邮箱，按数据库唯一约束标识账号。
	Password  string // bcrypt 密码哈希；Service 接收注册请求时可暂存明文并立即哈希。
	CreatedAt int64  // 注册时间，Unix 毫秒时间戳。
	UpdatedAt int64  // 用户资料最后更新时间，Unix 毫秒时间戳。
}

const (
	NicknameMinLength = 2
	NicknameMaxLength = 6
	DefaultNickname   = "技术旅人"
)

var reservedNicknames = map[string]struct{}{
	"管理员":           {},
	"官方":            {},
	"官方账号":          {},
	"系统":            {},
	"followingfeed": {},
}

// IsValidNickname 校验公开展示昵称。昵称允许重复，用户身份仍以用户 ID 为准。
func IsValidNickname(nickname string) bool {
	if nickname != strings.TrimSpace(nickname) {
		return false
	}

	length := utf8.RuneCountInString(nickname)
	if length < NicknameMinLength || length > NicknameMaxLength {
		return false
	}

	if _, reserved := reservedNicknames[strings.ToLower(nickname)]; reserved {
		return false
	}

	for _, char := range nickname {
		if unicode.Is(unicode.Han, char) ||
			char >= 'a' && char <= 'z' ||
			char >= 'A' && char <= 'Z' ||
			char >= '0' && char <= '9' {
			continue
		}
		return false
	}
	return true
}
