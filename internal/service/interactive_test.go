package service

import (
	"context"
	"errors"
	"testing"

	"followingfeed/internal/domain"
	repomocks "followingfeed/internal/repository/mocks"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

//go:generate mockgen -source=interactive.go -package=svcmocks -destination=mocks/interactive.mock.go
func TestInteractiveServiceGet(t *testing.T) {
	wantErr := errors.New("database unavailable")
	testCases := []struct {
		name  string
		uid   int64
		setup func(*repomocks.MockInteractiveRepository)
		want  domain.Interactive
		err   error
	}{
		{
			name: "登录用户依次查询聚合点赞和收藏状态", uid: 1,
			setup: func(repo *repomocks.MockInteractiveRepository) {
				repo.EXPECT().
					Get(gomock.Any(), "article", int64(2)).
					Return(domain.Interactive{Biz: "article", BizId: 2, LikeCnt: 3}, nil)
				repo.EXPECT().Liked(gomock.Any(), "article", int64(2), int64(1)).Return(true, nil)
				repo.EXPECT().
					Collected(gomock.Any(), "article", int64(2), int64(1)).
					Return(false, nil)
			},
			want: domain.Interactive{Biz: "article", BizId: 2, LikeCnt: 3, Liked: true},
		},
		{
			name: "公开请求只查询聚合数据",
			setup: func(repo *repomocks.MockInteractiveRepository) {
				repo.EXPECT().
					Get(gomock.Any(), "article", int64(2)).
					Return(domain.Interactive{Biz: "article", BizId: 2, LikeCnt: 3}, nil)
			},
			want: domain.Interactive{Biz: "article", BizId: 2, LikeCnt: 3},
		},
		{
			name: "聚合查询失败", uid: 1,
			setup: func(repo *repomocks.MockInteractiveRepository) {
				repo.EXPECT().
					Get(gomock.Any(), "article", int64(2)).
					Return(domain.Interactive{}, wantErr)
			},
			err: wantErr,
		},
		{
			name: "点赞状态查询失败", uid: 1,
			setup: func(repo *repomocks.MockInteractiveRepository) {
				repo.EXPECT().
					Get(gomock.Any(), "article", int64(2)).
					Return(domain.Interactive{Biz: "article", BizId: 2}, nil)
				repo.EXPECT().
					Liked(gomock.Any(), "article", int64(2), int64(1)).
					Return(false, wantErr)
			},
			err: wantErr,
		},
		{
			name: "收藏状态查询失败", uid: 1,
			setup: func(repo *repomocks.MockInteractiveRepository) {
				repo.EXPECT().
					Get(gomock.Any(), "article", int64(2)).
					Return(domain.Interactive{Biz: "article", BizId: 2}, nil)
				repo.EXPECT().Liked(gomock.Any(), "article", int64(2), int64(1)).Return(false, nil)
				repo.EXPECT().
					Collected(gomock.Any(), "article", int64(2), int64(1)).
					Return(false, wantErr)
			},
			want: domain.Interactive{Biz: "article", BizId: 2}, err: wantErr,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := repomocks.NewMockInteractiveRepository(ctrl)
			tc.setup(repo)
			got, err := NewInteractiveService(repo).Get(context.Background(), "article", 2, tc.uid)
			assert.Equal(t, tc.want, got)
			assert.ErrorIs(t, err, tc.err)
		})
	}
}
