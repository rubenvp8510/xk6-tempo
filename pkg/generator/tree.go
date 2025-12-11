package generator

import (
	cryptoRand "crypto/rand"
	"math/rand"
	"time"

	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	tracev1 "go.opentelemetry.io/proto/otlp/trace/v1"
)

// DurationConfig configures duration for a node
type DurationConfig struct {
	BaseMs     int `js:"baseMs"`
	VarianceMs int `js:"varianceMs"`
}

// CountConfig configures repetitions for a child
type CountConfig struct {
	Min int `js:"min"`
	Max int `js:"max"`
}

// TraceTreeNode represents a tree node
type TraceTreeNode struct {
	Service         string            `js:"service"`
	Operation       string            `js:"operation"`
	SpanKind        string            `js:"spanKind"`
	Tags            map[string]string `js:"tags"`
	Duration        DurationConfig    `js:"duration"`
	ErrorRate       float64           `js:"errorRate"`
	ErrorPropagates bool              `js:"errorPropagates"`
	Children        []TraceTreeEdge   `js:"children"`
}

// TraceTreeEdge represents an edge with weight and configuration
type TraceTreeEdge struct {
	Weight   float64        `js:"weight"`   // 0 = equiprobable
	Parallel bool           `js:"parallel"` // Execute in parallel
	Count    CountConfig    `js:"count"`    // Repetitions
	Node     *TraceTreeNode `js:"node"`
}

// TreeContext holds context propagated through the trace
type TreeContext struct {
	Propagate   []string       `js:"propagate"`
	Cardinality map[string]int `js:"cardinality"`
}

// TreeDefaults holds default configuration settings
type TreeDefaults struct {
	UseSemanticAttributes bool    `js:"useSemanticAttributes"`
	EnableTags            bool    `js:"enableTags"`
	TagDensity            float64 `js:"tagDensity"`
}

// TraceTreeConfig holds complete tree configuration
type TraceTreeConfig struct {
	Seed     int64          `js:"seed"` // Seed for reproducibility (0 = random)
	Context  TreeContext    `js:"context"`
	Defaults TreeDefaults   `js:"defaults"`
	Root     *TraceTreeNode `js:"root"`
}

// NormalizeWeights normalizes edge weights to sum to 1
func NormalizeWeights(edges []TraceTreeEdge) {
	defined := 0
	total := 0.0

	for _, e := range edges {
		if e.Weight > 0 {
			defined++
			total += e.Weight
		}
	}

	if defined == 0 {
		// All undefined: equiprobable
		equal := 1.0 / float64(len(edges))
		for i := range edges {
			edges[i].Weight = equal
		}
	} else {
		// Normalize defined weights to 1
		for i := range edges {
			if edges[i].Weight > 0 {
				edges[i].Weight /= total
			}
		}
	}
}

// SelectChildren selects children based on probabilities
func SelectChildren(edges []TraceTreeEdge, rng *rand.Rand) []TraceTreeEdge {
	selected := make([]TraceTreeEdge, 0)

	for _, edge := range edges {
		if rng.Float64() < edge.Weight {
			// Determine how many times to execute
			count := 1
			if edge.Count.Max > 0 {
				if edge.Count.Min < edge.Count.Max {
					count = edge.Count.Min + rng.Intn(edge.Count.Max-edge.Count.Min+1)
				} else {
					count = edge.Count.Min
				}
			}
			for i := 0; i < count; i++ {
				selected = append(selected, edge)
			}
		}
	}
	return selected
}

// filterParallel filters edges by the parallel flag
func filterParallel(edges []TraceTreeEdge, parallel bool) []TraceTreeEdge {
	result := make([]TraceTreeEdge, 0)
	for _, e := range edges {
		if e.Parallel == parallel {
			result = append(result, e)
		}
	}
	return result
}

