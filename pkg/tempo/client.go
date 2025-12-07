package tempo

import (
	"context"
	"fmt"
	"time"

	"github.com/rvargasp/xk6-tempo/pkg/generator"
	"github.com/rvargasp/xk6-tempo/pkg/otlp"
	"go.k6.io/k6/lib"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// Client represents the Tempo client for k6
type Client struct {
	exporter    otlpExporter
	queryClient *QueryClient
	state       *lib.State
	config      Config
	workload    *QueryWorkload
	testContext *TestContext
}

type otlpExporter interface {
	ExportTraces(ctx context.Context, traces ptrace.Traces) error
	ExportBatch(ctx context.Context, traces []ptrace.Traces) error
	Shutdown(ctx context.Context) error
}

// NewClient creates a new Tempo client
func NewClient(state *lib.State, config Config) (*Client, error) {
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

	// Create query client (use same endpoint, but for query API)
	queryClient := NewQueryClient(config.Endpoint, config.Tenant, timeout)

	// Extract test context from config if available
	var testCtx *TestContext
	if config.TestName != "" || config.TargetQPS > 0 || config.TargetMBps > 0 {
		testCtx = &TestContext{
			TestName:   config.TestName,
			TargetQPS:  config.TargetQPS,
			TargetMBps: config.TargetMBps,
		}
	}

	return &Client{
		exporter:    exporter,
		queryClient: queryClient,
		state:       state,
		config:      config,
		testContext: testCtx,
	}, nil
}

// Push pushes a single trace to Tempo
func (c *Client) Push(ctx context.Context, trace ptrace.Traces) error {
	start := time.Now()

	// Calculate size before export
	size := estimateTraceSize(trace)

	err := c.exporter.ExportTraces(ctx, trace)
	duration := time.Since(start)

	// Record metrics
	if err == nil {
		RecordIngestionWithContext(c.state, c.testContext, int64(size), 1, duration)
	}

	return err
}

// PushBatch pushes a batch of traces to Tempo
func (c *Client) PushBatch(ctx context.Context, traces []ptrace.Traces) error {
	return c.PushBatchWithRateLimit(ctx, traces, nil)
}

// PushBatchWithRateLimit pushes a batch of traces to Tempo with rate limiting
func (c *Client) PushBatchWithRateLimit(ctx context.Context, traces []ptrace.Traces, limiter *generator.ByteRateLimiter) error {
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
	if err == nil {
		RecordIngestionWithContext(c.state, c.testContext, int64(totalSize), len(traces), duration)
	}

	return err
}

// Search performs a TraceQL search query
func (c *Client) Search(ctx context.Context, query string, options map[string]interface{}) (*SearchResponse, error) {
	start := time.Now()

	// Convert options
	queryOpts := QueryOptions{}
	if start, ok := options["start"].(string); ok {
		queryOpts.Start = start
	}
	if end, ok := options["end"].(string); ok {
		queryOpts.End = end
	}
	if limit, ok := options["limit"].(int); ok {
		queryOpts.Limit = limit
	}

	result, err := c.queryClient.Search(ctx, query, queryOpts)
	duration := time.Since(start)

	// Record metrics
	spans := 0
	if result != nil {
		// Count spans from all traces
		for range result.Traces {
			// Estimate spans (we don't have exact count from search results)
			spans += 1 // At least one span per trace
		}
	}

	RecordQuery(c.state, duration, spans, err == nil)

	return result, err
}

// GetTrace retrieves a full trace by trace ID
func (c *Client) GetTrace(ctx context.Context, traceID string) (*Trace, error) {
	start := time.Now()

	result, err := c.queryClient.GetTrace(ctx, traceID)
	duration := time.Since(start)

	// Record metrics
	spans := 0
	if result != nil {
		for _, batch := range result.Batches {
			for _, scopeSpan := range batch.ScopeSpans {
				spans += len(scopeSpan.Spans)
			}
		}
	}

	RecordQuery(c.state, duration, spans, err == nil)

	return result, err
}

// CreateQueryWorkload creates a query workload manager
func (c *Client) CreateQueryWorkload(workloadConfig map[string]interface{}, queries map[string]interface{}) (*QueryWorkload, error) {
	// Parse workload config
	cfg := DefaultQueryWorkloadConfig()
	if targetQPS, ok := workloadConfig["targetQPS"].(float64); ok {
		cfg.TargetQPS = targetQPS
	}
	if burstMult, ok := workloadConfig["burstMultiplier"].(float64); ok {
		cfg.BurstMultiplier = burstMult
	}
	if qpsMult, ok := workloadConfig["qpsMultiplier"].(float64); ok {
		cfg.QPSMultiplier = qpsMult
	}
	if enableBackoff, ok := workloadConfig["enableBackoff"].(bool); ok {
		cfg.EnableBackoff = enableBackoff
	}
	if minBackoff, ok := workloadConfig["minBackoffMs"].(int); ok {
		cfg.MinBackoffMs = minBackoff
	}
	if maxBackoff, ok := workloadConfig["maxBackoffMs"].(int); ok {
		cfg.MaxBackoffMs = maxBackoff
	}
	if backoffJitter, ok := workloadConfig["backoffJitter"].(bool); ok {
		cfg.BackoffJitter = backoffJitter
	}
	if traceFetchProb, ok := workloadConfig["traceFetchProbability"].(float64); ok {
		cfg.TraceFetchProbability = traceFetchProb
	}
	if timeWindowJitter, ok := workloadConfig["timeWindowJitterMs"].(int); ok {
		cfg.TimeWindowJitterMs = timeWindowJitter
	}

	// Parse time buckets
	if timeBuckets, ok := workloadConfig["timeBuckets"].([]interface{}); ok {
		cfg.TimeBuckets = make([]TimeBucketConfig, 0, len(timeBuckets))
		for _, tb := range timeBuckets {
			if tbMap, ok := tb.(map[string]interface{}); ok {
				bucket := TimeBucketConfig{
					Weight: 1.0,
				}
				if name, ok := tbMap["name"].(string); ok {
					bucket.Name = name
				}
				if ageStart, ok := tbMap["ageStart"].(string); ok {
					bucket.AgeStart = ageStart
				}
				if ageEnd, ok := tbMap["ageEnd"].(string); ok {
					bucket.AgeEnd = ageEnd
				}
				if weight, ok := tbMap["weight"].(float64); ok {
					bucket.Weight = weight
				}
				cfg.TimeBuckets = append(cfg.TimeBuckets, bucket)
			}
		}
	}

	// Parse execution plan
	if executionPlan, ok := workloadConfig["executionPlan"].([]interface{}); ok {
		cfg.ExecutionPlan = make([]PlanEntry, 0, len(executionPlan))
		for _, ep := range executionPlan {
			if epMap, ok := ep.(map[string]interface{}); ok {
				entry := PlanEntry{
					Weight: 1.0,
				}
				if queryName, ok := epMap["queryName"].(string); ok {
					entry.QueryName = queryName
				}
				if bucketName, ok := epMap["bucketName"].(string); ok {
					entry.BucketName = bucketName
				}
				if weight, ok := epMap["weight"].(float64); ok {
					entry.Weight = weight
				}
				cfg.ExecutionPlan = append(cfg.ExecutionPlan, entry)
			}
		}
	}

	// Parse query definitions
	queryDefs := make(map[string]QueryDefinition)
	for name, q := range queries {
		if qMap, ok := q.(map[string]interface{}); ok {
			def := QueryDefinition{
				Name:  name,
				Limit: 20,
			}
			if query, ok := qMap["query"].(string); ok {
				def.Query = query
			}
			if limit, ok := qMap["limit"].(int); ok {
				def.Limit = limit
			}
			if options, ok := qMap["options"].(map[string]interface{}); ok {
				def.Options = options
			}
			queryDefs[name] = def
		}
	}

	// Create state wrapper
	state := &WorkloadState{
		Metrics: c.state,
	}

	// Create workload
	workload := NewQueryWorkload(cfg, c.queryClient, state, queryDefs)
	c.workload = workload

	return workload, nil
}

// ExecuteWorkloadQuery executes the next query from the workload
func (c *Client) ExecuteWorkloadQuery(ctx context.Context) (*SearchResponse, error) {
	if c.workload == nil {
		return nil, fmt.Errorf("workload not initialized, call CreateQueryWorkload first")
	}
	return c.workload.ExecuteNext(ctx)
}

// ExecuteWorkloadSearchAndFetch executes search and fetch workflow
func (c *Client) ExecuteWorkloadSearchAndFetch(ctx context.Context) error {
	if c.workload == nil {
		return fmt.Errorf("workload not initialized, call CreateQueryWorkload first")
	}
	return c.workload.ExecuteSearchAndFetch(ctx)
}

// estimateTraceSize estimates the size of a trace in bytes
func estimateTraceSize(trace ptrace.Traces) int {
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
