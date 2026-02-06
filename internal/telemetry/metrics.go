package telemetry

import (
	"context"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	otelprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// initMetrics initializes the metrics provider.
func (p *Provider) initMetrics(ctx context.Context, res *resource.Resource) error {
	var reader sdkmetric.Reader
	var handler http.Handler
	var err error

	switch p.config.Metrics.Exporter {
	case "prometheus", "":
		reader, handler, err = p.createPrometheusExporter()
	case "otlp":
		reader, err = p.createOTLPMetricExporter(ctx)
	case "stdout":
		exporter, exporterErr := stdoutmetric.New(stdoutmetric.WithPrettyPrint())
		if exporterErr != nil {
			return fmt.Errorf("failed to create stdout metric exporter: %w", exporterErr)
		}
		reader = sdkmetric.NewPeriodicReader(exporter,
			sdkmetric.WithInterval(p.config.Metrics.ExportInterval),
		)
	default:
		return fmt.Errorf("unknown metrics exporter: %s", p.config.Metrics.Exporter)
	}

	if err != nil {
		return fmt.Errorf("failed to create metrics exporter: %w", err)
	}

	// Create meter provider
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(reader),
	)

	// Register as global meter provider
	otel.SetMeterProvider(mp)

	// Store provider and handler
	p.meterProvider = &MeterProvider{
		handler: handler,
	}

	// Add shutdown function
	p.shutdownFuncs = append(p.shutdownFuncs, mp.Shutdown)

	log.Info().
		Str("exporter", p.config.Metrics.Exporter).
		Str("path", p.config.Metrics.Path).
		Bool("same_port", p.config.Metrics.SamePort).
		Int("port", p.config.Metrics.Port).
		Msg("Metrics provider initialized")

	return nil
}

// createPrometheusExporter creates a Prometheus metrics exporter.
func (p *Provider) createPrometheusExporter() (sdkmetric.Reader, http.Handler, error) {
	// Create a new prometheus registry
	registry := prometheus.NewRegistry()

	// Create the prometheus exporter
	exporter, err := otelprometheus.New(
		otelprometheus.WithRegisterer(registry),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
	}

	// Create HTTP handler for metrics endpoint
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})

	return exporter, handler, nil
}

// createOTLPMetricExporter creates an OTLP metrics exporter.
func (p *Provider) createOTLPMetricExporter(ctx context.Context) (sdkmetric.Reader, error) {
	endpoint := p.config.Metrics.Endpoint
	if endpoint == "" {
		if p.config.Metrics.Protocol == "http" {
			endpoint = "localhost:4318"
		} else {
			endpoint = "localhost:4317"
		}
	}

	var exporter sdkmetric.Exporter
	var err error

	switch p.config.Metrics.Protocol {
	case "http":
		opts := []otlpmetrichttp.Option{
			otlpmetrichttp.WithEndpoint(endpoint),
		}
		if p.config.Metrics.Insecure {
			opts = append(opts, otlpmetrichttp.WithInsecure())
		}
		exporter, err = otlpmetrichttp.New(ctx, opts...)

	case "grpc", "":
		opts := []otlpmetricgrpc.Option{
			otlpmetricgrpc.WithEndpoint(endpoint),
		}
		if p.config.Metrics.Insecure {
			opts = append(opts, otlpmetricgrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())))
		}
		exporter, err = otlpmetricgrpc.New(ctx, opts...)

	default:
		return nil, fmt.Errorf("unknown metrics protocol: %s", p.config.Metrics.Protocol)
	}

	if err != nil {
		return nil, err
	}

	return sdkmetric.NewPeriodicReader(exporter,
		sdkmetric.WithInterval(p.config.Metrics.ExportInterval),
	), nil
}

// Meter returns a named meter for creating instruments.
func Meter(name string) metric.Meter {
	return otel.Meter(name)
}

// Metrics holds commonly used metrics instruments.
type Metrics struct {
	// HTTP metrics
	HTTPRequestsTotal   metric.Int64Counter
	HTTPRequestDuration metric.Float64Histogram

	// Run metrics
	RunsTotal   metric.Int64Counter
	RunDuration metric.Float64Histogram
	ActiveRuns  metric.Int64UpDownCounter

	// Task metrics
	TasksTotal     metric.Int64Counter
	TaskDuration   metric.Float64Histogram
	CacheHitsTotal metric.Int64Counter

	// Auth metrics
	AuthAttemptsTotal metric.Int64Counter
}

// NewMetrics creates and registers all metric instruments.
func NewMetrics() (*Metrics, error) {
	meter := Meter("dagryn")

	httpRequestsTotal, err := meter.Int64Counter(
		"dagryn_http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	httpRequestDuration, err := meter.Float64Histogram(
		"dagryn_http_request_duration_seconds",
		metric.WithDescription("HTTP request duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	runsTotal, err := meter.Int64Counter(
		"dagryn_runs_total",
		metric.WithDescription("Total number of runs"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	runDuration, err := meter.Float64Histogram(
		"dagryn_run_duration_seconds",
		metric.WithDescription("Run duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	activeRuns, err := meter.Int64UpDownCounter(
		"dagryn_active_runs",
		metric.WithDescription("Number of currently active runs"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	tasksTotal, err := meter.Int64Counter(
		"dagryn_tasks_total",
		metric.WithDescription("Total number of task executions"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	taskDuration, err := meter.Float64Histogram(
		"dagryn_task_duration_seconds",
		metric.WithDescription("Task execution duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	cacheHitsTotal, err := meter.Int64Counter(
		"dagryn_cache_hits_total",
		metric.WithDescription("Total number of cache hits"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	authAttemptsTotal, err := meter.Int64Counter(
		"dagryn_auth_attempts_total",
		metric.WithDescription("Total number of authentication attempts"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		HTTPRequestsTotal:   httpRequestsTotal,
		HTTPRequestDuration: httpRequestDuration,
		RunsTotal:           runsTotal,
		RunDuration:         runDuration,
		ActiveRuns:          activeRuns,
		TasksTotal:          tasksTotal,
		TaskDuration:        taskDuration,
		CacheHitsTotal:      cacheHitsTotal,
		AuthAttemptsTotal:   authAttemptsTotal,
	}, nil
}
