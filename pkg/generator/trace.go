package generator

import (
	cryptoRand "crypto/rand"
	"math/rand"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	tracev1 "go.opentelemetry.io/proto/otlp/trace/v1"
)

// spanInfo holds information about a span for tree building
type spanInfo struct {
	span      *tracev1.Span
	index     int
	depth     int
	children  []int // indices of child spans
	maxChildren int
}

// GenerateTrace generates a single trace based on the configuration
func GenerateTrace(config Config) ptrace.Traces {
	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans().AppendEmpty()
	
	// Set resource attributes
	resource := resourceSpans.Resource()
	
	// Generate resource attributes if not provided
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	resourceAttrs := config.ResourceAttributes
	if len(resourceAttrs) == 0 {
		// Generate default resource attributes
		serviceName := generateServiceName(0)
		resourceAttrs = generateResourceAttributes(serviceName, rng)
		resourceAttrs["service.name"] = serviceName
	}
	
	for key, value := range resourceAttrs {
		resource.Attributes().PutStr(key, value)
	}
	
	// Generate trace ID
	traceID := make([]byte, 16)
	cryptoRand.Read(traceID)
	
	// Generate spans
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
	spans := scopeSpans.Spans()
	
	// Build span tree with variable fan-out
	spansMap := make(map[int]*spanInfo)
	serviceIndex := 0
	
	// Trace start time (all spans relative to this)
	traceStartTime := time.Now().Add(-time.Duration(rng.Intn(3600)) * time.Second)
	
	// Generate root span
	rootSpan := buildSpan(
		traceID,
		nil, // no parent
		0,
		0,
		generateServiceName(serviceIndex),
		config,
		traceStartTime,
		rng,
	)
	
	spansMap[0] = &spanInfo{
		span:         rootSpan,
		index:        0,
		depth:        0,
		children:     make([]int, 0),
		maxChildren: calculateMaxChildren(0, config, rng),
	}
	
	// Generate child spans with variable fan-out
	spansGenerated := 1
	for spansGenerated < config.SpansPerTrace {
		// Select a parent span that can still have children
		parentInfo := selectParentWithFanOut(spansMap, config, rng)
		if parentInfo == nil {
			break
		}
		
		// Check depth limit
		if parentInfo.depth >= config.SpanDepth {
			// Max depth reached, try to find another parent
			parentInfo = findAvailableParent(spansMap, config)
			if parentInfo == nil {
				break
			}
		}
		
		// Check if parent can have more children
		if len(parentInfo.children) >= parentInfo.maxChildren {
			// Parent is full, find another
			parentInfo = findAvailableParent(spansMap, config)
			if parentInfo == nil {
				break
			}
		}
		
		// Calculate child timing (must fit within parent)
		parentSpan := parentInfo.span
		parentStart := time.Unix(0, int64(parentSpan.StartTimeUnixNano))
		parentEnd := time.Unix(0, int64(parentSpan.EndTimeUnixNano))
		parentDuration := parentEnd.Sub(parentStart)
		
		// Child starts after some delay within parent
		delay := time.Duration(rng.Float64() * 0.3 * float64(parentDuration)) // Up to 30% delay
		childStartTime := parentStart.Add(delay)
		
		// Child duration must fit within remaining parent time
		maxChildDuration := parentEnd.Sub(childStartTime) - time.Millisecond*10 // Small buffer
		if maxChildDuration < time.Millisecond {
			maxChildDuration = time.Millisecond
		}
		
		// Temporarily override duration config for this child
		childConfig := config
		childConfig.DurationBaseMs = int(maxChildDuration.Milliseconds() / 2) // Use half of available time
		if childConfig.DurationBaseMs < 1 {
			childConfig.DurationBaseMs = 1
		}
		
		// Rotate service for variety
		serviceIndex = (serviceIndex + 1) % config.Services
		
		childSpan := buildSpan(
			traceID,
			parentSpan.SpanId,
			spansGenerated,
			parentInfo.depth+1,
			generateServiceName(serviceIndex),
			childConfig,
			childStartTime,
			rng,
		)
		
		// Ensure child ends before parent
		childEnd := time.Unix(0, int64(childSpan.EndTimeUnixNano))
		if childEnd.After(parentEnd) {
			childSpan.EndTimeUnixNano = parentSpan.EndTimeUnixNano - uint64(time.Millisecond.Nanoseconds())
		}
		
		childInfo := &spanInfo{
			span:         childSpan,
			index:        spansGenerated,
			depth:        parentInfo.depth + 1,
			children:     make([]int, 0),
			maxChildren: calculateMaxChildren(parentInfo.depth+1, config, rng),
		}
		
		spansMap[spansGenerated] = childInfo
		parentInfo.children = append(parentInfo.children, spansGenerated)
		spansGenerated++
	}
	
	// Convert to ptrace.Span and add to scope spans
	for _, spanInfo := range spansMap {
		span := spans.AppendEmpty()
		spanProtoToPtrace(spanInfo.span, span)
	}
	
	return traces
}

