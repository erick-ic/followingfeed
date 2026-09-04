//go:build integration

package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"followingfeed/config"
	"followingfeed/internal/domain"
	"followingfeed/internal/repository/dao"
	"followingfeed/migrations"
	"followingfeed/pkg/logger"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type repositoryMySQLFixture struct {
	db      *gorm.DB
	ctx     context.Context
	userIDs []int64
	artIDs  []int64
}

func newRepositoryMySQLFixture(t *testing.T) *repositoryMySQLFixture {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
	oldDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(root))
	cfg, err := config.Load()
	require.NoError(t, os.Chdir(oldDir))
	require.NoError(t, err, "请先启动 docker compose 中的 MySQL，并准备 config/dev.yaml 与 .env")
	db, err := gorm.Open(mysql.Open(cfg.MySQL.GetDSN()), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Ping())
	require.NoError(t, migrations.ApplyAll(context.Background(), sqlDB))
	f := &repositoryMySQLFixture{db: db, ctx: context.Background()}
	t.Cleanup(func() { f.cleanup(t) })
	return f
}

func (f *repositoryMySQLFixture) createUser(t *testing.T, nickname string) int64 {
	t.Helper()
	email := fmt.Sprintf("repository-integration-%s@example.com", uuid.NewString())
	require.NoError(t, dao.NewGORMUserDAO(f.db).Insert(f.ctx, domain.User{
		Nickname: nickname, Email: email, Password: "integration-hash",
	}))
	user, err := dao.NewGORMUserDAO(f.db).FindByEmail(f.ctx, email)
	require.NoError(t, err)
	id := int64(user.Id)
	f.userIDs = append(f.userIDs, id)
	return id
}

func (f *repositoryMySQLFixture) createPublishedArticle(
	t *testing.T,
	authorID int64,
	title string,
	updatedAt int64,
) int64 {
	t.Helper()
	article := dao.Article{Title: title, Content: "Repository 集成测试正文", AuthorId: authorID,
		Status: domain.ArticleStatusPublished.ToUint8(), CreatedAt: time.Now().UnixMilli(), UpdatedAt: updatedAt}
	require.NoError(t, f.db.Create(&article).Error)
	f.artIDs = append(f.artIDs, article.Id)
	published := dao.PublishArticle(article)
	require.NoError(t, f.db.Create(&published).Error)
	return article.Id
}

func (f *repositoryMySQLFixture) cleanup(t *testing.T) {
	t.Helper()
	if len(f.artIDs) > 0 {
		require.NoError(t, f.db.Where("biz_id IN ?", f.artIDs).Delete(&dao.UserLikeBiz{}).Error)
		require.NoError(
			t,
			f.db.Where("biz_id IN ?", f.artIDs).Delete(&dao.UserCollectionBiz{}).Error,
		)
		require.NoError(t, f.db.Where("biz_id IN ?", f.artIDs).Delete(&dao.Interactive{}).Error)
		require.NoError(t, f.db.Where("id IN ?", f.artIDs).Delete(&dao.PublishArticle{}).Error)
		require.NoError(t, f.db.Where("id IN ?", f.artIDs).Delete(&dao.Article{}).Error)
	}
	if len(f.userIDs) > 0 {
		require.NoError(
			t,
			f.db.Where("follower_id IN ? OR following_id IN ?", f.userIDs, f.userIDs).
				Delete(&dao.Follow{}).
				Error,
		)
		require.NoError(t, f.db.Where("id IN ?", f.userIDs).Delete(&dao.User{}).Error)
	}
}

func TestMySQLArticleRepositoryFeedDomainMapping(t *testing.T) {
	f := newRepositoryMySQLFixture(t)
	reader := f.createUser(t, "Repository 读者")
	author := f.createUser(t, "Repository 作者")
	require.NoError(t, dao.NewFollow(f.db).Insert(f.ctx, reader, author))
	f.createPublishedArticle(t, author, "较早文章", 100)
	newID := f.createPublishedArticle(t, author, "较新文章", 200)
	repo := NewArticleRepositoryImpl(dao.NewGORMArticleDAO(f.db), nil, &logger.NopLogger{})
	items, err := repo.Feed(f.ctx, reader, 0, 1)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, newID, items[0].Id)
	assert.Equal(t, author, items[0].Author.Id)
	assert.Equal(t, "Repository 作者", items[0].Author.Nickname)
}

func TestMySQLArticleRepositoryAuthorPagination(t *testing.T) {
	f := newRepositoryMySQLFixture(t)
	author := f.createUser(t, "作者分页映射")
	oldID := f.createPublishedArticle(t, author, "作者旧文", 100)
	f.createPublishedArticle(t, author, "作者新文", 200)
	repo := NewArticleRepositoryImpl(dao.NewGORMArticleDAO(f.db), nil, &logger.NopLogger{})
	items, err := repo.PubListByAuthor(f.ctx, author, 1, 1)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, oldID, items[0].Id)
	count, err := repo.CountPublishedByAuthor(f.ctx, author)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestMySQLFollowRepositoryPaginationMapping(t *testing.T) {
	f := newRepositoryMySQLFixture(t)
	follower := f.createUser(t, "Repository 关注者")
	oldTarget := f.createUser(t, "较早关注")
	newTarget := f.createUser(t, "较新关注")
	now := time.Now().UnixMilli()
	require.NoError(
		t,
		f.db.Create(
			&dao.Follow{
				FollowerId:  follower,
				FollowingId: oldTarget,
				CreatedAt:   now - 100,
				UpdatedAt:   now - 100,
			},
		).Error,
	)
	require.NoError(
		t,
		f.db.Create(
			&dao.Follow{
				FollowerId:  follower,
				FollowingId: newTarget,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		).Error,
	)
	repo := NewFollowRepository(dao.NewFollow(f.db))
	items, err := repo.ListFollowingPage(f.ctx, follower, 1, 1)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, oldTarget, items[0].FollowingId)
	assert.Equal(t, "较早关注", items[0].Nickname)
	count, err := repo.CountFollowing(f.ctx, follower)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestMySQLInteractiveRepositoryCollectionAndBatchMapping(t *testing.T) {
	f := newRepositoryMySQLFixture(t)
	uid := f.createUser(t, "Repository 收藏用户")
	author := f.createUser(t, "Repository 收藏作者")
	articleID := f.createPublishedArticle(t, author, "收藏映射文章", 100)
	require.NoError(t, f.db.Create(&dao.UserCollectionBiz{
		Uid: uid, BizId: articleID, Biz: "article", Status: 1, CreatedAt: 100, UpdatedAt: 100,
	}).Error)
	require.NoError(t, f.db.Create(&dao.Interactive{
		Biz: "article", BizId: articleID, ReadCnt: 8, LikeCnt: 2, CollectCnt: 1, CreatedAt: 100, UpdatedAt: 100,
	}).Error)
	repo := NewInteractiveRepository(dao.NewInteractiveDAO(f.db))
	collected, err := repo.ListCollected(f.ctx, "article", uid, 0, 10)
	require.NoError(t, err)
	require.Len(t, collected, 1)
	assert.Equal(t, "Repository 收藏作者", collected[0].Author.Nickname)
	counts, err := repo.BatchGet(f.ctx, "article", []int64{articleID})
	require.NoError(t, err)
	assert.Equal(t, int64(8), counts[articleID].ReadCnt)
	assert.Equal(t, int64(2), counts[articleID].LikeCnt)
}
