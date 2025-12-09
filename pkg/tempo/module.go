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
			"estimateTraceSize":   mi.estimateTraceSize,
			"calculateThroughput": mi.calculateThroughput,
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

	return NewIngestClient(mi.vu, cfg, mi.metrics)
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
	return CreateQueryWorkload(queryClient, mi.vu, mi.metrics, workloadConfig, queries)
}

// generateTrace generates a single trace
func (mi *ModuleInstance) generateTrace(config map[string]interface{}) (ptrace.Traces, error) {
	cfg := generator.DefaultConfig()
	populateConfigFromMap(&cfg, config)
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
		populateConfigFromMap(&traceConfig, traceCfgMap)
		
		// Handle special case for goja.Value conversion
		if _, ok := traceCfgMap["useWorkflows"].(bool); !ok {
			if val := traceCfgMap["useWorkflows"]; val != nil {
				if strVal, ok := val.(string); ok && strVal == "true" {
					traceConfig.UseWorkflows = true
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

// estimateTraceSize estimates the size of a trace in bytes based on configuration
func (mi *ModuleInstance) estimateTraceSize(config map[string]interface{}) (int, error) {
	cfg := parseConfigFromMap(config)
	return generator.EstimateTraceSizeFromConfig(cfg), nil
}

// calculateThroughput calculates the number of traces per second per VU needed to achieve target bytes/s
func (mi *ModuleInstance) calculateThroughput(config map[string]interface{}, targetBytesPerSec interface{}, numVUs interface{}) (map[string]interface{}, error) {
	cfg := parseConfigFromMap(config)

	// Convert targetBytesPerSec to float64
	var bytesPerSec float64
	switch v := targetBytesPerSec.(type) {
	case float64:
		bytesPerSec = v
	case int:
		bytesPerSec = float64(v)
	case int64:
		bytesPerSec = float64(v)
	default:
		bytesPerSec = 1024 * 1024 // Default 1 MB/s
	}

	// Convert numVUs to int
	var vus int
	if vusVal, ok := getIntValue(numVUs); ok {
		vus = vusVal
	} else {
		vus = 1 // Default
	}

	throughput := generator.CalculateThroughput(cfg, bytesPerSec, vus)

	return map[string]interface{}{
		"targetBytesPerSec": throughput.TargetBytesPerSec,
		"tracesPerVU":       throughput.TracesPerVU,
		"estimatedSizeB":    throughput.EstimatedSizeB,
		"totalTracesPerSec": throughput.TotalTracesPerSec,
	}, nil
}

// parseConfigFromMap parses a Config from a JavaScript map (helper function)
func parseConfigFromMap(config map[string]interface{}) generator.Config {
	cfg := generator.DefaultConfig()
	populateConfigFromMap(&cfg, config)
	return cfg
}

// populateConfigFromMap populates a generator.Config from a JavaScript map
// This is a helper to reduce duplication between generateTrace, generateBatch, and calculateThroughput
func populateConfigFromMap(cfg *generator.Config, config map[string]interface{}) {
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
	// Workflow configuration
	if useWorkflows, ok := config["useWorkflows"].(bool); ok {
		cfg.UseWorkflows = useWorkflows
	}
	if workflowWeights, ok := config["workflowWeights"].(map[string]interface{}); ok {
		cfg.WorkflowWeights = make(map[string]float64)
		for k, v := range workflowWeights {
			if weight, ok := v.(float64); ok {
				cfg.WorkflowWeights[k] = weight
			}
		}
	}
	if businessDensity, ok := config["businessAttributesDensity"].(float64); ok {
		cfg.BusinessAttributesDensity = businessDensity
	}
	// Tag configuration
	if enableTags, ok := config["enableTags"].(bool); ok {
		cfg.EnableTags = enableTags
	}
	if tagDensity, ok := config["tagDensity"].(float64); ok {
		cfg.TagDensity = tagDensity
	}
	// Cardinality configuration
	if cardinalityConfig, ok := config["cardinalityConfig"].(map[string]interface{}); ok {
		cfg.CardinalityConfig = make(map[string]int)
		for k, v := range cardinalityConfig {
			if cardinality, ok := getIntValue(v); ok {
				cfg.CardinalityConfig[k] = cardinality
			}
		}
	}
	// Tree-based generation
	if useTraceTree, ok := config["useTraceTree"].(bool); ok && useTraceTree {
		if traceTreeObj, ok := config["traceTree"].(map[string]interface{}); ok {
			treeConfig, err := parseTraceTree(traceTreeObj)
			if err == nil {
				cfg.UseTraceTree = true
				cfg.TraceTreeConfig = treeConfig
			}
		}
	}
}

// parseTraceTree parses a trace tree from a JavaScript object
func parseTraceTree(jsObj map[string]interface{}) (*generator.TraceTreeConfig, error) {
	config := &generator.TraceTreeConfig{}

	// Parse seed
	if seed, ok := getIntValue(jsObj["seed"]); ok {
		config.Seed = int64(seed)
	} else if seedFloat, ok := jsObj["seed"].(float64); ok {
		config.Seed = int64(seedFloat)
	}

	// Parse context
	if contextObj, ok := jsObj["context"].(map[string]interface{}); ok {
		ctx := generator.TreeContext{}

		// Parse propagate
		if propagateArr, ok := contextObj["propagate"].([]interface{}); ok {
			ctx.Propagate = make([]string, 0, len(propagateArr))
			for _, v := range propagateArr {
				if str, ok := v.(string); ok {
					ctx.Propagate = append(ctx.Propagate, str)
				}
			}
		}

		// Parse cardinality
		if cardinalityObj, ok := contextObj["cardinality"].(map[string]interface{}); ok {
			ctx.Cardinality = make(map[string]int)
			for k, v := range cardinalityObj {
				if cardinality, ok := getIntValue(v); ok {
					ctx.Cardinality[k] = cardinality
				}
			}
		}

		config.Context = ctx
	}

	// Parse defaults
	if defaultsObj, ok := jsObj["defaults"].(map[string]interface{}); ok {
		defs := generator.TreeDefaults{
			UseSemanticAttributes: true,
			EnableTags:            true,
			TagDensity:            0.9,
		}

		if useSemantic, ok := defaultsObj["useSemanticAttributes"].(bool); ok {
			defs.UseSemanticAttributes = useSemantic
		}
		if enableTags, ok := defaultsObj["enableTags"].(bool); ok {
			defs.EnableTags = enableTags
		}
		if tagDensity, ok := defaultsObj["tagDensity"].(float64); ok {
			defs.TagDensity = tagDensity
		}

		config.Defaults = defs
	} else {
		// Default defaults
		config.Defaults = generator.TreeDefaults{
			UseSemanticAttributes: true,
			EnableTags:            true,
			TagDensity:            0.9,
		}
	}

	// Parse root node
	if rootObj, ok := jsObj["root"].(map[string]interface{}); ok {
		rootNode, err := parseTraceTreeNode(rootObj)
		if err != nil {
			return nil, fmt.Errorf("failed to parse root node: %v", err)
		}
		config.Root = rootNode
	} else {
		return nil, fmt.Errorf("root node is required")
	}

	return config, nil
}

// parseTraceTreeNode parses a tree node
func parseTraceTreeNode(jsObj map[string]interface{}) (*generator.TraceTreeNode, error) {
	node := &generator.TraceTreeNode{}

	// Service (required)
	if service, ok := jsObj["service"].(string); ok {
		node.Service = service
	} else {
		return nil, fmt.Errorf("service is required for node")
	}

	// Operation
	if operation, ok := jsObj["operation"].(string); ok {
		node.Operation = operation
	}

	// SpanKind
	if spanKind, ok := jsObj["spanKind"].(string); ok {
		node.SpanKind = spanKind
	} else {
		node.SpanKind = "server" // default
	}

	// Tags
	if tagsObj, ok := jsObj["tags"].(map[string]interface{}); ok {
		node.Tags = make(map[string]string)
		for k, v := range tagsObj {
			if str, ok := v.(string); ok {
				node.Tags[k] = str
			}
		}
	}

	// Duration
	if durationObj, ok := jsObj["duration"].(map[string]interface{}); ok {
		dur := generator.DurationConfig{}
		if baseMs, ok := getIntValue(durationObj["baseMs"]); ok {
			dur.BaseMs = baseMs
		} else if baseMsFloat, ok := durationObj["baseMs"].(float64); ok {
			dur.BaseMs = int(baseMsFloat)
		}
		if varianceMs, ok := getIntValue(durationObj["varianceMs"]); ok {
			dur.VarianceMs = varianceMs
		} else if varianceMsFloat, ok := durationObj["varianceMs"].(float64); ok {
			dur.VarianceMs = int(varianceMsFloat)
		}
		node.Duration = dur
	}

	// ErrorRate
	if errorRate, ok := jsObj["errorRate"].(float64); ok {
		node.ErrorRate = errorRate
	}

	// ErrorPropagates
	if errorPropagates, ok := jsObj["errorPropagates"].(bool); ok {
		node.ErrorPropagates = errorPropagates
	}

	// Children
	if childrenArr, ok := jsObj["children"].([]interface{}); ok {
		node.Children = make([]generator.TraceTreeEdge, 0, len(childrenArr))
		for _, childObj := range childrenArr {
			if childMap, ok := childObj.(map[string]interface{}); ok {
				edge, err := parseTraceTreeEdge(childMap)
				if err != nil {
					return nil, fmt.Errorf("failed to parse child edge: %v", err)
				}
				node.Children = append(node.Children, *edge)
			}
		}
	}

	return node, nil
}

// parseTraceTreeEdge parses a tree edge
func parseTraceTreeEdge(jsObj map[string]interface{}) (*generator.TraceTreeEdge, error) {
	edge := &generator.TraceTreeEdge{}

	// Weight
	if weight, ok := jsObj["weight"].(float64); ok {
		edge.Weight = weight
	}

	// Parallel
	if parallel, ok := jsObj["parallel"].(bool); ok {
		edge.Parallel = parallel
	}

	// Count
	if countObj, ok := jsObj["count"].(map[string]interface{}); ok {
		count := generator.CountConfig{Min: 1, Max: 1}
		if min, ok := getIntValue(countObj["min"]); ok {
			count.Min = min
		} else if minFloat, ok := countObj["min"].(float64); ok {
			count.Min = int(minFloat)
		}
		if max, ok := getIntValue(countObj["max"]); ok {
			count.Max = max
		} else if maxFloat, ok := countObj["max"].(float64); ok {
			count.Max = int(maxFloat)
		}
		edge.Count = count
	}

	// Node
	if nodeObj, ok := jsObj["node"].(map[string]interface{}); ok {
		node, err := parseTraceTreeNode(nodeObj)
		if err != nil {
			return nil, fmt.Errorf("failed to parse node: %v", err)
		}
		edge.Node = node
	} else {
		return nil, fmt.Errorf("node is required for edge")
	}

	return edge, nil
}
