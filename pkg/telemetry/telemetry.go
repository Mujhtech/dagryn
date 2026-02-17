// Package telemetry provides OpenTelemetry integration for traces, metrics, and logs.
package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Provider holds initialized telemetry providers.
type Provider struct {
	config         Config
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *MeterProvider
	shutdownFuncs  []func(context.Context) error
	mu             sync.Mutex
}

// MeterProvider wraps the OpenTelemetry meter provider.
type MeterProvider struct {
	handler http.Handler // Prometheus handler if using prometheus exporter
}

var (
	globalProvider *Provider
	globalMu       sync.RWMutex
)

// Init initializes the telemetry system with the given configuration.
func Init(ctx context.Context, cfg Config) (*Provider, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid telemetry config: %w", err)
	}

	if !cfg.Enabled {
		log.Info().Msg("Telemetry disabled")
		return &Provider{config: cfg}, nil
	}

	p := &Provider{
		config:        cfg,
		shutdownFuncs: make([]func(context.Context) error, 0),
	}

	// Create resource with service info
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Initialize traces
	if cfg.Traces.Enabled && cfg.Traces.Exporter != "none" {
		if err := p.initTraces(ctx, res); err != nil {
			return nil, fmt.Errorf("failed to initialize traces: %w", err)
		}
	}

	// Initialize metrics
	if cfg.Metrics.Enabled && cfg.Metrics.Exporter != "none" {
		if err := p.initMetrics(ctx, res); err != nil {
			return nil, fmt.Errorf("failed to initialize metrics: %w", err)
		}
	}

	// Set up propagation
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Store as global provider
	globalMu.Lock()
	globalProvider = p
	globalMu.Unlock()

	log.Info().
		Str("service", cfg.ServiceName).
		Bool("traces", cfg.Traces.Enabled).
		Bool("metrics", cfg.Metrics.Enabled).
		Msg("Telemetry initialized")

	return p, nil
}

// Shutdown gracefully shuts down all telemetry providers.
func (p *Provider) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var errs []error
	for _, fn := range p.shutdownFuncs {
		if err := fn(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("telemetry shutdown errors: %v", errs)
	}

	log.Info().Msg("Telemetry shut down successfully")
	return nil
}

// Tracer returns a named tracer.
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

// MetricsHandler returns the Prometheus HTTP handler if using prometheus exporter.
// Returns nil if prometheus is not enabled.
func (p *Provider) MetricsHandler() http.Handler {
	if p.meterProvider != nil && p.meterProvider.handler != nil {
		return p.meterProvider.handler
	}
	// Return a default prometheus handler
	return promhttp.Handler()
}

// Config returns the telemetry configuration.
func (p *Provider) Config() Config {
	return p.config
}

// GetProvider returns the global telemetry provider.
func GetProvider() *Provider {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalProvider
}
