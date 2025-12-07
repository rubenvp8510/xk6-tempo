package tempo

import (
	"fmt"

	"github.com/rvargasp/xk6-tempo/pkg/generator"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/metrics"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func init() {
	modules.Register("k6/x/tempo", new(RootModule))
}

// tempoMetrics holds all custom metrics for the tempo module
type tempoMetrics struct {
	// Ingestion metrics
	IngestionBytesTotal      *metrics.Metric
	IngestionRateBytesPerSec *metrics.Metric
	IngestionTracesTotal     *metrics.Metric
	IngestionDuration        *metrics.Metric

	// Query metrics
	QueryDuration           *metrics.Metric
	QueryRequestsTotal      *metrics.Metric
	QueryFailuresTotal      *metrics.Metric
	QuerySpansReturned      *metrics.Metric
	QueryFailuresByStatus   *metrics.Metric
	QueryBackoffEvents      *metrics.Metric
	QueryBackoffDuration    *metrics.Metric
	TraceFetchLatency       *metrics.Metric
	TraceFetchFailures      *metrics.Metric
	QueryTimeBucketQueries  *metrics.Metric
	QueryTimeBucketDuration *metrics.Metric
}

// registerMetrics registers all custom metrics with the k6 registry
func registerMetrics(vu modules.VU) (*tempoMetrics, error) {
	var err error
	registry := vu.InitEnv().Registry
	m := &tempoMetrics{}

	// Ingestion metrics
	m.IngestionBytesTotal, err = registry.NewMetric("tempo_ingestion_bytes_total", metrics.Counter, metrics.Data)
	if err != nil {
		return nil, err
	}

	m.IngestionRateBytesPerSec, err = registry.NewMetric("tempo_ingestion_rate_bytes_per_sec", metrics.Rate, metrics.Data)
	if err != nil {
		return nil, err
	}

	m.IngestionTracesTotal, err = registry.NewMetric("tempo_ingestion_traces_total", metrics.Counter, metrics.Default)
	if err != nil {
		return nil, err
	}

	m.IngestionDuration, err = registry.NewMetric("tempo_ingestion_duration_seconds", metrics.Trend, metrics.Time)
	if err != nil {
		return nil, err
	}

	// Query metrics
	m.QueryDuration, err = registry.NewMetric("tempo_query_duration_seconds", metrics.Trend, metrics.Time)
	if err != nil {
		return nil, err
	}

	m.QueryRequestsTotal, err = registry.NewMetric("tempo_query_requests_total", metrics.Counter, metrics.Default)
	if err != nil {
		return nil, err
	}

	m.QueryFailuresTotal, err = registry.NewMetric("tempo_query_failures_total", metrics.Counter, metrics.Default)
	if err != nil {
		return nil, err
	}

	m.QuerySpansReturned, err = registry.NewMetric("tempo_query_spans_returned", metrics.Trend, metrics.Default)
	if err != nil {
		return nil, err
	}

	m.QueryFailuresByStatus, err = registry.NewMetric("tempo_query_failures_by_status", metrics.Counter, metrics.Default)
	if err != nil {
		return nil, err
	}

	m.QueryBackoffEvents, err = registry.NewMetric("tempo_query_backoff_events_total", metrics.Counter, metrics.Default)
	if err != nil {
		return nil, err
	}

	m.QueryBackoffDuration, err = registry.NewMetric("tempo_query_backoff_duration_seconds", metrics.Trend, metrics.Time)
	if err != nil {
		return nil, err
	}

	m.TraceFetchLatency, err = registry.NewMetric("tempo_trace_fetch_latency_seconds", metrics.Trend, metrics.Time)
	if err != nil {
		return nil, err
	}

	m.TraceFetchFailures, err = registry.NewMetric("tempo_trace_fetch_failures_total", metrics.Counter, metrics.Default)
	if err != nil {
		return nil, err
	}

	m.QueryTimeBucketQueries, err = registry.NewMetric("tempo_query_time_bucket_queries_total", metrics.Counter, metrics.Default)
	if err != nil {
		return nil, err
	}

	m.QueryTimeBucketDuration, err = registry.NewMetric("tempo_query_time_bucket_duration_seconds", metrics.Trend, metrics.Time)
	if err != nil {
		return nil, err
	}

	return m, nil
}

// getIntValue safely extracts an integer value from interface{} (handles both int and float64 from JavaScript)
func getIntValue(v interface{}) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case float64:
		return int(val), true
	default:
		return 0, false
	}
}

// RootModule is the global module instance
type RootModule struct{}

// ModuleInstance represents an instance of the module
type ModuleInstance struct {
	vu      modules.VU
	metrics *tempoMetrics
}

// NewModuleInstance implements the modules.Module interface
func (r *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	metrics, err := registerMetrics(vu)
	if err != nil {
		// If we can't register metrics, we should panic as this is a critical error
		panic(fmt.Sprintf("failed to register tempo metrics: %v", err))
	}
	return &ModuleInstance{
		vu:      vu,
		metrics: metrics,
	}
}