// calculateMaxChildren calculates max children for a span based on depth and config
func calculateMaxChildren(depth int, config Config, rng *rand.Rand) int {
	maxFanOut := config.MaxFanOut
	if maxFanOut <= 0 {
		maxFanOut = 5
	}
	
	// Decrease fan-out as depth increases
	fanOut := maxFanOut
	if depth > 0 {
		fanOut = maxFanOut - depth
		if fanOut < 1 {
			fanOut = 1
		}
	}
	
	// Add variance
	variance := config.FanOutVariance
	if variance < 0 {
		variance = 0
	}
	if variance > 1 {
		variance = 1
	}
	
	// Apply variance: fanOut ± (variance * fanOut)
	adjustment := int(float64(fanOut) * variance * (rng.Float64()*2 - 1)) // -variance to +variance
	fanOut += adjustment
	
	if fanOut < 1 {
		fanOut = 1
	}
	if fanOut > maxFanOut {
		fanOut = maxFanOut
	}
	
	return fanOut
}

// selectParentWithFanOut selects a parent span that can still have children
func selectParentWithFanOut(spansMap map[int]*spanInfo, config Config, rng *rand.Rand) *spanInfo {
	// Collect available parents (those that can still have children)
	available := make([]*spanInfo, 0)
	for _, info := range spansMap {
		if len(info.children) < info.maxChildren && info.depth < config.SpanDepth {
			available = append(available, info)
		}
	}
	
	if len(available) == 0 {
		return nil
	}
	
	// Weight selection towards earlier spans (root and shallow spans)
	// This creates a more realistic tree structure
	weights := make([]float64, len(available))
	totalWeight := 0.0
	for i, info := range available {
		// Weight inversely proportional to depth and child count
		weight := 1.0 / (float64(info.depth) + float64(len(info.children)) + 1.0)
		weights[i] = weight
		totalWeight += weight
	}
	
	// Weighted random selection
	r := rng.Float64() * totalWeight
	currentWeight := 0.0
	for i, info := range available {
		currentWeight += weights[i]
		if r <= currentWeight {
			return info
		}
	}
	
	return available[0]
}

// findAvailableParent finds any parent that can still have children
func findAvailableParent(spansMap map[int]*spanInfo, config Config) *spanInfo {
	for _, info := range spansMap {
		if len(info.children) < info.maxChildren && info.depth < config.SpanDepth {
			return info
		}
	}
	return nil
}

// GenerateBatch generates a batch of traces targeting a specific size in bytes
func GenerateBatch(config BatchConfig) []ptrace.Traces {
	traces := make([]ptrace.Traces, 0)
	currentSize := 0
	
	// Estimate size per trace
	sampleTrace := GenerateTrace(config.TraceConfig)
	sampleSize := estimateTraceSize(sampleTrace)
	
	if sampleSize == 0 {
		// Fallback: generate at least one trace
		traces = append(traces, GenerateTrace(config.TraceConfig))
		return traces
	}
	
	// Calculate how many traces we need
	targetCount := config.TargetSizeBytes / sampleSize
	if targetCount == 0 {
		targetCount = 1
	}
	
	// Generate traces until we reach target size
	for currentSize < config.TargetSizeBytes {
		trace := GenerateTrace(config.TraceConfig)
		traceSize := estimateTraceSize(trace)
		
		if currentSize+traceSize > config.TargetSizeBytes && len(traces) > 0 {
			// Adding this trace would exceed target, stop
			break
		}
		
		traces = append(traces, trace)
		currentSize += traceSize
		
		// Safety limit
		if len(traces) > 10000 {
			break
		}
	}
	
	return traces
}

