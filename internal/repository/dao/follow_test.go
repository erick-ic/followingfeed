package dao

import (
	"context"
	"database/sql"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gormMysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"regexp"
	"testing"
)

func followTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, *sql.DB) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	gdb, err := gorm.Open(
		gormMysql.New(gormMysql.Config{Conn: db, SkipInitializeWithVersion: true}),
		&gorm.Config{SkipDefaultTransaction: true},
	)
	require.NoError(t, err)
	return gdb, mock, db
}

func TestFollowDAOInsert(t *testing.T) {
	tests := []struct {
		name        string
		uid, target int64
		dbErr       error
		wantErr     error
	}{
		{
			name:    "唯一键冲突转换为重复关注错误",
			uid:     1,
			target:  2,
			dbErr:   &mysql.MySQLError{Number: 1062},
			wantErr: ErrFollowDuplicated,
		},
		{name: "插入成功", uid: 1, target: 2},
		{name: "不能关注自己", uid: 1, target: 1, wantErr: gorm.ErrInvalidData},
		{
			name:    "数据库异常",
			uid:     1,
			target:  2,
			dbErr:   errors.New("database unavailable"),
			wantErr: errors.New("database unavailable"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, sqlDB := followTestDB(t)
			defer sqlDB.Close()
			if tt.uid != tt.target {
				expect := mock.ExpectExec("INSERT INTO `follows`").
					WithArgs(tt.uid, tt.target, sqlmock.AnyArg(), sqlmock.AnyArg())
				if tt.dbErr != nil {
					expect.WillReturnError(tt.dbErr)
				} else {
					expect.WillReturnResult(sqlmock.NewResult(1, 1))
				}
			}
			err := NewFollow(db).Insert(context.Background(), tt.uid, tt.target)
			if tt.wantErr != nil {
				if tt.name == "数据库异常" {
					assert.EqualError(t, err, tt.wantErr.Error())
				} else {
					assert.ErrorIs(t, err, tt.wantErr)
				}
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestFollowDAOUnfollow(t *testing.T) {
	databaseErr := errors.New("database unavailable")
	for _, tt := range []struct {
		name  string
		dbErr error
	}{
		{name: "取消成功"},
		{name: "数据库异常", dbErr: databaseErr},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, sqlDB := followTestDB(t)
			defer sqlDB.Close()
			expect := mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `follows` WHERE follower_id = ? AND following_id = ?")).
				WithArgs(1, 2)
			if tt.dbErr != nil {
				expect.WillReturnError(tt.dbErr)
			} else {
				expect.WillReturnResult(sqlmock.NewResult(0, 1))
			}
			assert.ErrorIs(t, NewFollow(db).UnFollow(context.Background(), 1, 2), tt.dbErr)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestFollowDAOIsFollowing(t *testing.T) {
	databaseErr := errors.New("database unavailable")
	for _, tt := range []struct {
		name  string
		count int64
		dbErr error
		want  bool
	}{
		{name: "已关注", count: 1, want: true},
		{name: "未关注"},
		{name: "数据库异常", dbErr: databaseErr},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, sqlDB := followTestDB(t)
			defer sqlDB.Close()
			expect := mock.ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM `follows` WHERE follower_id = ? AND following_id = ?")).
				WithArgs(1, 2)
			if tt.dbErr != nil {
				expect.WillReturnError(tt.dbErr)
			} else {
				expect.WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(tt.count))
			}
			got, err := NewFollow(db).IsFollowing(context.Background(), 1, 2)
			assert.Equal(t, tt.want, got)
			assert.ErrorIs(t, err, tt.dbErr)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestFollowDAOListFollowingPage(t *testing.T) {
	db, mock, sqlDB := followTestDB(t)
	defer sqlDB.Close()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT "+followListSelect+" FROM `follows` LEFT JOIN users ON users.id = follows.following_id WHERE follows.follower_id = ? ORDER BY follows.created_at DESC, follows.id DESC LIMIT ? OFFSET ?")).
		WithArgs(1, 5, 5).
		WillReturnRows(sqlmock.NewRows([]string{"id", "follower_id", "following_id", "created_at", "updated_at", "nickname"}).
			AddRow(3, 1, 2, 100, 200, "云端旅人"))

	items, err := NewFollow(db).ListFollowingPage(context.Background(), 1, 5, 5)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "云端旅人", items[0].Nickname)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFollowDAOListFollowersPage(t *testing.T) {
	db, mock, sqlDB := followTestDB(t)
	defer sqlDB.Close()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT "+followListSelect+" FROM `follows` LEFT JOIN users ON users.id = follows.follower_id WHERE follows.following_id = ? ORDER BY follows.created_at DESC, follows.id DESC LIMIT ? OFFSET ?")).
		WithArgs(1, 5, 5).
		WillReturnRows(sqlmock.NewRows([]string{"id", "follower_id", "following_id", "created_at", "updated_at", "nickname"}).
			AddRow(3, 2, 1, 100, 200, "云端旅人"))

	items, err := NewFollow(db).ListFollowersPage(context.Background(), 1, 5, 5)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "云端旅人", items[0].Nickname)
	assert.NoError(t, mock.ExpectationsWereMet())
}