// GenerateTraceFromTree generates a trace from a configured tree
func GenerateTraceFromTree(config TraceTreeConfig) ptrace.Traces {
	// Initialize RNG with seed if defined
	var rng *rand.Rand
	if config.Seed != 0 {
		rng = rand.New(rand.NewSource(config.Seed))
	} else {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	// Initialize cardinality manager
	cm := GetCardinalityManager()

	// Reset pools if seed is provided for reproducibility
	if config.Seed != 0 {
		cm.ResetPools()
	}

	// Create trace context
	traceCtx := NewTreeTraceContext(config.Context, rng)

	// Generate trace ID (use RNG for reproducibility if seed provided)
	traceID := make([]byte, 16)
	if config.Seed != 0 {
		// Use RNG for reproducibility
		for i := 0; i < 16; i++ {
			traceID[i] = byte(rng.Intn(256))
		}
	} else {
		// Random if no seed
		cryptoRand.Read(traceID)
	}

	// Create traces structure
	traces := ptrace.NewTraces()

	// Trace start time
	traceStartTime := time.Now().Add(-time.Duration(rng.Intn(3600)) * time.Second)

	// Generate spans from tree
	spansByService := make(map[string][]*tracev1.Span)
	generateSpansFromNode(
		config.Root,
		nil, // no parent
		traceID,
		traceStartTime,
		rng,
		config,
		traceCtx,
		spansByService,
	)

	// Group spans by service and create ResourceSpans
	for serviceName, spans := range spansByService {
		rs := traces.ResourceSpans().AppendEmpty()
		resource := rs.Resource()

		// Resource attributes for the service
		resourceAttrs := generateResourceAttributes(serviceName, rng)
		resourceAttrs["service.name"] = serviceName
		for key, value := range resourceAttrs {
			resource.Attributes().PutStr(key, value)
		}

		// Add spans to scope
		scopeSpans := rs.ScopeSpans().AppendEmpty()
		for _, protoSpan := range spans {
			span := scopeSpans.Spans().AppendEmpty()
			spanProtoToPtrace(protoSpan, span)
		}
	}

	return traces
}

// generateSpansFromNode recursively generates spans from a node
func generateSpansFromNode(
	node *TraceTreeNode,
	parentSpan *tracev1.Span,
	traceID []byte,
	parentStartTime time.Time,
	rng *rand.Rand,
	config TraceTreeConfig,
	traceCtx *TreeTraceContext,
	spansByService map[string][]*tracev1.Span,
) *tracev1.Span {
	if node == nil {
		return nil
	}

	// Calculate duration
	duration := calculateDurationFromConfig(node.Duration, rng)

	// Determine start time
	var startTime time.Time
	if parentSpan == nil {
		// Root node
		startTime = parentStartTime
	} else {
		// Child: must start after parent and end before parent
		parentStart := time.Unix(0, int64(parentSpan.StartTimeUnixNano))
		parentEnd := time.Unix(0, int64(parentSpan.EndTimeUnixNano))
		parentDuration := parentEnd.Sub(parentStart)

		// Random delay within parent (up to 30% of parent time)
		delay := time.Duration(rng.Float64() * 0.3 * float64(parentDuration))
		startTime = parentStart.Add(delay)

		// Ensure child ends before parent
		maxEnd := parentEnd.Add(-time.Millisecond * 10)
		if startTime.Add(duration).After(maxEnd) {
			duration = maxEnd.Sub(startTime)
			if duration < time.Millisecond {
				duration = time.Millisecond
			}
		}
	}

	endTime := startTime.Add(duration)

	// Convert spanKind string to enum
	spanKind := parseSpanKind(node.SpanKind)

	// Determine if there's an error
	hasError := rng.Float64() < node.ErrorRate
	status := &tracev1.Status{
		Code: tracev1.Status_STATUS_CODE_OK,
	}
	if hasError {
		status.Code = tracev1.Status_STATUS_CODE_ERROR
		status.Message = getRandomErrorMessage(rng)
	}

	// Create span ID (use RNG for reproducibility)
	spanID := make([]byte, 8)
	for i := 0; i < 8; i++ {
		spanID[i] = byte(rng.Intn(256))
	}

	var parentSpanID []byte
	if parentSpan != nil {
		parentSpanID = parentSpan.SpanId
	}

	span := &tracev1.Span{
		TraceId:           traceID,
		SpanId:            spanID,
		ParentSpanId:      parentSpanID,
		Name:              node.Operation,
		Kind:              spanKind,
		StartTimeUnixNano: uint64(startTime.UnixNano()),
		EndTimeUnixNano:   uint64(endTime.UnixNano()),
		Status:            status,
	}

	// Add attributes
	attrs := make([]*commonv1.KeyValue, 0)

	// Service name
	attrs = append(attrs, &commonv1.KeyValue{
		Key: "service.name",
		Value: &commonv1.AnyValue{
			Value: &commonv1.AnyValue_StringValue{
				StringValue: node.Service,
			},
		},
	})

	// Node tags
	for key, value := range node.Tags {
		attrs = append(attrs, &commonv1.KeyValue{
			Key: key,
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: value,
				},
			},
		})
	}

	// Semantic attributes if enabled
	if config.Defaults.UseSemanticAttributes {
		semanticAttrs := generateSemanticAttributes(spanKind, node.Service, rng)
		attrs = append(attrs, semanticAttrs...)
	}

	// Infrastructure tags if enabled
	if config.Defaults.EnableTags {
		tagAttrs := traceCtx.GetPropagatedTags(config.Defaults.TagDensity, rng)
		attrs = append(attrs, tagAttrs...)
	}

	span.Attributes = attrs

	// Add span to service collection
	if spansByService[node.Service] == nil {
		spansByService[node.Service] = make([]*tracev1.Span, 0)
	}
	spansByService[node.Service] = append(spansByService[node.Service], span)

	// Process children
	if len(node.Children) > 0 {
		// Normalize weights
		NormalizeWeights(node.Children)

		// Select children
		selectedChildren := SelectChildren(node.Children, rng)

		// Separate parallel and sequential
		parallel := filterParallel(selectedChildren, true)
		sequential := filterParallel(selectedChildren, false)

		// Process sequential first
		currentTime := startTime
		for _, childEdge := range sequential {
			childSpan := generateSpansFromNode(
				childEdge.Node,
				span,
				traceID,
				currentTime,
				rng,
				config,
				traceCtx,
				spansByService,
			)
			if childSpan != nil {
				// Update time for next sequential child
				childEnd := time.Unix(0, int64(childSpan.EndTimeUnixNano))
				if childEnd.After(currentTime) {
					currentTime = childEnd
				}
			}
		}

		// Process parallel (can overlap)
		for _, childEdge := range parallel {
			// Random delay within available time
			availableTime := endTime.Sub(currentTime)
			if availableTime > 0 {
				delay := time.Duration(rng.Float64() * 0.2 * float64(availableTime))
				parallelStart := currentTime.Add(delay)

				childSpan := generateSpansFromNode(
					childEdge.Node,
					span,
					traceID,
					parallelStart,
					rng,
					config,
					traceCtx,
					spansByService,
				)

				// If child fails and errorPropagates is active, mark parent as error
				if childSpan != nil && childSpan.Status != nil &&
					childSpan.Status.Code == tracev1.Status_STATUS_CODE_ERROR &&
					childEdge.Node.ErrorPropagates {
					span.Status.Code = tracev1.Status_STATUS_CODE_ERROR
					if span.Status.Message == "" {
						span.Status.Message = "child span failed"
					}
				}
			}
		}
	}

	return span
}

