package ioc

import (
	"context"
	"database/sql"
	"followingfeed/internal/repository/dao"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestQueryTimeoutPluginUsesPublicGORMPipeline(t *testing.T) {
	testCases := []struct {
		name          string
		parentTimeout time.Duration
		queryTimeout  time.Duration
	}{
		{name: "增加默认查询超时", queryTimeout: 50 * time.Millisecond},
		{name: "保留更短的上游超时", parentTimeout: 20 * time.Millisecond, queryTimeout: time.Second},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db, sqlDB, mock := newQueryTimeoutTestDB(t)
			t.Cleanup(func() { _ = sqlDB.Close() })
			plugin := queryTimeoutPlugin{timeout: tc.queryTimeout}
			require.NoError(t, db.Use(plugin))

			var observed context.Context
			require.NoError(t, db.Callback().Query().
				After(plugin.beforeName("query")).
				Before("gorm:query").
				Register("test:observe_query_timeout", func(tx *gorm.DB) {
					observed = tx.Statement.Context
				}))

			parent := context.Background()
			if tc.parentTimeout > 0 {
				var cancel context.CancelFunc
				parent, cancel = context.WithTimeout(parent, tc.parentTimeout)
				defer cancel()
			}
			parentDeadline, parentHasDeadline := parent.Deadline()
			started := time.Now()
			mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `timeout_probe`")).
				WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

			var rows []struct{ ID int }
			require.NoError(t, db.WithContext(parent).Table("timeout_probe").Find(&rows).Error)
			require.NotNil(t, observed)
			deadline, ok := observed.Deadline()
			require.True(t, ok)
			if parentHasDeadline {
				assert.Equal(t, parentDeadline, deadline)
			} else {
				assert.WithinDuration(t, started.Add(tc.queryTimeout), deadline, 20*time.Millisecond)
			}
			assert.ErrorIs(t, observed.Err(), context.Canceled)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestQueryTimeoutPluginRejectsInvalidDuration(t *testing.T) {
	db, sqlDB, _ := newQueryTimeoutTestDB(t)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.EqualError(t, db.Use(queryTimeoutPlugin{}), "查询超时必须大于 0")
}

func newQueryTimeoutTestDB(t *testing.T) (*gorm.DB, *sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DisableAutomaticPing: true})
	require.NoError(t, err)
	return db, sqlDB, mock
}

// 直接调用业务 DAO，防止查询改用 Scan/Rows 后绕过超时回调。
func TestArticleReadQueriesKeepTimeout(t *testing.T) {
	for _, name := range []string{"feed", "status_counts"} {
		t.Run(name, func(t *testing.T) {
			db, sqlDB, mock := newQueryTimeoutTestDB(t)
			t.Cleanup(func() { _ = sqlDB.Close() })
			plugin := queryTimeoutPlugin{timeout: 50 * time.Millisecond}
			require.NoError(t, db.Use(plugin))
			var observed context.Context
			require.NoError(t, db.Callback().Query().After(plugin.beforeName("query")).Before("gorm:query").Register("test:article_deadline", func(tx *gorm.DB) {
				observed = tx.Statement.Context
			}))
			mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}))
			d := dao.NewGORMArticleDAO(db)
			var err error
			if name == "feed" {
				_, err = d.GetFeed(context.Background(), 1, 0, 10)
			} else {
				_, err = d.CountByAuthorStatus(context.Background(), 1)
			}
			require.NoError(t, err)
			require.NotNil(t, observed, "业务查询必须经过超时插件")
			_, hasDeadline := observed.Deadline()
			require.True(t, hasDeadline)
			assert.ErrorIs(t, observed.Err(), context.Canceled)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
