package domain

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

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
