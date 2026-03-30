package observability

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var registerOnce sync.Once

var (
	apiRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agentmsg_api_requests_total",
			Help: "Total number of API requests processed.",
		},
		[]string{"method", "route", "status"},
	)
	apiRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "agentmsg_api_request_duration_seconds",
			Help:    "API request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route", "status"},
	)
	authFailuresTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agentmsg_auth_failures_total",
			Help: "Total number of authentication failures.",
		},
		[]string{"reason"},
	)
	rateLimitExceededTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agentmsg_rate_limit_exceeded_total",
			Help: "Total number of rate limit rejections.",
		},
		[]string{"route"},
	)
	auditLogsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agentmsg_audit_logs_total",
			Help: "Total number of persisted audit log entries.",
		},
		[]string{"action", "status"},
	)
	messageOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agentmsg_message_operations_total",
			Help: "Total number of message operations by operation and outcome.",
		},
		[]string{"operation", "outcome"},
	)
	messageProcessingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "agentmsg_message_processing_duration_seconds",
			Help:    "Duration of message processing operations in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation", "outcome"},
	)
)

func init() {
	registerMetrics()
}

func registerMetrics() {
	registerOnce.Do(func() {
		prometheus.MustRegister(
			apiRequestsTotal,
			apiRequestDuration,
			authFailuresTotal,
			rateLimitExceededTotal,
			auditLogsTotal,
			messageOperationsTotal,
			messageProcessingDuration,
		)
	})
}

func RecordAPIRequest(method, route string, statusCode int, duration time.Duration) {
	registerMetrics()
	status := prometheusLabelStatus(statusCode)
	apiRequestsTotal.WithLabelValues(method, route, status).Inc()
	apiRequestDuration.WithLabelValues(method, route, status).Observe(duration.Seconds())
}

func RecordAuthFailure(reason string) {
	registerMetrics()
	authFailuresTotal.WithLabelValues(reason).Inc()
}

func RecordRateLimitExceeded(route string) {
	registerMetrics()
	rateLimitExceededTotal.WithLabelValues(route).Inc()
}

func RecordAuditLog(action string, statusCode int) {
	registerMetrics()
	auditLogsTotal.WithLabelValues(action, prometheusLabelStatus(statusCode)).Inc()
}

func RecordMessageOperation(operation, outcome string, duration time.Duration) {
	registerMetrics()
	messageOperationsTotal.WithLabelValues(operation, outcome).Inc()
	messageProcessingDuration.WithLabelValues(operation, outcome).Observe(duration.Seconds())
}

func prometheusLabelStatus(statusCode int) string {
	switch {
	case statusCode >= 500:
		return "5xx"
	case statusCode >= 400:
		return "4xx"
	case statusCode >= 300:
		return "3xx"
	case statusCode >= 200:
		return "2xx"
	default:
		return "1xx"
	}
}
