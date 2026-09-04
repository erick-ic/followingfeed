//go:build integration

package dao

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"followingfeed/config"
	"followingfeed/internal/domain"
	"followingfeed/migrations"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const integrationBiz = "integration_article"

type mysqlFixture struct {
	db      *gorm.DB
	sqlDB   *sql.DB
	ctx     context.Context
	userIDs []int64
	artIDs  []int64
}

func newMySQLFixture(t *testing.T) *mysqlFixture {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../.."))
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

	f := &mysqlFixture{db: db, sqlDB: sqlDB, ctx: context.Background()}
	t.Cleanup(func() { f.cleanup(t) })
	return f
}

func (f *mysqlFixture) createUser(t *testing.T, nickname string) int64 {
	t.Helper()
	email := fmt.Sprintf("integration-%s@example.com", uuid.NewString())
	require.NoError(t, NewGORMUserDAO(f.db).Insert(f.ctx, domain.User{
		Nickname: nickname,
		Email:    email,
		Password: "integration-hash",
	}))
	user, err := NewGORMUserDAO(f.db).FindByEmail(f.ctx, email)
	require.NoError(t, err)
	id := int64(user.Id)
	f.userIDs = append(f.userIDs, id)
	return id
}

func (f *mysqlFixture) createPublishedArticle(
	t *testing.T,
	authorID int64,
	title string,
	updatedAt int64,
) int64 {
	t.Helper()
	now := time.Now().UnixMilli()
	article := Article{Title: title, Content: "集成测试正文", AuthorId: authorID,
		Status: domain.ArticleStatusPublished.ToUint8(), CreatedAt: now, UpdatedAt: updatedAt}
	require.NoError(t, f.db.Create(&article).Error)
	f.artIDs = append(f.artIDs, article.Id)
	published := PublishArticle(article)
	require.NoError(t, f.db.Create(&published).Error)
	return article.Id
}

func (f *mysqlFixture) cleanup(t *testing.T) {
	t.Helper()
	defer func() { require.NoError(t, f.sqlDB.Close()) }()
	if len(f.artIDs) > 0 {
		require.NoError(t, f.db.Where("biz_id IN ?", f.artIDs).Delete(&UserLikeBiz{}).Error)
		require.NoError(t, f.db.Where("biz_id IN ?", f.artIDs).Delete(&UserCollectionBiz{}).Error)
		require.NoError(t, f.db.Where("biz_id IN ?", f.artIDs).Delete(&Interactive{}).Error)
		require.NoError(t, f.db.Where("id IN ?", f.artIDs).Delete(&PublishArticle{}).Error)
		require.NoError(t, f.db.Where("id IN ?", f.artIDs).Delete(&Article{}).Error)
	}
	if len(f.userIDs) > 0 {
		require.NoError(
			t,
			f.db.Where("follower_id IN ? OR following_id IN ?", f.userIDs, f.userIDs).
				Delete(&Follow{}).
				Error,
		)
		require.NoError(t, f.db.Where("id IN ?", f.userIDs).Delete(&User{}).Error)
	}
}

func TestMySQLUserDAOInsertAndUniqueEmail(t *testing.T) {
	f := newMySQLFixture(t)
	email := fmt.Sprintf("integration-%s@example.com", uuid.NewString())
	d := NewGORMUserDAO(f.db)
	require.NoError(
		t,
		d.Insert(f.ctx, domain.User{Nickname: "云端旅人", Email: email, Password: "hash"}),
	)
	user, err := d.FindByEmail(f.ctx, email)
	require.NoError(t, err)
	f.userIDs = append(f.userIDs, int64(user.Id))
	assert.Equal(t, "云端旅人", user.Nickname)
	assert.ErrorIs(
		t,
		d.Insert(f.ctx, domain.User{Nickname: "重复用户", Email: email, Password: "hash"}),
		ErrUserDuplicated,
	)
}

func TestMySQLArticleDAOSyncPublishTransaction(t *testing.T) {
	f := newMySQLFixture(t)
	uid := f.createUser(t, "发布作者")
	d := NewGORMArticleDAO(f.db)
	id, err := d.Sync(
		f.ctx,
		Article{
			Title:    "事务发布",
			Content:  "正文",
			AuthorId: uid,
			Status:   domain.ArticleStatusPublished.ToUint8(),
		},
	)
	require.NoError(t, err)
	f.artIDs = append(f.artIDs, id)
	draft, err := d.GetById(f.ctx, id, uid)
	require.NoError(t, err)
	published, err := d.GetByPubId(f.ctx, id)
	require.NoError(t, err)
	assert.Equal(t, draft.Title, published.Title)
	assert.Equal(t, draft.AuthorId, published.AuthorId)
}

func TestMySQLArticleDAORetractTransaction(t *testing.T) {
	f := newMySQLFixture(t)
	uid := f.createUser(t, "撤回作者")
	id := f.createPublishedArticle(t, uid, "待撤回文章", time.Now().UnixMilli())
	_, err := NewGORMArticleDAO(f.db).SyncStatus(f.ctx, id, uid, domain.ArticleStatusUnPublished)
	require.NoError(t, err)
	var draft Article
	var published PublishArticle
	require.NoError(t, f.db.First(&draft, id).Error)
	require.NoError(t, f.db.First(&published, id).Error)
	assert.Equal(t, domain.ArticleStatusUnPublished.ToUint8(), draft.Status)
	assert.Equal(t, domain.ArticleStatusUnPublished.ToUint8(), published.Status)
}

