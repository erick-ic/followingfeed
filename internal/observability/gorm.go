package observability

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

const dbObservationKey = "followingfeed:observability:db_start"

type dbObservation struct {
	operation string
	start     time.Time
}

// InstrumentGORM 记录标签范围有限且可聚合的查询耗时指标，
// 不会把 SQL 文本、参数、标识符或错误内容写入标签。
func (m *Metrics) InstrumentGORM(db *gorm.DB) error {
	registrations := []struct {
		name     string
		register func() error
	}{
		{"create.before", func() error {
			return db.Callback().Create().Before("gorm:create").
				Register("followingfeed:metrics:before_create", m.beforeGORMMetric("insert"))
		}},
		{"create.after", func() error {
			return db.Callback().Create().After("gorm:create").
				Register("followingfeed:metrics:after_create", m.afterGORMMetric())
		}},
		{"query.before", func() error {
			return db.Callback().Query().Before("gorm:query").
				Register("followingfeed:metrics:before_query", m.beforeGORMMetric("select"))
		}},
		{"query.after", func() error {
			return db.Callback().Query().After("gorm:query").
				Register("followingfeed:metrics:after_query", m.afterGORMMetric())
		}},
		{"update.before", func() error {
			return db.Callback().Update().Before("gorm:update").
				Register("followingfeed:metrics:before_update", m.beforeGORMMetric("update"))
		}},
		{"update.after", func() error {
			return db.Callback().Update().After("gorm:update").
				Register("followingfeed:metrics:after_update", m.afterGORMMetric())
		}},
		{"delete.before", func() error {
			return db.Callback().Delete().Before("gorm:delete").
				Register("followingfeed:metrics:before_delete", m.beforeGORMMetric("delete"))
		}},
		{"delete.after", func() error {
			return db.Callback().Delete().After("gorm:delete").
				Register("followingfeed:metrics:after_delete", m.afterGORMMetric())
		}},
		{"row.before", func() error {
			return db.Callback().Row().Before("gorm:row").
				Register("followingfeed:metrics:before_row", m.beforeGORMMetric("row"))
		}},
		{"row.after", func() error {
			return db.Callback().Row().After("gorm:row").
				Register("followingfeed:metrics:after_row", m.afterGORMMetric())
		}},
		{"raw.before", func() error {
			return db.Callback().Raw().Before("gorm:raw").
				Register("followingfeed:metrics:before_raw", m.beforeGORMMetric("raw"))
		}},
		{"raw.after", func() error {
			return db.Callback().Raw().After("gorm:raw").
				Register("followingfeed:metrics:after_raw", m.afterGORMMetric())
		}},
	}

	for _, registration := range registrations {
		if err := registration.register(); err != nil {
			return fmt.Errorf("注册 GORM 回调 %s 失败：%w", registration.name, err)
		}
	}
	return nil
}

func (m *Metrics) beforeGORMMetric(operation string) func(*gorm.DB) {
	return func(tx *gorm.DB) {
		tx.InstanceSet(dbObservationKey, dbObservation{
			operation: operation,
			start:     time.Now(),
		})
	}
}

func (m *Metrics) afterGORMMetric() func(*gorm.DB) {
	return func(tx *gorm.DB) {
		value, ok := tx.InstanceGet(dbObservationKey)
		if !ok {
			return
		}
		observation, ok := value.(dbObservation)
		if !ok {
			return
		}

		table := tx.Statement.Table
		if table == "" {
			table = "unknown"
		}
		result := "success"
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			result = "not_found"
		} else if tx.Error != nil {
			result = "error"
		}
		m.observeDB(observation.operation, table, result, time.Since(observation.start))
	}
}

func (m *Metrics) observeDB(operation, table, result string, duration time.Duration) {
	m.dbDuration.WithLabelValues(operation, table, result).Observe(duration.Seconds())
}
