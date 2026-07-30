package domain

import "testing"

func TestIsValidNickname(t *testing.T) {
	testCases := []struct {
		name     string
		nickname string
		want     bool
	}{
		{name: "中文昵称", nickname: "云端旅人", want: true},
		{name: "中英文数字混合", nickname: "极客Go2", want: true},
		{name: "少于两位", nickname: "云"},
		{name: "超过六位", nickname: "云端漫游开发者"},
		{name: "包含空格", nickname: "云端 旅人"},
		{name: "包含符号", nickname: "云端-旅人"},
		{name: "保留名称", nickname: "管理员"},
		{name: "产品保留名称", nickname: "FollowingFeed"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsValidNickname(tc.nickname); got != tc.want {
				t.Fatalf("IsValidNickname(%q) = %v, want %v", tc.nickname, got, tc.want)
			}
		})
	}
}
