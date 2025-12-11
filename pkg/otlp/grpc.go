package otlp

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// GRPCExporter exports traces via OTLP gRPC
type GRPCExporter struct {
	client   ptraceotlp.GRPCClient
	endpoint string
	tenant   string
}

// NewGRPCExporter creates a new gRPC exporter
func NewGRPCExporter(endpoint string, tenant string, timeout time.Duration) (*GRPCExporter, error) {
	// Ensure endpoint doesn't have http:// prefix for gRPC
	if len(endpoint) > 7 && endpoint[:7] == "http://" {
		endpoint = endpoint[7:]
	}
	if len(endpoint) > 8 && endpoint[:8] == "https://" {
		endpoint = endpoint[8:]
	}
	// Default port if not specified
	if endpoint[len(endpoint)-1] == ':' || !containsPort(endpoint) {
		if endpoint[len(endpoint)-1] != ':' {
			endpoint += ":"
		}
		endpoint += "4317"
	}

	// Create gRPC connection
	conn, err := grpc.NewClient(
		endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithTimeout(timeout),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection: %w", err)
	}

	client := ptraceotlp.NewGRPCClient(conn)

	return &GRPCExporter{
		client:   client,
		endpoint: endpoint,
		tenant:   tenant,
	}, nil
}

// ExportTraces exports traces to Tempo via gRPC
func (e *GRPCExporter) ExportTraces(ctx context.Context, traces ptrace.Traces) error {
	// Add tenant header if configured
	if e.tenant != "" {
		md := metadata.New(map[string]string{
			"X-Scope-OrgID": e.tenant,
		})
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	// Convert to OTLP request
	req := ptraceotlp.NewExportRequestFromTraces(traces)

	// Send request
	_, err := e.client.Export(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to export traces: %w", err)
	}

	return nil
}

// ExportBatch exports multiple traces in a batch
func (e *GRPCExporter) ExportBatch(ctx context.Context, traces []ptrace.Traces) error {
	// Combine all traces into a single request
	combined := ptrace.NewTraces()
	for _, trace := range traces {
		// Merge resource spans
		trace.ResourceSpans().MoveAndAppendTo(combined.ResourceSpans())
	}

	return e.ExportTraces(ctx, combined)
}

// Shutdown closes the exporter
func (e *GRPCExporter) Shutdown(ctx context.Context) error {
	// gRPC client cleanup handled by connection
	return nil
}

func containsPort(endpoint string) bool {
	// Simple check: if endpoint contains : and has digits after it, assume port is specified
	for i := len(endpoint) - 1; i >= 0; i-- {
		if endpoint[i] == ':' {
			return i < len(endpoint)-1
		}
	}
	return false
}
