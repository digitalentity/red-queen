package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Events
	EventsProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "redqueen_events_processed_total",
		Help: "The total number of processed surveillance events",
	}, []string{"zone", "status"}) // status: success, error

	// ML Analysis
	MLAnalysisDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "redqueen_ml_analysis_duration_seconds",
		Help:    "Duration of ML analysis in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"provider", "zone"})

	MLThreatsDetected = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "redqueen_threats_detected_total",
		Help: "The total number of confirmed threats detected",
	}, []string{"zone"})

	// Detection Pipeline (Prefilter)
	PrefilterDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "redqueen_detection_prefilter_duration_seconds",
		Help:    "Latency of the prefilter stage in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"zone", "analyzer"})

	PrefilterOutcome = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "redqueen_detection_prefilter_outcome_total",
		Help: "Counts of prefilter outcomes (pass, filtered, error, analysis-fallback)",
	}, []string{"zone", "analyzer", "outcome"})

	// Notifications
	NotificationsSent = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "redqueen_notifications_sent_total",
		Help: "The total number of notifications sent",
	}, []string{"provider", "status"}) // status: success, error

	// Storage
	StorageOperations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "redqueen_storage_operations_total",
		Help: "The total number of storage operations",
	}, []string{"provider", "status"})
)