// Exports returns the exports of the module
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]interface{}{
			"IngestClient":        mi.newIngestClient,
			"QueryClient":         mi.newQueryClient,
			"generateTrace":       mi.generateTrace,
			"generateBatch":       mi.generateBatch,
			"createRateLimiter":   mi.createRateLimiter,
			"createQueryWorkload": mi.createQueryWorkload,
		},
	}
}

// newIngestClient creates a new Tempo ingestion client
func (mi *ModuleInstance) newIngestClient(config map[string]interface{}) (*IngestClient, error) {
	// Convert map to IngestConfig struct
	cfg := DefaultIngestConfig()
	if endpoint, ok := config["endpoint"].(string); ok && endpoint != "" {
		cfg.Endpoint = endpoint
	}
	if protocol, ok := config["protocol"].(string); ok && protocol != "" {
		cfg.Protocol = protocol
	}
	if tenant, ok := config["tenant"].(string); ok {
		cfg.Tenant = tenant
	}
	if timeout, ok := getIntValue(config["timeout"]); ok && timeout > 0 {
		cfg.Timeout = timeout
	}
	if testName, ok := config["testName"].(string); ok {
		cfg.TestName = testName
	}
	if targetQPS, ok := getIntValue(config["targetQPS"]); ok && targetQPS > 0 {
		cfg.TargetQPS = targetQPS
	}
	if targetMBps, ok := config["targetMBps"].(float64); ok && targetMBps > 0 {
		cfg.TargetMBps = targetMBps
	}

	return NewIngestClient(mi.vu, cfg)
}

// newQueryClient creates a new Tempo query client
func (mi *ModuleInstance) newQueryClient(config map[string]interface{}) (*QueryClient, error) {
	// Convert map to QueryConfig struct
	cfg := DefaultQueryConfig()
	if endpoint, ok := config["endpoint"].(string); ok && endpoint != "" {
		cfg.Endpoint = endpoint
	}
	if tenant, ok := config["tenant"].(string); ok {
		cfg.Tenant = tenant
	}
	if timeout, ok := getIntValue(config["timeout"]); ok && timeout > 0 {
		cfg.Timeout = timeout
	}
	if bearerToken, ok := config["bearerToken"].(string); ok {
		cfg.BearerToken = bearerToken
	}
	if bearerTokenFile, ok := config["bearerTokenFile"].(string); ok {
		cfg.BearerTokenFile = bearerTokenFile
	}

	return NewQueryClient(cfg)
}

// createQueryWorkload creates a query workload manager
func (mi *ModuleInstance) createQueryWorkload(queryClient *QueryClient, workloadConfig map[string]interface{}, queries map[string]interface{}) (*QueryWorkload, error) {
	return CreateQueryWorkload(queryClient, mi.vu, workloadConfig, queries)
}

// generateTrace generates a single trace
func (mi *ModuleInstance) generateTrace(config map[string]interface{}) (ptrace.Traces, error) {
	cfg := generator.DefaultConfig()

	if services, ok := getIntValue(config["services"]); ok && services > 0 {
		cfg.Services = services
	}
	if spanDepth, ok := getIntValue(config["spanDepth"]); ok && spanDepth > 0 {
		cfg.SpanDepth = spanDepth
	}
	if spansPerTrace, ok := getIntValue(config["spansPerTrace"]); ok && spansPerTrace > 0 {
		cfg.SpansPerTrace = spansPerTrace
	}
	if attributeCount, ok := getIntValue(config["attributeCount"]); ok && attributeCount > 0 {
		cfg.AttributeCount = attributeCount
	}
	if attributeValueSize, ok := getIntValue(config["attributeValueSize"]); ok && attributeValueSize > 0 {
		cfg.AttributeValueSize = attributeValueSize
	}
	if eventCount, ok := getIntValue(config["eventCount"]); ok {
		cfg.EventCount = eventCount
	}
	if resourceAttrs, ok := config["resourceAttributes"].(map[string]interface{}); ok {
		cfg.ResourceAttributes = make(map[string]string)
		for k, v := range resourceAttrs {
			if str, ok := v.(string); ok {
				cfg.ResourceAttributes[k] = str
			}
		}
	}
	if durationBaseMs, ok := getIntValue(config["durationBaseMs"]); ok && durationBaseMs > 0 {
		cfg.DurationBaseMs = durationBaseMs
	}
	if durationVarianceMs, ok := getIntValue(config["durationVarianceMs"]); ok && durationVarianceMs >= 0 {
		cfg.DurationVarianceMs = durationVarianceMs
	}
	if errorRate, ok := config["errorRate"].(float64); ok && errorRate >= 0 && errorRate <= 1 {
		cfg.ErrorRate = errorRate
	}
	if maxFanOut, ok := getIntValue(config["maxFanOut"]); ok && maxFanOut > 0 {
		cfg.MaxFanOut = maxFanOut
	}
	if fanOutVariance, ok := config["fanOutVariance"].(float64); ok && fanOutVariance >= 0 && fanOutVariance <= 1 {
		cfg.FanOutVariance = fanOutVariance
	}
	if useSemantic, ok := config["useSemanticAttributes"].(bool); ok {
		cfg.UseSemanticAttributes = useSemantic
	}
	if spanKindWeights, ok := config["spanKindWeights"].(map[string]interface{}); ok {
		cfg.SpanKindWeights = make(map[string]float64)
		for k, v := range spanKindWeights {
			if weight, ok := v.(float64); ok {
				cfg.SpanKindWeights[k] = weight
			}
		}
	}

	return generator.GenerateTrace(cfg), nil
}