func TestMySQLArticleDAOSoftDeleteTransaction(t *testing.T) {
	f := newMySQLFixture(t)
	uid := f.createUser(t, "删除作者")
	id := f.createPublishedArticle(t, uid, "待删除文章", time.Now().UnixMilli())
	_, err := NewGORMArticleDAO(f.db).SoftDelete(f.ctx, id, uid)
	require.NoError(t, err)
	var draft Article
	var published PublishArticle
	require.NoError(t, f.db.First(&draft, id).Error)
	require.NoError(t, f.db.First(&published, id).Error)
	assert.NotZero(t, draft.DeletedAt)
	assert.NotZero(t, published.DeletedAt)
	_, err = NewGORMArticleDAO(f.db).GetById(f.ctx, id, uid)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestMySQLFollowDAOFollowDuplicateAndUnfollow(t *testing.T) {
	f := newMySQLFixture(t)
	follower := f.createUser(t, "关注者")
	target := f.createUser(t, "被关注者")
	d := NewFollow(f.db)
	require.NoError(t, d.Insert(f.ctx, follower, target))
	assert.ErrorIs(t, d.Insert(f.ctx, follower, target), ErrFollowDuplicated)
	following, err := d.IsFollowing(f.ctx, follower, target)
	require.NoError(t, err)
	assert.True(t, following)
	require.NoError(t, d.UnFollow(f.ctx, follower, target))
	following, err = d.IsFollowing(f.ctx, follower, target)
	require.NoError(t, err)
	assert.False(t, following)
}

func TestMySQLInteractiveDAOLikeIdempotency(t *testing.T) {
	f := newMySQLFixture(t)
	uid := f.createUser(t, "点赞用户")
	bizID := time.Now().UnixNano()
	f.artIDs = append(f.artIDs, bizID)
	d := NewInteractiveDAO(f.db)
	changed, err := d.InsertLikeInfo(f.ctx, integrationBiz, bizID, uid)
	require.NoError(t, err)
	assert.True(t, changed)
	changed, err = d.InsertLikeInfo(f.ctx, integrationBiz, bizID, uid)
	require.NoError(t, err)
	assert.False(t, changed)
	inter, err := d.Get(f.ctx, integrationBiz, bizID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), inter.LikeCnt)
	changed, err = d.DeleteLikeInfo(f.ctx, integrationBiz, bizID, uid)
	require.NoError(t, err)
	assert.True(t, changed)
	changed, err = d.DeleteLikeInfo(f.ctx, integrationBiz, bizID, uid)
	require.NoError(t, err)
	assert.False(t, changed)
}

func TestMySQLInteractiveDAOCollectionIdempotency(t *testing.T) {
	f := newMySQLFixture(t)
	uid := f.createUser(t, "收藏用户")
	bizID := time.Now().UnixNano()
	f.artIDs = append(f.artIDs, bizID)
	d := NewInteractiveDAO(f.db)
	changed, err := d.InsertCollectInfo(f.ctx, integrationBiz, bizID, uid)
	require.NoError(t, err)
	assert.True(t, changed)
	changed, err = d.InsertCollectInfo(f.ctx, integrationBiz, bizID, uid)
	require.NoError(t, err)
	assert.False(t, changed)
	inter, err := d.Get(f.ctx, integrationBiz, bizID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), inter.CollectCnt)
}

func TestMySQLArticleDAOFeedPagination(t *testing.T) {
	f := newMySQLFixture(t)
	reader := f.createUser(t, "订阅读者")
	author := f.createUser(t, "订阅作者")
	require.NoError(t, NewFollow(f.db).Insert(f.ctx, reader, author))
	first := f.createPublishedArticle(t, author, "较早文章", 100)
	second := f.createPublishedArticle(t, author, "较新文章", 200)
	items, err := NewGORMArticleDAO(f.db).GetFeed(f.ctx, reader, 1, 1)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, first, items[0].Id)
	assert.NotEqual(t, second, items[0].Id)
}

func TestMySQLArticleDAOAuthorPagination(t *testing.T) {
	f := newMySQLFixture(t)
	author := f.createUser(t, "分页作者")
	first := f.createPublishedArticle(t, author, "作者旧文", 100)
	f.createPublishedArticle(t, author, "作者新文", 200)
	items, err := NewGORMArticleDAO(f.db).GetPublishedByAuthor(f.ctx, author, 1, 1)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, first, items[0].Id)
}

