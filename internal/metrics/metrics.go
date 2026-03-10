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
