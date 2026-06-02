package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// promauto registers metrics automatically — no need to call Register() manually

var (
	// ── job metrics ──────────────────────────────────────────────

	JobsEnqueued = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notiq_jobs_enqueued_total",
			Help: "Total number of jobs enqueued, partitioned by type.",
		},
		[]string{"type"},
	)

	JobsProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notiq_jobs_processed_total",
			Help: "Total number of jobs processed, partitioned by type and final status.",
		},
		[]string{"type", "status"},
	)

	JobDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "notiq_job_processing_duration_seconds",
			Help:    "Job processing duration in seconds, partitioned by type.",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		},
		[]string{"type"},
	)

	// ── worker pool metrics ───────────────────────────────────────

	WorkerPoolActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "notiq_worker_pool_active_goroutines",
			Help: "Number of goroutines currently processing jobs in the pool.",
		},
	)

	WorkerPoolQueued = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "notiq_worker_pool_queued_jobs",
			Help: "Number of jobs currently buffered in the pool channel.",
		},
	)

	// ── HTTP metrics ──────────────────────────────────────────────

	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notiq_http_requests_total",
			Help: "Total HTTP requests, partitioned by method, path, and status code.",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "notiq_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds, partitioned by method and path.",
			Buckets: prometheus.DefBuckets, // .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10
		},
		[]string{"method", "path"},
	)
)

// ── helper functions — called by other packages ───────────────────────────────

// RecordJobEnqueued increments the enqueued counter for a job type.
func RecordJobEnqueued(jobType string) {
	JobsEnqueued.WithLabelValues(jobType).Inc()
}

// RecordJobProcessed increments the processed counter and records duration.
func RecordJobProcessed(jobType, status string, durationSeconds float64) {
	JobsProcessed.WithLabelValues(jobType, status).Inc()
	JobDuration.WithLabelValues(jobType).Observe(durationSeconds)
}

// RecordHTTPRequest records an HTTP request's method, path, status, and duration.
func RecordHTTPRequest(method, path, status string, durationSeconds float64) {
	HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
	HTTPRequestDuration.WithLabelValues(method, path).Observe(durationSeconds)
}