func TestMySQLInteractiveDAOCollectedPagination(t *testing.T) {
	f := newMySQLFixture(t)
	uid := f.createUser(t, "收藏分页用户")
	author := f.createUser(t, "收藏文章作者")
	oldID := f.createPublishedArticle(t, author, "较早收藏", 100)
	newID := f.createPublishedArticle(t, author, "较新收藏", 200)
	d := NewInteractiveDAO(f.db)
	require.NoError(
		t,
		f.db.Create(
			&UserCollectionBiz{
				Uid:       uid,
				BizId:     oldID,
				Biz:       "article",
				Status:    1,
				CreatedAt: 100,
				UpdatedAt: 100,
			},
		).Error,
	)
	require.NoError(
		t,
		f.db.Create(
			&UserCollectionBiz{
				Uid:       uid,
				BizId:     newID,
				Biz:       "article",
				Status:    1,
				CreatedAt: 200,
				UpdatedAt: 200,
			},
		).Error,
	)
	items, err := d.ListCollected(f.ctx, "article", uid, 1, 1)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, oldID, items[0].Id)
	count, err := d.CountCollected(f.ctx, "article", uid)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestMySQLInteractiveDAOBatchAggregation(t *testing.T) {
	f := newMySQLFixture(t)
	firstID := time.Now().UnixNano()
	secondID := firstID + 1
	f.artIDs = append(f.artIDs, firstID, secondID)
	require.NoError(t, f.db.Create(&[]Interactive{
		{
			Biz:        integrationBiz,
			BizId:      firstID,
			ReadCnt:    10,
			LikeCnt:    2,
			CollectCnt: 1,
			CreatedAt:  1,
			UpdatedAt:  1,
		},
		{
			Biz:        integrationBiz,
			BizId:      secondID,
			ReadCnt:    20,
			LikeCnt:    3,
			CollectCnt: 4,
			CreatedAt:  1,
			UpdatedAt:  1,
		},
	}).Error)
	items, err := NewInteractiveDAO(
		f.db,
	).BatchGet(f.ctx, integrationBiz, []int64{firstID, secondID})
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, int64(10), items[0].ReadCnt)
	assert.Equal(t, int64(4), items[1].CollectCnt)
}

// TestMySQLArticleDAOConcurrentPublishAndRetract 验证对同一篇文章的并发发布和撤回
// 会按固定的 articles -> publish_articles 加锁顺序串行化，且两张表的最终状态一致。
func TestMySQLArticleDAOConcurrentPublishAndRetract(t *testing.T) {
	f := newMySQLFixture(t)
	authorID := f.createUser(t, "并发发布作者")
	articleID := f.createPublishedArticle(t, authorID, "并发文章", time.Now().UnixMilli())
	d := NewGORMArticleDAO(f.db)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	const operations = 12
	runConcurrently(t, operations*2, func(index int) error {
		if index%2 == 0 {
			_, err := d.Sync(ctx, Article{
				Id: articleID, Title: "并发文章", Content: "发布内容",
				AuthorId: authorID, Status: domain.ArticleStatusPublished.ToUint8(),
			})
			return err
		}
		_, err := d.SyncStatus(ctx, articleID, authorID, domain.ArticleStatusUnPublished)
		return err
	})

	var draft Article
	var published PublishArticle
	require.NoError(t, f.db.First(&draft, articleID).Error)
	require.NoError(t, f.db.First(&published, articleID).Error)
	assert.Equal(t, draft.Status, published.Status)
}

// TestMySQLInteractiveDAOConcurrentLikeAndUnlike 使用不同用户并发更新同一聚合行，
// 以真实 InnoDB 锁竞争验证不会遗失计数，也不会在未处理的死锁上通过测试。
func TestMySQLInteractiveDAOConcurrentLikeAndUnlike(t *testing.T) {
	f := newMySQLFixture(t)
	authorID := f.createUser(t, "并发互动作者")
	articleID := f.createPublishedArticle(t, authorID, "并发互动文章", time.Now().UnixMilli())
	const users = 12
	userIDs := make([]int64, users)
	for index := range userIDs {
		userIDs[index] = f.createUser(t, fmt.Sprintf("并发互动用户-%d", index))
	}

	d := NewInteractiveDAO(f.db)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	runConcurrently(t, users, func(index int) error {
		changed, err := d.InsertLikeInfo(ctx, "article", articleID, userIDs[index])
		if err == nil && !changed {
			return fmt.Errorf("用户 %d 首次点赞未改变状态", userIDs[index])
		}
		return err
	})
	inter, err := d.Get(ctx, "article", articleID)
	require.NoError(t, err)
	assert.Equal(t, int64(users), inter.LikeCnt)

	runConcurrently(t, users, func(index int) error {
		changed, err := d.DeleteLikeInfo(ctx, "article", articleID, userIDs[index])
		if err == nil && !changed {
			return fmt.Errorf("用户 %d 取消点赞未改变状态", userIDs[index])
		}
		return err
	})
	inter, err = d.Get(ctx, "article", articleID)
	require.NoError(t, err)
	assert.Zero(t, inter.LikeCnt)
}

func runConcurrently(t *testing.T, count int, operation func(int) error) {
	t.Helper()
	start := make(chan struct{})
	errs := make(chan error, count)
	var wg sync.WaitGroup
	wg.Add(count)
	for index := 0; index < count; index++ {
		go func() {
			defer wg.Done()
			<-start
			errs <- operation(index)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
}
