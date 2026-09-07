//go:build migrationcheck

package migrations

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type indexDefinition struct {
	Unique  bool
	Columns []string
}

// TestMigrationsAgainstFreshMySQL 验证全部迁移可在空 MySQL 数据库执行且可安全重入。
func TestMigrationsAgainstFreshMySQL(t *testing.T) {
	dsn := os.Getenv("MIGRATION_CHECK_DSN")
	require.NotEmpty(
		t,
		dsn,
		"请通过 make check-migrations 运行，或为 IDE 配置隔离 MySQL 和 MIGRATION_CHECK_DSN",
	)

	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	require.NoError(t, db.PingContext(ctx))
	require.NoError(t, ApplyAllWithLock(ctx, db, 5))

	all, err := All()
	require.NoError(t, err)

	var applied, dirty int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*), COALESCE(SUM(dirty), 0) FROM schema_migrations",
	).Scan(&applied, &dirty))
	assert.Equal(t, len(all), applied)
	assert.Zero(t, dirty)

	expectedTables := []string{
		"articles",
		"follows",
		"interactives",
		"publish_articles",
		"schema_migrations",
		"user_collection_bizs",
		"user_like_bizs",
		"users",
	}
	rows, err := db.QueryContext(ctx, `SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'
		ORDER BY table_name`)
	require.NoError(t, err)
	defer rows.Close()

	actualTables := make([]string, 0, len(expectedTables))
	for rows.Next() {
		var table string
		require.NoError(t, rows.Scan(&table))
		actualTables = append(actualTables, table)
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, expectedTables, actualTables)
	require.NoError(t, rows.Close())

	expectedIndexes := map[string]map[string]indexDefinition{
		"schema_migrations": {
			"PRIMARY": {Unique: true, Columns: []string{"version"}},
		},
		"users": {
			"PRIMARY":        {Unique: true, Columns: []string{"id"}},
			"uk_users_email": {Unique: true, Columns: []string{"email"}},
		},
		"articles": {
			"PRIMARY":                    {Unique: true, Columns: []string{"id"}},
			"idx_articles_author_status": {Columns: []string{"author_id", "deleted_at", "status"}},
			"idx_articles_author_list":   {Columns: []string{"author_id", "deleted_at", "updated_at", "id"}},
		},
		"publish_articles": {
			"PRIMARY":                          {Unique: true, Columns: []string{"id"}},
			"idx_publish_articles_public_list": {Columns: []string{"status", "deleted_at", "updated_at", "id"}},
			"idx_publish_articles_author_list": {Columns: []string{"author_id", "status", "deleted_at", "updated_at", "id"}},
		},
		"follows": {
			"PRIMARY":                       {Unique: true, Columns: []string{"id"}},
			"uk_follows_follower_following": {Unique: true, Columns: []string{"follower_id", "following_id"}},
			"idx_follows_follower_created":  {Columns: []string{"follower_id", "created_at", "id"}},
			"idx_follows_following_created": {Columns: []string{"following_id", "created_at", "id"}},
		},
		"interactives": {
			"PRIMARY":             {Unique: true, Columns: []string{"id"}},
			"uk_interactives_biz": {Unique: true, Columns: []string{"biz_id", "biz"}},
		},
		"user_like_bizs": {
			"PRIMARY":                  {Unique: true, Columns: []string{"id"}},
			"uk_user_like_biz":         {Unique: true, Columns: []string{"uid", "biz_id", "biz"}},
			"idx_user_like_biz_target": {Columns: []string{"biz", "biz_id", "status"}},
		},
		"user_collection_bizs": {
			"PRIMARY":                    {Unique: true, Columns: []string{"id"}},
			"uk_user_collection_biz":     {Unique: true, Columns: []string{"uid", "biz_id", "biz"}},
			"idx_user_collection_target": {Columns: []string{"biz", "biz_id", "status"}},
			"idx_user_collection_list":   {Columns: []string{"uid", "biz", "status", "updated_at", "id", "biz_id"}},
		},
	}
	assert.Equal(t, expectedIndexes, loadIndexes(t, ctx, db))

	// 再次执行必须保持成功且不能重复登记版本。
	require.NoError(t, ApplyAllWithLock(ctx, db, 5))
	var reapplied int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&reapplied))
	assert.Equal(t, applied, reapplied)
}

func loadIndexes(t *testing.T, ctx context.Context, db *sql.DB) map[string]map[string]indexDefinition {
	t.Helper()
	rows, err := db.QueryContext(ctx, `SELECT table_name, index_name, non_unique, column_name
		FROM information_schema.statistics
		WHERE table_schema = DATABASE()
		ORDER BY table_name, index_name, seq_in_index`)
	require.NoError(t, err)
	defer rows.Close()

	indexes := make(map[string]map[string]indexDefinition)
	for rows.Next() {
		var table, name, column string
		var nonUnique int
		require.NoError(t, rows.Scan(&table, &name, &nonUnique, &column))
		if indexes[table] == nil {
			indexes[table] = make(map[string]indexDefinition)
		}
		definition := indexes[table][name]
		definition.Unique = nonUnique == 0
		definition.Columns = append(definition.Columns, column)
		indexes[table][name] = definition
	}
	require.NoError(t, rows.Err())
	return indexes
}
