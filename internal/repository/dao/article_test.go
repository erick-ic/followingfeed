package dao

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"followingfeed/internal/domain"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gormMysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func newArticleTestDB(t *testing.T, db *sql.DB) *gorm.DB {
	gormDB, err := gorm.Open(
		gormMysql.New(gormMysql.Config{
			Conn:                      db,
			SkipInitializeWithVersion: true,
		}),
		&gorm.Config{SkipDefaultTransaction: true},
	)
	require.NoError(t, err)
	return gormDB
}

func TestGORMArticleDAOInsert(t *testing.T) {
	testCases := []struct {
		name    string
		mock    func(*testing.T) *sql.DB
		wantID  int64
		wantErr error
	}{
		{
			name: "插入成功",
			mock: func(t *testing.T) *sql.DB {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				mock.ExpectExec("INSERT INTO `articles`").
					WillReturnResult(sqlmock.NewResult(6, 1))
				return db
			},
			wantID: 6,
		},
		{
			name: "数据库异常",
			mock: func(t *testing.T) *sql.DB {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				mock.ExpectExec("INSERT INTO `articles`").
					WillReturnError(errors.New("database unavailable"))
				return db
			},
			wantErr: errors.New("database unavailable"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := tc.mock(t)
			defer db.Close()
			id, err := NewGORMArticleDAO(newArticleTestDB(t, db)).Insert(
				context.Background(),
				Article{Title: "标题", Content: "正文", AuthorId: 1, Status: 2},
			)
			assert.Equal(t, tc.wantID, id)
			if tc.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.wantErr.Error())
			}
		})
	}
}

func TestGORMArticleDAOGetById(t *testing.T) {
	testCases := []struct {
		name    string
		mock    func(*testing.T) *sql.DB
		want    Article
		wantErr error
	}{
		{
			name: "查询成功",
			mock: func(t *testing.T) *sql.DB {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				mock.ExpectQuery("SELECT .* FROM `articles`").
					WillReturnRows(articleRows().AddRow(1, "标题", []byte("正文"), 2, 2, 100, 200, 0))
				return db
			},
			want: Article{
				Id: 1, Title: "标题", Content: "正文", AuthorId: 2,
				Status: 2, CreatedAt: 100, UpdatedAt: 200,
			},
		},
		{
			name: "查询失败",
			mock: func(t *testing.T) *sql.DB {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				mock.ExpectQuery("SELECT .* FROM `articles`").
					WillReturnError(errors.New("database unavailable"))
				return db
			},
			wantErr: errors.New("database unavailable"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := tc.mock(t)
			defer db.Close()
			got, err := NewGORMArticleDAO(newArticleTestDB(t, db)).
				GetById(context.Background(), 1)
			assert.Equal(t, tc.want, got)
			if tc.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.wantErr.Error())
			}
		})
	}
}

