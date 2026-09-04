package migrations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const migrationLockName = "followingfeed_schema_migration"

// ApplyAll 执行所有尚未记录的内嵌迁移。测试和单进程工具可以直接使用；
// 多实例部署应使用 ApplyAllWithLock。
func ApplyAll(ctx context.Context, db *sql.DB) error {
	return applyAll(ctx, db)
}

// ApplyAllWithLock 使用数据库建议锁串行执行迁移任务。
func ApplyAllWithLock(ctx context.Context, db *sql.DB, lockTimeoutSeconds int) error {
	return withLock(ctx, db, lockTimeoutSeconds, applyAll)
}

type migrationDB interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func withLock(
	ctx context.Context,
	db *sql.DB,
	lockTimeoutSeconds int,
	operation func(context.Context, migrationDB) error,
) error {
	if lockTimeoutSeconds < 0 {
		return fmt.Errorf("迁移锁等待时间不能为负数")
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("获取迁移专用数据库连接失败：%w", err)
	}
	defer conn.Close()

	var acquired sql.NullInt64
	err = conn.QueryRowContext(
		ctx,
		"SELECT GET_LOCK(?, ?)",
		migrationLockName,
		lockTimeoutSeconds,
	).Scan(&acquired)
	if err != nil {
		return fmt.Errorf("获取数据库迁移锁失败：%w", err)
	}
	if !acquired.Valid || acquired.Int64 != 1 {
		return fmt.Errorf("获取数据库迁移锁超时，已等待 %d 秒", lockTimeoutSeconds)
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var released sql.NullInt64
		_ = conn.QueryRowContext(releaseCtx, "SELECT RELEASE_LOCK(?)", migrationLockName).
			Scan(&released)
	}()

	return operation(ctx, conn)
}

func applyAll(ctx context.Context, db migrationDB) error {
	if err := ensureMigrationTable(ctx, db); err != nil {
		return err
	}
	all, err := All()
	if err != nil {
		return err
	}
	for _, migration := range all {
		if err = apply(ctx, db, migration); err != nil {
			return err
		}
	}
	return nil
}

func ensureMigrationTable(ctx context.Context, db migrationDB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(128) NOT NULL PRIMARY KEY,
		checksum CHAR(64) NOT NULL DEFAULT '',
		dirty BOOLEAN NOT NULL DEFAULT FALSE,
		applied_at BIGINT NOT NULL DEFAULT 0
	)`); err != nil {
		return fmt.Errorf("创建 schema_migrations 表失败：%w", err)
	}
	return nil
}

func apply(ctx context.Context, db migrationDB, migration Migration) error {
	checksum := migrationChecksum(migration.Script)
	var storedChecksum string
	var dirty bool
	err := db.QueryRowContext(
		ctx,
		"SELECT checksum, dirty FROM schema_migrations WHERE version = ?",
		migration.Version,
	).Scan(&storedChecksum, &dirty)
	if err == nil {
		if dirty {
			return fmt.Errorf(
				"迁移 %s 处于 dirty 状态；请恢复迁移前的数据库备份后重新执行迁移",
				migration.Version,
			)
		}
		if storedChecksum != checksum {
			return fmt.Errorf(
				"迁移 %s 的 checksum 不一致：已经执行过的迁移文件禁止修改",
				migration.Version,
			)
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("查询迁移 %s 的执行状态失败：%w", migration.Version, err)
	}

	statements, err := splitSQLStatements(migration.Script)
	if err != nil {
		return fmt.Errorf("解析迁移 %s 失败：%w", migration.Version, err)
	}
	if len(statements) == 0 {
		return fmt.Errorf("解析迁移 %s 失败：脚本中没有 SQL 语句", migration.Version)
	}
	if _, err = db.ExecContext(
		ctx,
		"INSERT INTO schema_migrations (version, checksum, dirty, applied_at) VALUES (?, ?, TRUE, 0)",
		migration.Version,
		checksum,
	); err != nil {
		return fmt.Errorf("将迁移 %s 标记为 dirty 失败：%w", migration.Version, err)
	}

	for index, statement := range statements {
		if _, err = db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf(
				"执行迁移 %s 的第 %d/%d 条语句失败（迁移保持 dirty 状态）：%w",
				migration.Version,
				index+1,
				len(statements),
				err,
			)
		}
	}
	result, err := db.ExecContext(
		ctx,
		"UPDATE schema_migrations SET dirty = FALSE, applied_at = ? WHERE version = ? AND dirty = TRUE",
		time.Now().UnixMilli(),
		migration.Version,
	)
	if err != nil {
		return fmt.Errorf("将迁移 %s 标记为完成失败：%w", migration.Version, err)
	}
	if err = requireOneRow(result, "将迁移 "+migration.Version+" 标记为完成"); err != nil {
		return err
	}
	return nil
}

func requireOneRow(result sql.Result, action string) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s：读取受影响行数失败：%w", action, err)
	}
	if rows != 1 {
		return fmt.Errorf("%s：预期影响 1 行，实际影响 %d 行", action, rows)
	}
	return nil
}

func migrationChecksum(script string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(script)))
}
