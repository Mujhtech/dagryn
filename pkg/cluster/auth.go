package cluster

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const tokenMetadataKey = "x-registration-token"

// HashToken returns a SHA-256 hex hash of the token.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// TokenAuthStreamInterceptor validates the registration token on streaming RPCs.
func TokenAuthStreamInterceptor(registrationToken string) grpc.StreamServerInterceptor {
	expectedHash := HashToken(registrationToken)

	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if registrationToken == "" {
			return handler(srv, ss)
		}

		md, ok := metadata.FromIncomingContext(ss.Context())
		if !ok {
			return status.Error(codes.Unauthenticated, "missing metadata")
		}

		tokens := md.Get(tokenMetadataKey)
		if len(tokens) == 0 {
			return status.Error(codes.Unauthenticated, "missing registration token")
		}

		if HashToken(tokens[0]) != expectedHash {
			return status.Error(codes.Unauthenticated, "invalid registration token")
		}

		return handler(srv, ss)
	}
}

// TokenAuthUnaryInterceptor validates the registration token on unary RPCs.
func TokenAuthUnaryInterceptor(registrationToken string) grpc.UnaryServerInterceptor {
	expectedHash := HashToken(registrationToken)

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if registrationToken == "" {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		tokens := md.Get(tokenMetadataKey)
		if len(tokens) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing registration token")
		}

		if HashToken(tokens[0]) != expectedHash {
			return nil, status.Error(codes.Unauthenticated, "invalid registration token")
		}

		return handler(ctx, req)
	}
}

// LoadTLSCredentials loads mTLS credentials for the gRPC server.
func LoadTLSCredentials(certFile, keyFile, caFile string) (credentials.TransportCredentials, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load server certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
	}

	if caFile != "" {
		caPEM, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("read CA certificate: %w", err)
		}
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.ClientCAs = caPool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return credentials.NewTLS(tlsConfig), nil
}

// LoadClientTLSCredentials loads mTLS credentials for the gRPC client (agent).
func LoadClientTLSCredentials(certFile, keyFile, caFile string) (credentials.TransportCredentials, error) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
	}

	if certFile != "" && keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if caFile != "" {
		caPEM, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("read CA certificate: %w", err)
		}
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caPool
	}

	return credentials.NewTLS(tlsConfig), nil
}
