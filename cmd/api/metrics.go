package api

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/uptrace/bun"
)

var (
	promHttpTotalRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{ // metric name will be Namespace_Name
			Namespace: "http",
			Name:      "requests_total",
			Help:      "Number of HTTP request by path", // description of the metric
		},
		[]string{"path"}) // labels to be added to the metric

	promHttpTotalResponse = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "http",
			Name:      "responses_total",
			Help:      "Number of HTTP request by path",
		},
		[]string{})
	promHttpResponseStatus = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "http",
		Name:      "response_status_total",
		Help:      "Total number of response with specific status code",
	},
		[]string{"code"})

	promHttpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "http",
		Name:      "response_time_seconds",
		Help:      "Duration of HTTP requests.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"path"})

	promApplicationVersion = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "application",
		Name:      "info",
		Help:      "Application binary version",
	}, []string{"version"})

	promDbStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "database",
		Name:      "connection_status",
	}, []string{"type"})
)

func promInit(db *bun.DB) {
	prometheus.MustRegister(
		promHttpTotalRequests,
		promHttpResponseStatus,
		promHttpDuration,
		promApplicationVersion,
		promDbStatus,
		promHttpTotalResponse,
	)
	go func() {
		for {
			promDbStatus.WithLabelValues("MaxOpenConnections").Set(float64(db.Stats().MaxOpenConnections))
			promDbStatus.WithLabelValues("OpenConnections").Set(float64(db.Stats().OpenConnections))
			promDbStatus.WithLabelValues("Idle").Set(float64(db.Stats().Idle))
			promDbStatus.WithLabelValues("InUse").Set(float64(db.Stats().InUse))
			promDbStatus.WithLabelValues("MaxIdleClosed").Set(float64(db.Stats().MaxIdleClosed))
			promDbStatus.WithLabelValues("MaxIdleTimeClosed").Set(float64(db.Stats().MaxIdleTimeClosed))
			promDbStatus.WithLabelValues("MaxLifetimeClosed").Set(float64(db.Stats().MaxLifetimeClosed))
			promDbStatus.WithLabelValues("WaitCount").Set(float64(db.Stats().WaitCount))
			promDbStatus.WithLabelValues("WaitDuration").Set(float64(db.Stats().WaitDuration))
			time.Sleep(time.Millisecond * 500)
		}
	}()

	promApplicationVersion.WithLabelValues(version).Set(1)
}