func TestGORMArticleDAOGetByPubId(t *testing.T) {
	testCases := []struct {
		name    string
		mock    func(*testing.T) *sql.DB
		want    PublishArticle
		wantErr error
	}{
		{
			name: "查询成功",
			mock: func(t *testing.T) *sql.DB {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				mock.ExpectQuery("SELECT .* FROM `publish_articles`").
					WillReturnRows(publishedArticleRows().
						AddRow(1, "标题", []byte("正文"), 2, 2, 100, 200, 0, "云端旅人"))
				return db
			},
			want: PublishArticle{
				Id: 1, Title: "标题", Content: "正文", AuthorId: 2,
				AuthorNickname: "云端旅人", Status: 2, CreatedAt: 100, UpdatedAt: 200,
			},
		},
		{
			name: "查询失败",
			mock: func(t *testing.T) *sql.DB {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				mock.ExpectQuery("SELECT .* FROM `publish_articles`").
					WillReturnError(gorm.ErrRecordNotFound)
				return db
			},
			wantErr: gorm.ErrRecordNotFound,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := tc.mock(t)
			defer db.Close()
			got, err := NewGORMArticleDAO(newArticleTestDB(t, db)).
				GetByPubId(context.Background(), 1)
			assert.Equal(t, tc.want, got)
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestGORMArticleDAOGetByAuthor(t *testing.T) {
	testCases := []struct {
		name    string
		mock    func(*testing.T) *sql.DB
		want    []Article
		wantErr error
	}{
		{
			name: "查询成功",
			mock: func(t *testing.T) *sql.DB {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				mock.ExpectQuery("SELECT .* FROM `articles`").
					WillReturnRows(articleRows().AddRow(1, "标题", []byte("正文"), 1, 2, 100, 200, 0))
				return db
			},
			want: []Article{{
				Id: 1, Title: "标题", Content: "正文", AuthorId: 1,
				Status: 2, CreatedAt: 100, UpdatedAt: 200,
			}},
		},
		{
			name: "查询失败",
			mock: func(t *testing.T) *sql.DB {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				mock.ExpectQuery("SELECT .* FROM `articles`").
					WillReturnError(errors.New("database unavailable"))
				return db
			},
			wantErr: errors.New("database unavailable"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := tc.mock(t)
			defer db.Close()
			got, err := NewGORMArticleDAO(newArticleTestDB(t, db)).
				GetByAuthor(context.Background(), 1, 0, 10)
			assert.Equal(t, tc.want, got)
			if tc.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.wantErr.Error())
			}
		})
	}
}

func TestGORMArticleDAOGetPublished(t *testing.T) {
	testCases := []struct {
		name    string
		mock    func(*testing.T) *sql.DB
		want    []PublishArticle
		wantErr error
	}{
		{
			name: "查询成功",
			mock: func(t *testing.T) *sql.DB {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				mock.ExpectQuery("SELECT .* FROM `publish_articles`").
					WillReturnRows(publishedArticleRows().
						AddRow(1, "标题", []byte("正文"), 1, 2, 100, 200, 0, "云端旅人"))
				return db
			},
			want: []PublishArticle{{
				Id: 1, Title: "标题", Content: "正文", AuthorId: 1,
				AuthorNickname: "云端旅人", Status: 2, CreatedAt: 100, UpdatedAt: 200,
			}},
		},
		{
			name: "查询失败",
			mock: func(t *testing.T) *sql.DB {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				mock.ExpectQuery("SELECT .* FROM `publish_articles`").
					WillReturnError(errors.New("database unavailable"))
				return db
			},
			wantErr: errors.New("database unavailable"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := tc.mock(t)
			defer db.Close()
			got, err := NewGORMArticleDAO(newArticleTestDB(t, db)).
				GetPublished(context.Background(), 0, 10)
			assert.Equal(t, tc.want, got)
			if tc.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.wantErr.Error())
			}
		})
	}
}

