package otlp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
)

// HTTPExporter exports traces via OTLP HTTP
type HTTPExporter struct {
	client     *http.Client
	endpoint   string
	tenant     string
	headers    map[string]string
}

// NewHTTPExporter creates a new HTTP exporter
func NewHTTPExporter(endpoint string, tenant string, timeout time.Duration) *HTTPExporter {
	// Ensure endpoint ends with /v1/traces
	if endpoint[len(endpoint)-1] != '/' {
		endpoint += "/"
	}
	endpoint += "v1/traces"

	headers := make(map[string]string)
	headers["Content-Type"] = "application/x-protobuf"
	if tenant != "" {
		headers["X-Scope-OrgID"] = tenant
	}

	return &HTTPExporter{
		client: &http.Client{
			Timeout: timeout,
		},
		endpoint: endpoint,
		tenant:   tenant,
		headers:  headers,
	}
}

// ExportTraces exports traces to Tempo via HTTP
func (e *HTTPExporter) ExportTraces(ctx context.Context, traces ptrace.Traces) error {
	// Convert ptrace.Traces to OTLP request
	req := ptraceotlp.NewExportRequestFromTraces(traces)
	
	// Serialize to protobuf
	data, err := req.MarshalProto()
	if err != nil {
		return fmt.Errorf("failed to marshal traces: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", e.endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for key, value := range e.headers {
		httpReq.Header.Set(key, value)
	}

	// Send request
	resp, err := e.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ExportBatch exports multiple traces in a batch
func (e *HTTPExporter) ExportBatch(ctx context.Context, traces []ptrace.Traces) error {
	// Combine all traces into a single request
	combined := ptrace.NewTraces()
	for _, trace := range traces {
		// Merge resource spans
		trace.ResourceSpans().MoveAndAppendTo(combined.ResourceSpans())
	}
	
	return e.ExportTraces(ctx, combined)
}

// Shutdown closes the exporter
func (e *HTTPExporter) Shutdown(ctx context.Context) error {
	// HTTP client doesn't need explicit shutdown
	return nil
}