// calculateDurationFromConfig calculates duration from configuration
func calculateDurationFromConfig(dur DurationConfig, rng *rand.Rand) time.Duration {
	base := float64(dur.BaseMs)
	if base <= 0 {
		base = 50 // default
	}
	variance := float64(dur.VarianceMs)
	if variance < 0 {
		variance = 30 // default
	}

	duration := base + rng.NormFloat64()*variance
	if duration < 1 {
		duration = 1
	}
	return time.Duration(duration) * time.Millisecond
}

// parseSpanKind converts string to SpanKind
func parseSpanKind(kindStr string) tracev1.Span_SpanKind {
	switch kindStr {
	case "server":
		return tracev1.Span_SPAN_KIND_SERVER
	case "client":
		return tracev1.Span_SPAN_KIND_CLIENT
	case "internal":
		return tracev1.Span_SPAN_KIND_INTERNAL
	case "producer":
		return tracev1.Span_SPAN_KIND_PRODUCER
	case "consumer":
		return tracev1.Span_SPAN_KIND_CONSUMER
	default:
		return tracev1.Span_SPAN_KIND_SERVER
	}
}

// getRandomErrorMessage returns a random error message
func getRandomErrorMessage(rng *rand.Rand) string {
	errorMessages := []string{
		"connection timeout",
		"database connection failed",
		"invalid request",
		"authentication failed",
		"rate limit exceeded",
		"service unavailable",
		"internal server error",
		"not found",
		"permission denied",
		"request timeout",
	}
	return errorMessages[rng.Intn(len(errorMessages))]
}

// EstimateTreeTraceSize estimates the average byte size of traces generated from a tree configuration
// This generates sampleCount traces, serializes them to protobuf, and returns the average byte size
// If sampleCount is <= 0, it defaults to 40
func EstimateTreeTraceSize(config *TraceTreeConfig, sampleCount int) int {
	if config == nil || config.Root == nil {
		return 0
	}

	// Default to 40 samples if not specified or invalid
	if sampleCount <= 0 {
		sampleCount = 40
	}

	totalSize := 0
	successfulSamples := 0

	// Determine base seed: use the test seed if provided, otherwise use 1 for predictability
	baseSeed := config.Seed
	if baseSeed == 0 {
		baseSeed = 1
	}

	// Generate sampleCount traces with predictable seeds based on the test seed
	for i := 0; i < sampleCount; i++ {
		// Create a copy of config with a predictable seed for each sample
		sampleConfig := *config
		// Use baseSeed + i to ensure variety while maintaining predictability
		// This way, if the test uses seed=100, estimation uses 100, 101, 102, etc.
		sampleConfig.Seed = baseSeed + int64(i)

		// Generate trace
		trace := GenerateTraceFromTree(sampleConfig)

		// Serialize to protobuf and measure size
		req := ptraceotlp.NewExportRequestFromTraces(trace)
		data, err := req.MarshalProto()
		if err != nil {
			// Skip this sample if marshaling fails
			continue
		}

		totalSize += len(data)
		successfulSamples++
	}

	// Return average size (rounded to nearest integer)
	if successfulSamples == 0 {
		return 0
	}
	return totalSize / successfulSamples
}
