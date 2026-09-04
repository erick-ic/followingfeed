package dao

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"followingfeed/internal/domain"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gormMysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestGORMUserDAOInsert(t *testing.T) {
	testCases := []struct {
		name    string
		mock    func(*testing.T) *sql.DB
		wantErr error
	}{
		{
			name: "插入成功",
			mock: func(t *testing.T) *sql.DB {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				mock.ExpectExec("INSERT INTO `users`").WillReturnResult(sqlmock.NewResult(1, 1))
				return db
			},
		},
		{
			name: "邮箱重复",
			mock: func(t *testing.T) *sql.DB {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				mock.ExpectExec("INSERT INTO `users`").
					WillReturnError(&mysql.MySQLError{Number: 1062})
				return db
			},
			wantErr: ErrUserDuplicated,
		},
		{
			name: "数据库异常",
			mock: func(t *testing.T) *sql.DB {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				mock.ExpectExec("INSERT INTO `users`").
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
			gormDB, err := gorm.Open(
				gormMysql.New(gormMysql.Config{
					Conn:                      db,
					SkipInitializeWithVersion: true,
				}),
				&gorm.Config{
					SkipDefaultTransaction: true,
				})
			require.NoError(t, err)

			err = NewGORMUserDAO(gormDB).Insert(
				context.Background(),
				domain.User{
					Nickname: "云端旅人",
					Email:    "alice@example.com",
					Password: "hash",
				})
			if tc.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.wantErr.Error())
			}
		})
	}
}
