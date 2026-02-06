package telemetry

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// initTraces initializes the trace provider.
func (p *Provider) initTraces(ctx context.Context, res *resource.Resource) error {
	var exporter sdktrace.SpanExporter
	var err error

	switch p.config.Traces.Exporter {
	case "otlp", "":
		exporter, err = p.createOTLPTraceExporter(ctx)
	case "stdout":
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
	default:
		return fmt.Errorf("unknown trace exporter: %s", p.config.Traces.Exporter)
	}

	if err != nil {
		return fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create sampler based on sample rate
	var sampler sdktrace.Sampler
	if p.config.Traces.SampleRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if p.config.Traces.SampleRate <= 0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(p.config.Traces.SampleRate)
	}

	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sampler)),
	)

	// Register as global trace provider
	otel.SetTracerProvider(tp)
	p.tracerProvider = tp

	// Add shutdown function
	p.shutdownFuncs = append(p.shutdownFuncs, tp.Shutdown)

	log.Info().
		Str("exporter", p.config.Traces.Exporter).
		Str("protocol", p.config.Traces.Protocol).
		Str("endpoint", p.config.Traces.Endpoint).
		Float64("sample_rate", p.config.Traces.SampleRate).
		Msg("Trace provider initialized")

	return nil
}

// createOTLPTraceExporter creates an OTLP trace exporter based on protocol.
func (p *Provider) createOTLPTraceExporter(ctx context.Context) (sdktrace.SpanExporter, error) {
	endpoint := p.config.Traces.Endpoint
	if endpoint == "" {
		if p.config.Traces.Protocol == "http" {
			endpoint = "localhost:4318"
		} else {
			endpoint = "localhost:4317"
		}
	}

	switch p.config.Traces.Protocol {
	case "http":
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(endpoint),
		}
		if p.config.Traces.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		return otlptracehttp.New(ctx, opts...)

	case "grpc", "":
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(endpoint),
		}
		if p.config.Traces.Insecure {
			opts = append(opts, otlptracegrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())))
		}
		return otlptracegrpc.New(ctx, opts...)

	default:
		return nil, fmt.Errorf("unknown trace protocol: %s", p.config.Traces.Protocol)
	}
}

// TracerProvider returns the trace provider.
func (p *Provider) TracerProvider() trace.TracerProvider {
	if p.tracerProvider != nil {
		return p.tracerProvider
	}
	return otel.GetTracerProvider()
}

// StartSpan is a helper to start a new span with common attributes.
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return Tracer("dagryn").Start(ctx, name, opts...)
}

// SpanFromContext returns the current span from context.
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}
