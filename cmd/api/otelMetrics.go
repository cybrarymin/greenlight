package api

import (
	"context"

	"github.com/uptrace/bun"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	otelMeter                         = otel.Meter("cybrarymin.com/package/api") // calling the meter created by meterProvider
	otelMetricHTTPTotalRequests       metric.Int64Counter
	otelMetricHTTPTotalResponses      metric.Int64Counter
	otelMetricHTTPTotalResponseStatus metric.Int64Counter
	otelMetricHttpDuration            metric.Float64Histogram
	otelMetricApplicationVersion      metric.Int64Gauge
	otelMetricDBStatus                metric.Int64ObservableGauge
)

func initializeOtelMetrics(db *bun.DB) error {
	ctx := context.Background()
	var err error
	otelMetricHTTPTotalRequests, err = otelMeter.Int64Counter("http_requests",
		metric.WithDescription("total number of http requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return err
	}

	otelMetricHTTPTotalResponses, err = otelMeter.Int64Counter("http_responses",
		metric.WithDescription("total number of responses"),
		metric.WithUnit("{response}"),
	)
	if err != nil {
		return err
	}

	otelMetricHTTPTotalResponseStatus, err = otelMeter.Int64Counter("http_responses",
		metric.WithDescription("total number of responses based on status codes"),
		metric.WithUnit("{response}"),
	)
	if err != nil {
		return err
	}

	otelMetricHttpDuration, err = otelMeter.Float64Histogram("http_response_time",
		metric.WithDescription("http response time"),
		metric.WithUnit("{time}"),
		metric.WithExplicitBucketBoundaries(5, 10, 50, 100, 1000, 2000, 3000), // all numbers represent the miliseconds. So it will provide you infromation about the miliseconds took a response being sent
	)
	if err != nil {
		return err
	}

	otelMetricApplicationVersion, err = otelMeter.Int64Gauge("application_info",
		metric.WithDescription("application binary version info"),
	)
	if err != nil {
		return err
	}
	otelMetricApplicationVersion.Record(ctx, 1, metric.WithAttributes(attribute.String("version", Version)))

	otelMetricDBStatus, err = otelMeter.Int64ObservableGauge("db_connection_status",
		metric.WithDescription("database connection status"),
		metric.WithUnit("{count}"),
		metric.WithInt64Callback(func(ctx context.Context, obs metric.Int64Observer) error {
			stats := db.Stats()
			obs.Observe(int64(stats.MaxOpenConnections),
				metric.WithAttributes(attribute.String("stat_name", "MaxOpenConnections")),
			)
			obs.Observe(int64(stats.OpenConnections),
				metric.WithAttributes(attribute.String("stat_name", "OpenConnections")),
			)
			obs.Observe(int64(stats.Idle),
				metric.WithAttributes(attribute.String("stat_name", "Idle")),
			)
			obs.Observe(int64(stats.InUse),
				metric.WithAttributes(attribute.String("stat_name", "InUse")),
			)
			obs.Observe(int64(stats.WaitCount),
				metric.WithAttributes(attribute.String("stat_name", "WaitCount")),
			)
			obs.Observe(int64(stats.WaitDuration),
				metric.WithAttributes(attribute.String("stat_name", "WaitDurationMillis")),
			)
			return nil
		}),
	)

	if err != nil {
		return err
	}
	return nil
}
