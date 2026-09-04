package migrations

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyAllWithLockRejectsNegativeTimeout(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	err = ApplyAllWithLock(context.Background(), db, -1)
	assert.ErrorContains(t, err, "不能为负数")
}

func TestApplyRejectsChecksumMismatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	migration := Migration{Version: "009_example", Script: "CREATE TABLE example (id BIGINT);"}
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT checksum, dirty FROM schema_migrations WHERE version = ?",
	)).WithArgs(migration.Version).
		WillReturnRows(sqlmock.NewRows([]string{"checksum", "dirty"}).AddRow("old-checksum", false))

	err = apply(context.Background(), db, migration)
	assert.ErrorContains(t, err, "checksum 不一致")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyLeavesFailedDDLDirty(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	migration := Migration{Version: "009_example", Script: "CREATE TABLE example (id BIGINT);"}
	checksum := migrationChecksum(migration.Script)
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT checksum, dirty FROM schema_migrations WHERE version = ?",
	)).WithArgs(migration.Version).WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(regexp.QuoteMeta(
		"INSERT INTO schema_migrations (version, checksum, dirty, applied_at) VALUES (?, ?, TRUE, 0)",
	)).WithArgs(migration.Version, checksum).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE example (id BIGINT)")).
		WillReturnError(errors.New("ddl failed"))

	err = apply(context.Background(), db, migration)
	assert.ErrorContains(t, err, "迁移保持 dirty 状态")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyRejectsEmptyMigrationBeforeMarkingDirty(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	migration := Migration{Version: "009_example", Script: "  \n\t"}
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT checksum, dirty FROM schema_migrations WHERE version = ?",
	)).WithArgs(migration.Version).WillReturnError(sql.ErrNoRows)

	err = apply(context.Background(), db, migration)
	assert.ErrorContains(t, err, "没有 SQL 语句")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyStopsOnDirtyMigration(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	migration := Migration{Version: "009_example", Script: "SELECT 1;"}
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT checksum, dirty FROM schema_migrations WHERE version = ?",
	)).WithArgs(migration.Version).
		WillReturnRows(sqlmock.NewRows([]string{"checksum", "dirty"}).AddRow(migrationChecksum(migration.Script), true))

	err = apply(context.Background(), db, migration)
	assert.ErrorContains(t, err, "处于 dirty 状态")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyAllWithLockTimesOut(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("SELECT GET_LOCK").
		WithArgs("followingfeed_schema_migration", 30).
		WillReturnRows(sqlmock.NewRows([]string{"GET_LOCK"}).AddRow(0))

	err = ApplyAllWithLock(context.Background(), db, 30)
	assert.ErrorContains(t, err, "已等待 30 秒")
	assert.NoError(t, mock.ExpectationsWereMet())
}
