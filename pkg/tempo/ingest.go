package tempo

import (
	"context"
	"fmt"
	"time"

	"github.com/rvargasp/xk6-tempo/pkg/generator"
	"github.com/rvargasp/xk6-tempo/pkg/otlp"
	"go.k6.io/k6/lib"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
)

// IngestClient represents the Tempo ingestion client for k6
type IngestClient struct {
	exporter    otlpExporter
	vu          VU
	config      IngestConfig
	testContext *TestContext
	metrics     *tempoMetrics
}

// VU is an interface for k6 VU to avoid import cycles
type VU interface {
	State() *lib.State
}

type otlpExporter interface {
	ExportTraces(ctx context.Context, traces ptrace.Traces) error
	ExportBatch(ctx context.Context, traces []ptrace.Traces) error
	Shutdown(ctx context.Context) error
}

// NewIngestClient creates a new Tempo ingestion client
func NewIngestClient(vu VU, config IngestConfig, m *tempoMetrics) (*IngestClient, error) {
	timeout := time.Duration(config.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	var exporter otlpExporter
	var err error

	switch config.Protocol {
	case "otlp-grpc":
		exporter, err = otlp.NewGRPCExporter(config.Endpoint, config.Tenant, timeout)
		if err != nil {
			return nil, fmt.Errorf("failed to create gRPC exporter: %w", err)
		}
	case "otlp-http", "":
		exporter = otlp.NewHTTPExporter(config.Endpoint, config.Tenant, timeout)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s (use 'otlp-http' or 'otlp-grpc')", config.Protocol)
	}

	// Extract test context from config if available
	var testCtx *TestContext
	if config.TestName != "" || config.TargetQPS > 0 || config.TargetMBps > 0 {
		testCtx = &TestContext{
			TestName:   config.TestName,
			TargetQPS:  config.TargetQPS,
			TargetMBps: config.TargetMBps,
		}
	}

	return &IngestClient{
		exporter:    exporter,
		vu:          vu,
		config:      config,
		testContext: testCtx,
		metrics:     m,
	}, nil
}

// push pushes a single trace to Tempo (internal, requires context)
func (c *IngestClient) push(ctx context.Context, trace ptrace.Traces) error {
	start := time.Now()

	// Calculate size before export
	size := estimateTraceSize(trace)

	err := c.exporter.ExportTraces(ctx, trace)
	duration := time.Since(start)

	// Record metrics
	if err == nil && c.vu.State() != nil {
		RecordIngestionWithContext(c.vu.State(), c.metrics, c.testContext, int64(size), 1, duration)
	}

	return err
}

// pushBatchInternal pushes a batch of traces to Tempo (internal, requires context)
func (c *IngestClient) pushBatchInternal(ctx context.Context, traces []ptrace.Traces) error {
	return c.pushBatchWithRateLimitInternal(ctx, traces, nil)
}

// pushBatchWithRateLimitInternal pushes a batch of traces to Tempo with rate limiting (internal, requires context)
func (c *IngestClient) pushBatchWithRateLimitInternal(ctx context.Context, traces []ptrace.Traces, limiter *generator.ByteRateLimiter) error {
	start := time.Now()

	// Calculate total size
	totalSize := 0
	for _, trace := range traces {
		totalSize += estimateTraceSize(trace)
	}

	// Apply rate limiting if provided
	if limiter != nil {
		if err := limiter.Wait(ctx, totalSize); err != nil {
			return fmt.Errorf("rate limiter wait failed: %w", err)
		}
	}

	err := c.exporter.ExportBatch(ctx, traces)
	duration := time.Since(start)

	// Record metrics
	if err == nil && c.vu.State() != nil {
		RecordIngestionWithContext(c.vu.State(), c.metrics, c.testContext, int64(totalSize), len(traces), duration)
	}

	return err
}

// JavaScript-friendly wrapper methods (exported, no context parameter required)

// Push pushes a single trace to Tempo (JavaScript-friendly)
func (c *IngestClient) Push(trace ptrace.Traces) error {
	ctx := context.Background()
	return c.push(ctx, trace)
}

// PushBatch pushes a batch of traces to Tempo (JavaScript-friendly)
func (c *IngestClient) PushBatch(traces []ptrace.Traces) error {
	ctx := context.Background()
	return c.pushBatchInternal(ctx, traces)
}

// PushBatchWithRateLimit pushes a batch of traces to Tempo with rate limiting (JavaScript-friendly)
func (c *IngestClient) PushBatchWithRateLimit(traces []ptrace.Traces, limiter *generator.ByteRateLimiter) error {
	ctx := context.Background()
	return c.pushBatchWithRateLimitInternal(ctx, traces, limiter)
}

// estimateTraceSize calculates the actual protobuf-serialized size of a trace in bytes
func estimateTraceSize(trace ptrace.Traces) int {
	req := ptraceotlp.NewExportRequestFromTraces(trace)
	data, err := req.MarshalProto()
	if err != nil {
		// Fallback to rough estimate if marshaling fails
		return estimateTraceSizeRough(trace)
	}
	return len(data)
}

// estimateTraceSizeRough provides a rough estimate as fallback
func estimateTraceSizeRough(trace ptrace.Traces) int {
	size := 0
	for i := 0; i < trace.ResourceSpans().Len(); i++ {
		resourceSpans := trace.ResourceSpans().At(i)
		for j := 0; j < resourceSpans.ScopeSpans().Len(); j++ {
			scopeSpans := resourceSpans.ScopeSpans().At(j)
			spans := scopeSpans.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				// Base span overhead
				size += 100
				// Attributes - rough estimate
				size += span.Attributes().Len() * 50
				// Events
				size += span.Events().Len() * 50
			}
		}
	}
	return size
}
