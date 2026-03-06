package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
)

// ConnConfig holds the settings for dialing a gRPC REAPI server.
type ConnConfig struct {
	Target       string        // e.g. "localhost:9092" or "cache.buildbuddy.io:443"
	InstanceName string        // REAPI instance_name for multi-tenant servers
	TLS          bool          // enable TLS (default true)
	TLSCACert    string        // custom CA cert path (optional)
	AuthToken    string        // bearer token for authentication (optional)
	DialTimeout  time.Duration // default 10s
	Insecure     bool          // plaintext for local dev
}

// Dial creates a gRPC client connection with retry, keepalive, and optional TLS/auth.
func Dial(ctx context.Context, cfg ConnConfig) (*grpc.ClientConn, error) {
	if cfg.Target == "" {
		return nil, fmt.Errorf("grpc target is required")
	}

	timeout := cfg.DialTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	_, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	opts := []grpc.DialOption{
		grpc.WithDefaultServiceConfig(retryServiceConfig),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	}

	// Transport credentials.
	switch {
	case cfg.Insecure:
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	case cfg.TLS:
		creds, err := buildTLSCredentials(cfg.TLSCACert)
		if err != nil {
			return nil, fmt.Errorf("tls: %w", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	default:
		// Default: system TLS certs.
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	}

	// Bearer token auth interceptors.
	if cfg.AuthToken != "" {
		bearer := "Bearer " + cfg.AuthToken
		opts = append(opts,
			grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, callOpts ...grpc.CallOption) error {
				ctx = metadata.AppendToOutgoingContext(ctx, "authorization", bearer)
				return invoker(ctx, method, req, reply, cc, callOpts...)
			}),
			grpc.WithStreamInterceptor(func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, callOpts ...grpc.CallOption) (grpc.ClientStream, error) {
				ctx = metadata.AppendToOutgoingContext(ctx, "authorization", bearer)
				return streamer(ctx, desc, cc, method, callOpts...)
			}),
		)
	}

	conn, err := grpc.NewClient(cfg.Target, opts...)
	if err != nil {
		return nil, fmt.Errorf("grpc dial %s: %w", cfg.Target, err)
	}
	return conn, nil
}

func buildTLSCredentials(caCertPath string) (credentials.TransportCredentials, error) {
	if caCertPath == "" {
		return credentials.NewTLS(&tls.Config{}), nil
	}

	certPool := x509.NewCertPool()
	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("read CA cert %s: %w", caCertPath, err)
	}
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA cert %s", caCertPath)
	}
	return credentials.NewTLS(&tls.Config{
		RootCAs: certPool,
	}), nil
}

// retryServiceConfig configures gRPC built-in retry:
// max 4 attempts, initial 100ms backoff, 2x multiplier, max 5s,
// retryable on UNAVAILABLE (14) and DEADLINE_EXCEEDED (4).
const retryServiceConfig = `{
  "methodConfig": [{
    "name": [{}],
    "retryPolicy": {
      "maxAttempts": 4,
      "initialBackoff": "0.1s",
      "maxBackoff": "5s",
      "backoffMultiplier": 2,
      "retryableStatusCodes": ["UNAVAILABLE", "DEADLINE_EXCEEDED"]
    }
  }]
}`
