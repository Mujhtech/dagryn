// Package telemetry provides OpenTelemetry integration for traces, metrics, and logs.
package telemetry

import (
	"time"
)

// Config holds telemetry configuration.
type Config struct {
	Enabled     bool          `toml:"enabled"`
	ServiceName string        `toml:"service_name"`
	Traces      TracesConfig  `toml:"traces"`
	Metrics     MetricsConfig `toml:"metrics"`
}

// TracesConfig holds trace exporter configuration.
type TracesConfig struct {
	Enabled    bool    `toml:"enabled"`
	Exporter   string  `toml:"exporter"`    // "otlp", "stdout", "none"
	Protocol   string  `toml:"protocol"`    // "grpc", "http"
	Endpoint   string  `toml:"endpoint"`    // e.g., "localhost:4317"
	SampleRate float64 `toml:"sample_rate"` // 0.0 to 1.0
	Insecure   bool    `toml:"insecure"`    // Skip TLS verification
}

// MetricsConfig holds metrics exporter configuration.
type MetricsConfig struct {
	Enabled        bool          `toml:"enabled"`
	Exporter       string        `toml:"exporter"`        // "prometheus", "otlp", "stdout", "none"
	Protocol       string        `toml:"protocol"`        // "grpc", "http" (for otlp)
	Endpoint       string        `toml:"endpoint"`        // OTLP endpoint
	SamePort       bool          `toml:"same_port"`       // Expose on main server port
	Port           int           `toml:"port"`            // Dedicated metrics port
	Path           string        `toml:"path"`            // Prometheus path (default: /metrics)
	Insecure       bool          `toml:"insecure"`        // Skip TLS verification
	ExportInterval time.Duration `toml:"export_interval"` // For OTLP push
}

// DefaultConfig returns sensible defaults for telemetry configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:     true,
		ServiceName: "dagryn-server",
		Traces: TracesConfig{
			Enabled:    true,
			Exporter:   "otlp",
			Protocol:   "grpc",
			Endpoint:   "localhost:4317",
			SampleRate: 1.0,
			Insecure:   true,
		},
		Metrics: MetricsConfig{
			Enabled:        true,
			Exporter:       "prometheus",
			SamePort:       true,
			Port:           9090,
			Path:           "/metrics",
			ExportInterval: 30 * time.Second,
		},
	}
}

// Validate validates the telemetry configuration.
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}

	if c.ServiceName == "" {
		c.ServiceName = "dagryn-server"
	}

	if c.Traces.Enabled {
		switch c.Traces.Exporter {
		case "otlp", "stdout", "none", "":
			// valid
		default:
			return &ConfigError{Field: "traces.exporter", Message: "must be 'otlp', 'stdout', or 'none'"}
		}

		switch c.Traces.Protocol {
		case "grpc", "http", "":
			// valid
		default:
			return &ConfigError{Field: "traces.protocol", Message: "must be 'grpc' or 'http'"}
		}

		if c.Traces.SampleRate < 0 || c.Traces.SampleRate > 1 {
			return &ConfigError{Field: "traces.sample_rate", Message: "must be between 0.0 and 1.0"}
		}
	}

	if c.Metrics.Enabled {
		switch c.Metrics.Exporter {
		case "prometheus", "otlp", "stdout", "none", "":
			// valid
		default:
			return &ConfigError{Field: "metrics.exporter", Message: "must be 'prometheus', 'otlp', 'stdout', or 'none'"}
		}

		if c.Metrics.Path == "" {
			c.Metrics.Path = "/metrics"
		}
	}

	return nil
}

// ConfigError represents a configuration error.
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return "telemetry config error: " + e.Field + ": " + e.Message
}