func TestGORMArticleDAOUpdateByArticleId(t *testing.T) {
	testCases := []struct {
		name    string
		mock    func(*testing.T) *sql.DB
		wantErr bool
	}{
		{
			name: "更新成功",
			mock: func(t *testing.T) *sql.DB {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				mock.ExpectExec("UPDATE `articles` SET").
					WillReturnResult(sqlmock.NewResult(0, 1))
				return db
			},
		},
		{
			name: "文章不存在或作者非法",
			mock: func(t *testing.T) *sql.DB {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				mock.ExpectExec("UPDATE `articles` SET").
					WillReturnResult(sqlmock.NewResult(0, 0))
				return db
			},
			wantErr: true,
		},
		{
			name: "数据库异常",
			mock: func(t *testing.T) *sql.DB {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				mock.ExpectExec("UPDATE `articles` SET").
					WillReturnError(errors.New("database unavailable"))
				return db
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := tc.mock(t)
			defer db.Close()
			err := NewGORMArticleDAO(newArticleTestDB(t, db)).UpdateByArticleId(
				context.Background(),
				Article{Id: 1, Title: "标题", AuthorId: 2, Status: 1},
			)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGORMArticleDAOSyncStatus(t *testing.T) {
	testCases := []struct {
		name    string
		mock    func(*testing.T) *sql.DB
		wantErr bool
	}{
		{
			name: "同步成功",
			mock: func(t *testing.T) *sql.DB {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				mock.ExpectBegin()
				mock.ExpectExec("UPDATE `articles` SET").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec("UPDATE `publish_articles` SET").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
				return db
			},
		},
		{
			name: "作者校验失败",
			mock: func(t *testing.T) *sql.DB {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				mock.ExpectBegin()
				mock.ExpectExec("UPDATE `articles` SET").WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectRollback()
				return db
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := tc.mock(t)
			defer db.Close()
			id, err := NewGORMArticleDAO(newArticleTestDB(t, db)).SyncStatus(
				context.Background(), 1, 2, domain.ArticleStatusUnPublished,
			)
			assert.Equal(t, int64(1), id)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGORMArticleDAOSoftDelete(t *testing.T) {
	testCases := []struct {
		name    string
		mock    func(*testing.T) *sql.DB
		wantErr bool
	}{
		{
			name: "删除成功",
			mock: func(t *testing.T) *sql.DB {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				mock.ExpectBegin()
				mock.ExpectExec("UPDATE `articles` SET").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec("UPDATE `publish_articles` SET").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
				return db
			},
		},
		{
			name: "文章不存在或无权限",
			mock: func(t *testing.T) *sql.DB {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				mock.ExpectBegin()
				mock.ExpectExec("UPDATE `articles` SET").WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectRollback()
				return db
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := tc.mock(t)
			defer db.Close()
			id, err := NewGORMArticleDAO(newArticleTestDB(t, db)).
				SoftDelete(context.Background(), 1, 2)
			assert.Equal(t, int64(1), id)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGORMArticleDAOUpsert(t *testing.T) {
	testCases := []struct {
		name    string
		mock    func(*testing.T) *sql.DB
		wantErr error
	}{
		{
			name: "同步成功",
			mock: func(t *testing.T) *sql.DB {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				mock.ExpectExec("INSERT INTO `publish_articles`").
					WillReturnResult(sqlmock.NewResult(1, 1))
				return db
			},
		},
		{
			name: "同步失败",
			mock: func(t *testing.T) *sql.DB {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				mock.ExpectExec("INSERT INTO `publish_articles`").
					WillReturnError(errors.New("database unavailable"))
				return db
			},
			wantErr: errors.New("database unavailable"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := tc.mock(t)
			defer db.Close()
			err := NewGORMArticleDAO(newArticleTestDB(t, db)).Upsert(
				context.Background(),
				PublishArticle{Id: 1, Title: "标题", AuthorId: 2, Status: 2},
			)
			if tc.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.wantErr.Error())
			}
		})
	}
}

func TestGORMArticleDAOSync(t *testing.T) {
	testCases := []struct {
		name    string
		article Article
		mock    func(*testing.T) *sql.DB
		wantID  int64
		wantErr bool
	}{
		{
			name:    "新文章同步成功",
			article: Article{Title: "标题", AuthorId: 2, Status: 2},
			mock: func(t *testing.T) *sql.DB {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				mock.ExpectBegin()
				mock.ExpectExec("INSERT INTO `articles`").
					WillReturnResult(sqlmock.NewResult(6, 1))
				mock.ExpectExec("INSERT INTO `publish_articles`").
					WillReturnResult(sqlmock.NewResult(6, 1))
				mock.ExpectCommit()
				return db
			},
			wantID: 6,
		},
		{
			name:    "已有文章同步成功",
			article: Article{Id: 7, Title: "标题", AuthorId: 2, Status: 2},
			mock: func(t *testing.T) *sql.DB {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				mock.ExpectBegin()
				mock.ExpectExec("UPDATE `articles` SET").
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec("INSERT INTO `publish_articles`").
					WillReturnResult(sqlmock.NewResult(7, 1))
				mock.ExpectCommit()
				return db
			},
			wantID: 7,
		},
		{
			name:    "制作库写入失败并回滚",
			article: Article{Title: "标题", AuthorId: 2, Status: 2},
			mock: func(t *testing.T) *sql.DB {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				mock.ExpectBegin()
				mock.ExpectExec("INSERT INTO `articles`").
					WillReturnError(errors.New("database unavailable"))
				mock.ExpectRollback()
				return db
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := tc.mock(t)
			defer db.Close()
			id, err := NewGORMArticleDAO(newArticleTestDB(t, db)).
				Sync(context.Background(), tc.article)
			assert.Equal(t, tc.wantID, id)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func articleRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "title", "content", "author_id", "status",
		"created_at", "updated_at", "deleted_at",
	})
}

func publishedArticleRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "title", "content", "author_id", "status",
		"created_at", "updated_at", "deleted_at", "author_nickname",
	})
}
