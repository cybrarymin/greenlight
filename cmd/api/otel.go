package api

import (
	"context"
	"errors"
	"time"

	"github.com/uptrace/bun"
	"go.opentelemetry.io/otel"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

const (
	otelDBErr                    = "error in interaction with database"
	otelDBNotFoundInfo           = "no records found"
	otelunprocessableErr         = "failed to validate and process the information"
	otelAuthFailureErr           = "authentication failed"
	otelUserActivationFailureErr = "user activation failed"
)

var (
	OtlpTraceHost         string
	OtlpHTTPTracePort     string
	OtlpApplicationName   string
	OtlpMetriceHost       string
	OtlpHTTPMetricPort    string
	OtlpHTTPMetricAPIPath string
)

// setupOTelSDK bootstraps the OpenTelemetry pipeline.
// If it does not return an error, make sure to call shutdown for proper cleanup.
func setupOTelSDK(ctx context.Context, db *bun.DB) (shutdown func(context.Context) error, err error) {
	var shutdownFuncs []func(context.Context) error

	// shutdown calls cleanup functions registered via shutdownFuncs.
	// The errors from the calls are joined.
	// Each registered cleanup will be invoked once.
	shutdown = func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	// handleErr calls shutdown for cleanup and makes sure that all errors are returned.
	handleErr := func(inErr error) {
		err = errors.Join(inErr, shutdown(ctx))
	}

	// Setup propagator.
	prop := newPropagator()
	otel.SetTextMapPropagator(prop)

	// Setup trace provider.
	// Setup otel-collector otlphttp exporter
	traceExporter, err := newTraceExporter(ctx)
	if err != nil {
		handleErr(err)
		return
	}
	tracerProvider, err := newTraceProvider(traceExporter)
	if err != nil {
		handleErr(err)
		return
	}

	shutdownFuncs = append(shutdownFuncs, tracerProvider.Shutdown)
	otel.SetTracerProvider(tracerProvider)

	// Setup prometheusOTLP exporter.
	// Setup metric provider.
	metricExporter, err := newMetricExporter(ctx)
	if err != nil {
		handleErr(err)
		return
	}

	meterProvider, err := newMeterProvider(metricExporter)
	if err != nil {
		handleErr(err)
		return
	}

	shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)
	otel.SetMeterProvider(meterProvider)

	// Set up logger provider.
	loggerProvider, err := newLoggerProvider()
	if err != nil {
		handleErr(err)
		return
	}
	shutdownFuncs = append(shutdownFuncs, loggerProvider.Shutdown)
	global.SetLoggerProvider(loggerProvider)

	// Initialize the metrics
	err = initializeOtelMetrics(db)
	if err != nil {
		handleErr(err)
		return
	}

	return
}

func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

func newTraceExporter(ctx context.Context) (trace.SpanExporter, error) {
	// Create an exporter over HTTP for Jaeger endpoint. In latest version, Jaeger supports otlp endpoint
	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(OtlpTraceHost+":"+OtlpHTTPTracePort),
		otlptracehttp.WithInsecure(),
		otlptracehttp.WithTimeout(5*time.Second),
	)

	if err != nil {
		return nil, err
	}
	return traceExporter, nil
}

// create a new otel-collector metric exporter
func newMetricExporter(ctx context.Context) (metric.Exporter, error) {
	metricExporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint(OtlpMetriceHost+":"+OtlpHTTPMetricPort), // host and port only should be specified
		otlpmetrichttp.WithInsecure(),                                       // use http instead of https
		otlpmetrichttp.WithTimeout(5*time.Second),
		otlpmetrichttp.WithURLPath(OtlpHTTPMetricAPIPath), // default prometheus url path for OTLP is /api/v1/otlp/v1/metrics, which we should use here in case pushing metrics directly to prometheus instead of otel-collector
	)
	if err != nil {
		return nil, err
	}

	return metricExporter, nil
}

// To be able to create span
// you need to define a exporter ( stdout , jaeger, prometheus or ....)
// Then with that exporter create a tracer
// use the tracer to create span
func newTraceProvider(traceExporter trace.SpanExporter) (*trace.TracerProvider, error) {
	// define resource attributes. resource attributes are attrs such as pod name, service name, os, arch and...
	rattr, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(OtlpApplicationName),
		))
	if err != nil {
		return nil, err
	}

	traceProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter,
			// Default is 5s. Set to 1s for demonstrative purposes.
			trace.WithBatchTimeout(time.Second)),
		trace.WithResource(rattr),
	)
	return traceProvider, nil
}

// Creates a new metric provider
func newMeterProvider(metricExporter metric.Exporter) (*metric.MeterProvider, error) {
	rattr, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(OtlpApplicationName),
		))
	if err != nil {
		return nil, err
	}

	// reader will read the metrics based on interval and sent it to the exporter
	meterProvider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExporter, metric.WithInterval(time.Second))),
		metric.WithResource(rattr),
	)
	return meterProvider, nil
}

// Creates a new log provider
func newLoggerProvider() (*log.LoggerProvider, error) {
	logExporter, err := stdoutlog.New()
	if err != nil {
		return nil, err
	}

	loggerProvider := log.NewLoggerProvider(
		log.WithProcessor(log.NewBatchProcessor(logExporter)),
	)
	return loggerProvider, nil
}
