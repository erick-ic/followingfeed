package observability

import (
	"database/sql"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const namespace = "followingfeed"

// Metrics 管理应用自身的 Prometheus 注册表和采集器。使用独立注册表可避免测试中重复注册，
// 也能防止无关的全局采集器出现在管理端点中。
type Metrics struct {
	registry *prometheus.Registry

	httpRequests    *prometheus.CounterVec
	httpDuration    *prometheus.HistogramVec
	httpInFlight    prometheus.Gauge
	panics          prometheus.Counter
	cacheOperations *prometheus.CounterVec
	cacheDuration   *prometheus.HistogramVec
	dbDuration      *prometheus.HistogramVec
}

func NewMetrics() *Metrics {
	registry := prometheus.NewRegistry()
	m := &Metrics{
		registry: registry,
		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "http_server",
			Name:      "requests_total",
			Help:      "Total number of HTTP server requests.",
		}, []string{"method", "route", "status_code"}),
		httpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "http_server",
			Name:      "request_duration_seconds",
			Help:      "HTTP server request duration in seconds.",
			Buckets:   []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		}, []string{"method", "route", "status_code"}),
		httpInFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "http_server",
			Name:      "requests_in_flight",
			Help:      "Current number of HTTP requests being served.",
		}),
		panics: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "http_server",
			Name:      "panics_total",
			Help:      "Total number of recovered HTTP handler panics.",
		}),
		cacheOperations: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "cache",
			Name:      "operations_total",
			Help:      "Business cache operations by cache name, operation, and result.",
		}, []string{"cache", "operation", "result"}),
		cacheDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "cache",
			Name:      "operation_duration_seconds",
			Help:      "Business cache operation duration in seconds.",
			Buckets:   []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25},
		}, []string{"cache", "operation"}),
		dbDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "db",
			Name:      "operation_duration_seconds",
			Help:      "Database operation duration in seconds by operation, table, and result.",
			Buckets:   []float64{0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5},
		}, []string{"operation", "table", "result"}),
	}

	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.httpRequests,
		m.httpDuration,
		m.httpInFlight,
		m.panics,
		m.cacheOperations,
		m.cacheDuration,
		m.dbDuration,
	)
	return m
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *Metrics) Registry() *prometheus.Registry {
	return m.registry
}

func (m *Metrics) RegisterDBStats(db *sql.DB) error {
	return m.registry.Register(collectors.NewDBStatsCollector(db, "mysql"))
}
