package dao

import (
	"context"
	"database/sql"
	"errors"
	"followingfeed/internal/domain"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gormMysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func interactiveTestDB(t *testing.T, db *sql.DB) *gorm.DB {
	gdb, err := gorm.Open(gormMysql.New(gormMysql.Config{
		Conn: db, SkipInitializeWithVersion: true,
	}), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	return gdb
}

func expectPublishedInteractiveTarget(mock sqlmock.Sqlmock, articleID int64) {
	mock.ExpectQuery("SELECT `id` FROM `publish_articles` WHERE id = \\? AND status = \\? AND deleted_at = 0.*FOR SHARE").
		WithArgs(articleID, domain.ArticleStatusPublished.ToUint8(), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(articleID))
}

func TestInteractiveDAOInsertLikeInfo(t *testing.T) {
	testCases := []struct {
		name       string
		inserted   int64
		restored   int64
		wantChange bool
	}{
		{name: "首次点赞写入关系并增加聚合计数", inserted: 1, wantChange: true},
		{name: "重复点赞未改变关系也不增加计数"},
		{name: "恢复已取消点赞并增加聚合计数", restored: 1, wantChange: true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			mock.ExpectBegin()
			expectPublishedInteractiveTarget(mock, 2)
			mock.ExpectExec("INSERT INTO `user_like_bizs`").
				WillReturnResult(sqlmock.NewResult(tc.inserted, tc.inserted))
			if tc.inserted == 0 {
				mock.ExpectExec("UPDATE `user_like_bizs`").
					WillReturnResult(sqlmock.NewResult(0, tc.restored))
			}
			if tc.wantChange {
				mock.ExpectExec("INSERT INTO `interactives`").
					WillReturnResult(sqlmock.NewResult(1, 1))
			}
			mock.ExpectCommit()
			changed, err := NewInteractiveDAO(
				interactiveTestDB(t, db),
			).InsertLikeInfo(context.Background(), "article", 2, 1)
			assert.NoError(t, err)
			assert.Equal(t, tc.wantChange, changed)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestInteractiveDAOInsertLikeRejectsUnavailableArticle(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT `id` FROM `publish_articles`").
		WithArgs(int64(2), domain.ArticleStatusPublished.ToUint8(), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectRollback()
	changed, err := NewInteractiveDAO(
		interactiveTestDB(t, db),
	).InsertLikeInfo(context.Background(), "article", 2, 1)
	assert.False(t, changed)
	assert.ErrorIs(t, err, ErrInteractiveTargetNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInteractiveDAODeleteLikeInfo(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `user_like_bizs`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `interactives`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	changed, err := NewInteractiveDAO(interactiveTestDB(t, db)).
		DeleteLikeInfo(context.Background(), "article", 2, 1)
	assert.NoError(t, err)
	assert.True(t, changed)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInteractiveDAOGetNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery("SELECT .* FROM `interactives`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "biz_id", "biz", "like_cnt"}))
	_, err = NewInteractiveDAO(interactiveTestDB(t, db)).Get(context.Background(), "article", 2)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInteractiveDAOBatchGet(t *testing.T) {
	databaseErr := errors.New("database unavailable")
	testCases := []struct {
		name     string
		ids      []int64
		queryErr error
		wantLen  int
	}{
		{name: "批量查询并映射全部互动字段", ids: []int64{2, 3}, wantLen: 2},
		{name: "空ID列表直接返回空结果"},
		{name: "数据库异常", ids: []int64{2, 3}, queryErr: databaseErr},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			if len(tc.ids) > 0 {
				expect := mock.ExpectQuery("SELECT .* FROM `interactives` WHERE biz = \\? AND biz_id IN \\(\\?,\\?\\)").
					WithArgs("article", int64(2), int64(3))
				if tc.queryErr != nil {
					expect.WillReturnError(tc.queryErr)
				} else {
					expect.WillReturnRows(sqlmock.NewRows([]string{"id", "biz_id", "biz", "read_cnt", "like_cnt", "collect_cnt", "created_at", "updated_at"}).
						AddRow(1, 2, "article", 10, 3, 4, 100, 200).
						AddRow(2, 3, "article", 20, 5, 6, 101, 201))
				}
			}
			items, err := NewInteractiveDAO(
				interactiveTestDB(t, db),
			).BatchGet(context.Background(), "article", tc.ids)
			assert.Len(t, items, tc.wantLen)
			assert.ErrorIs(t, err, tc.queryErr)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestInteractiveDAOLikedError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	wantErr := errors.New("database unavailable")
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_like_bizs`").WillReturnError(wantErr)
	liked, err := NewInteractiveDAO(interactiveTestDB(t, db)).
		Liked(context.Background(), "article", 2, 1)
	assert.False(t, liked)
	assert.ErrorIs(t, err, wantErr)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInteractiveDAOIncrRead(t *testing.T) {
	testCases := []struct {
		name    string
		execErr error
	}{
		{name: "增加成功"},
		{name: "增加失败", execErr: errors.New("database unavailable")},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			expect := mock.ExpectExec("INSERT INTO `interactives`")
			if tc.execErr == nil {
				expect.WillReturnResult(sqlmock.NewResult(1, 1))
			} else {
				expect.WillReturnError(tc.execErr)
			}
			err = NewInteractiveDAO(
				interactiveTestDB(t, db),
			).IncrRead(context.Background(), "article", 2)
			assert.ErrorIs(t, err, tc.execErr)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestInteractiveDAOInsertCollectInfo(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectBegin()
	expectPublishedInteractiveTarget(mock, 2)
	mock.ExpectExec("INSERT INTO `user_collection_bizs`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO `interactives`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	changed, err := NewInteractiveDAO(
		interactiveTestDB(t, db),
	).InsertCollectInfo(context.Background(), "article", 2, 1)
	assert.NoError(t, err)
	assert.True(t, changed)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInteractiveDAODeleteCollectInfo(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `user_collection_bizs`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `interactives`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	changed, err := NewInteractiveDAO(
		interactiveTestDB(t, db),
	).DeleteCollectInfo(context.Background(), "article", 2, 1)
	assert.NoError(t, err)
	assert.True(t, changed)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInteractiveDAOCollected(t *testing.T) {
	testCases := []struct {
		name     string
		count    int64
		queryErr error
		want     bool
	}{
		{name: "已收藏", count: 1, want: true},
		{name: "未收藏"},
		{name: "查询失败", queryErr: errors.New("database unavailable")},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			expect := mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_collection_bizs`")
			if tc.queryErr == nil {
				expect.WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(tc.count))
			} else {
				expect.WillReturnError(tc.queryErr)
			}
			got, err := NewInteractiveDAO(
				interactiveTestDB(t, db),
			).Collected(context.Background(), "article", 2, 1)
			assert.Equal(t, tc.want, got)
			assert.ErrorIs(t, err, tc.queryErr)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestInteractiveDAOCountCollected(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `publish_articles` JOIN user_collection_bizs").
		WithArgs("article", int64(1), 2).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(6))
	got, err := NewInteractiveDAO(
		interactiveTestDB(t, db),
	).CountCollected(context.Background(), "article", 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(6), got)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInteractiveDAOListCollectedUsesStableOrder(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery("SELECT .* FROM `publish_articles` JOIN user_collection_bizs.*ORDER BY user_collection_bizs.updated_at DESC, user_collection_bizs.id DESC LIMIT \\?").
		WithArgs("article", int64(1), 2, 10).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "content", "author_id", "status", "created_at", "updated_at", "deleted_at", "author_nickname",
		}))

	items, err := NewInteractiveDAO(interactiveTestDB(t, db)).
		ListCollected(context.Background(), "article", 1, 0, 10)
	require.NoError(t, err)
	assert.Empty(t, items)
	assert.NoError(t, mock.ExpectationsWereMet())
}