// generateBatch generates a batch of traces
func (mi *ModuleInstance) generateBatch(config map[string]interface{}) ([]ptrace.Traces, error) {
	batchConfig := generator.BatchConfig{}

	if targetSize, ok := getIntValue(config["targetSizeBytes"]); ok && targetSize > 0 {
		batchConfig.TargetSizeBytes = targetSize
	} else {
		return nil, fmt.Errorf("targetSizeBytes is required")
	}

	// Parse traceConfig
	traceConfig := generator.DefaultConfig()
	if traceCfgMap, ok := config["traceConfig"].(map[string]interface{}); ok {
		if services, ok := getIntValue(traceCfgMap["services"]); ok && services > 0 {
			traceConfig.Services = services
		}
		if spanDepth, ok := getIntValue(traceCfgMap["spanDepth"]); ok && spanDepth > 0 {
			traceConfig.SpanDepth = spanDepth
		}
		if spansPerTrace, ok := getIntValue(traceCfgMap["spansPerTrace"]); ok && spansPerTrace > 0 {
			traceConfig.SpansPerTrace = spansPerTrace
		}
		if attributeCount, ok := getIntValue(traceCfgMap["attributeCount"]); ok && attributeCount > 0 {
			traceConfig.AttributeCount = attributeCount
		}
		if attributeValueSize, ok := getIntValue(traceCfgMap["attributeValueSize"]); ok && attributeValueSize > 0 {
			traceConfig.AttributeValueSize = attributeValueSize
		}
		if eventCount, ok := getIntValue(traceCfgMap["eventCount"]); ok {
			traceConfig.EventCount = eventCount
		}
		if resourceAttrs, ok := traceCfgMap["resourceAttributes"].(map[string]interface{}); ok {
			traceConfig.ResourceAttributes = make(map[string]string)
			for k, v := range resourceAttrs {
				if str, ok := v.(string); ok {
					traceConfig.ResourceAttributes[k] = str
				}
			}
		}
		if durationBaseMs, ok := getIntValue(traceCfgMap["durationBaseMs"]); ok && durationBaseMs > 0 {
			traceConfig.DurationBaseMs = durationBaseMs
		}
		if durationVarianceMs, ok := getIntValue(traceCfgMap["durationVarianceMs"]); ok && durationVarianceMs >= 0 {
			traceConfig.DurationVarianceMs = durationVarianceMs
		}
		if errorRate, ok := traceCfgMap["errorRate"].(float64); ok && errorRate >= 0 && errorRate <= 1 {
			traceConfig.ErrorRate = errorRate
		}
		if maxFanOut, ok := getIntValue(traceCfgMap["maxFanOut"]); ok && maxFanOut > 0 {
			traceConfig.MaxFanOut = maxFanOut
		}
		if fanOutVariance, ok := traceCfgMap["fanOutVariance"].(float64); ok && fanOutVariance >= 0 && fanOutVariance <= 1 {
			traceConfig.FanOutVariance = fanOutVariance
		}
		if useSemantic, ok := traceCfgMap["useSemanticAttributes"].(bool); ok {
			traceConfig.UseSemanticAttributes = useSemantic
		}
		if spanKindWeights, ok := traceCfgMap["spanKindWeights"].(map[string]interface{}); ok {
			traceConfig.SpanKindWeights = make(map[string]float64)
			for k, v := range spanKindWeights {
				if weight, ok := v.(float64); ok {
					traceConfig.SpanKindWeights[k] = weight
				}
			}
		}
	}
	batchConfig.TraceConfig = traceConfig

	return generator.GenerateBatch(batchConfig), nil
}

// createRateLimiter creates a new byte-based rate limiter
func (mi *ModuleInstance) createRateLimiter(config map[string]interface{}) (*generator.ByteRateLimiter, error) {
	targetMBps := 1.0
	burstMultiplier := 1.5

	if mbps, ok := config["targetMBps"].(float64); ok && mbps > 0 {
		targetMBps = mbps
	}
	if burst, ok := config["burstMultiplier"].(float64); ok && burst > 0 {
		burstMultiplier = burst
	}

	return generator.NewByteRateLimiter(targetMBps, burstMultiplier), nil
}