// Helper functions


func calculateDepth(spanIndex, totalSpans int) int {
	if spanIndex == 0 {
		return 0
	}
	depth := 0
	current := spanIndex
	for current > 0 {
		current = (current - 1) / 2
		depth++
	}
	return depth
}

func spanProtoToPtrace(proto *tracev1.Span, ptraceSpan ptrace.Span) {
	// Convert []byte to TraceID/SpanID
	var traceID pcommon.TraceID
	copy(traceID[:], proto.TraceId)
	ptraceSpan.SetTraceID(traceID)
	
	var spanID pcommon.SpanID
	copy(spanID[:], proto.SpanId)
	ptraceSpan.SetSpanID(spanID)
	
	if len(proto.ParentSpanId) > 0 {
		var parentSpanID pcommon.SpanID
		copy(parentSpanID[:], proto.ParentSpanId)
		ptraceSpan.SetParentSpanID(parentSpanID)
	}
	ptraceSpan.SetName(proto.Name)
	ptraceSpan.SetKind(ptrace.SpanKind(proto.Kind))
	ptraceSpan.SetStartTimestamp(pcommon.Timestamp(proto.StartTimeUnixNano))
	ptraceSpan.SetEndTimestamp(pcommon.Timestamp(proto.EndTimeUnixNano))
	
	// Set status
	if proto.Status != nil {
		ptraceSpan.Status().SetCode(ptrace.StatusCode(proto.Status.Code))
		if proto.Status.Message != "" {
			ptraceSpan.Status().SetMessage(proto.Status.Message)
		}
	}
	
	// Set attributes
	for _, attr := range proto.Attributes {
		if strVal := attr.Value.GetStringValue(); strVal != "" {
			ptraceSpan.Attributes().PutStr(attr.Key, strVal)
		} else if intVal := attr.Value.GetIntValue(); intVal != 0 {
			ptraceSpan.Attributes().PutInt(attr.Key, intVal)
		} else if boolVal := attr.Value.GetBoolValue(); boolVal {
			ptraceSpan.Attributes().PutBool(attr.Key, boolVal)
		} else if doubleVal := attr.Value.GetDoubleValue(); doubleVal != 0 {
			ptraceSpan.Attributes().PutDouble(attr.Key, doubleVal)
		}
	}
	
	// Set events
	for _, event := range proto.Events {
		eventPtrace := ptraceSpan.Events().AppendEmpty()
		eventPtrace.SetName(event.Name)
		eventPtrace.SetTimestamp(pcommon.Timestamp(event.TimeUnixNano))
		for _, attr := range event.Attributes {
			if strVal := attr.Value.GetStringValue(); strVal != "" {
				eventPtrace.Attributes().PutStr(attr.Key, strVal)
			}
		}
	}
}

func estimateTraceSize(trace ptrace.Traces) int {
	size := 0
	if trace.ResourceSpans().Len() == 0 {
		return 0
	}
	resourceSpans := trace.ResourceSpans().At(0)
	if resourceSpans.ScopeSpans().Len() == 0 {
		return 0
	}
	scopeSpans := resourceSpans.ScopeSpans().At(0)
	spans := scopeSpans.Spans()
	for i := 0; i < spans.Len(); i++ {
		span := spans.At(i)
		// Rough estimate: count attributes, events, and basic span data
		size += 100 // Base span overhead
		attrs := span.Attributes()
		attrs.Range(func(key string, value pcommon.Value) bool {
			size += len(key) + len(value.AsString())
			return true
		})
		events := span.Events()
		for j := 0; j < events.Len(); j++ {
			event := events.At(j)
			size += len(event.Name()) + 50
		}
	}
	return size
}

