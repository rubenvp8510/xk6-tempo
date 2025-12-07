package generator

import (
	cryptoRand "crypto/rand"
	"encoding/hex"
	"fmt"
	"math/rand"
	"time"

	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	tracev1 "go.opentelemetry.io/proto/otlp/trace/v1"
)

// generateSpanID generates a random span ID
func generateSpanID() []byte {
	id := make([]byte, 8)
	cryptoRand.Read(id)
	return id
}

// generateTraceID generates a random trace ID
func generateTraceID() []byte {
	id := make([]byte, 16)
	rand.Read(id)
	return id
}

// generateAttributeValue generates a random attribute value of specified size
func generateAttributeValue(size int) string {
	if size <= 0 {
		return ""
	}
	bytes := make([]byte, size)
	cryptoRand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// calculateDuration calculates span duration with variance
func calculateDuration(config Config, rng *rand.Rand) time.Duration {
	base := float64(config.DurationBaseMs)
	if base <= 0 {
		base = 50
	}
	variance := float64(config.DurationVarianceMs)
	if variance < 0 {
		variance = 30
	}

	// Normal distribution: base + (random * variance)
	duration := base + rng.NormFloat64()*variance
	if duration < 1 {
		duration = 1
	}
	return time.Duration(duration) * time.Millisecond
}

// selectSpanKind selects a span kind based on weighted distribution
func selectSpanKind(config Config, serviceName string, rng *rand.Rand) tracev1.Span_SpanKind {
	if len(config.SpanKindWeights) == 0 {
		// Default to server if no weights configured
		return tracev1.Span_SPAN_KIND_SERVER
	}

	// Normalize weights
	totalWeight := 0.0
	for _, weight := range config.SpanKindWeights {
		totalWeight += weight
	}

	if totalWeight == 0 {
		return tracev1.Span_SPAN_KIND_SERVER
	}

	// Weighted random selection
	r := rng.Float64() * totalWeight
	currentWeight := 0.0

	for kindStr, weight := range config.SpanKindWeights {
		currentWeight += weight
		if r <= currentWeight {
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
	}

	return tracev1.Span_SPAN_KIND_SERVER
}

// generateStatus generates span status with error injection
func generateStatus(config Config, rng *rand.Rand) *tracev1.Status {
	errorRate := config.ErrorRate
	if errorRate < 0 {
		errorRate = 0
	}
	if errorRate > 1 {
		errorRate = 1
	}

	if rng.Float64() < errorRate {
		// Generate error
		message := errorMessages[rng.Intn(len(errorMessages))]
		return &tracev1.Status{
			Code:    tracev1.Status_STATUS_CODE_ERROR,
			Message: message,
		}
	}

	return &tracev1.Status{
		Code: tracev1.Status_STATUS_CODE_OK,
	}
}

// buildSpan creates a span with the given configuration
func buildSpan(
	traceID []byte,
	parentSpanID []byte,
	spanIndex int,
	depth int,
	serviceName string,
	config Config,
	startTime time.Time,
	rng *rand.Rand,
) *tracev1.Span {
	spanID := generateSpanID()

	// Generate realistic operation name
	spanName := generateOperationName(serviceName, rng)

	// Calculate duration with variance
	duration := calculateDuration(config, rng)
	endTime := startTime.Add(duration)

	// Select span kind
	kind := selectSpanKind(config, serviceName, rng)

	// Generate status (with error injection)
	status := generateStatus(config, rng)

	span := &tracev1.Span{
		TraceId:           traceID,
		SpanId:            spanID,
		ParentSpanId:      parentSpanID,
		Name:              spanName,
		Kind:              kind,
		StartTimeUnixNano: uint64(startTime.UnixNano()),
		EndTimeUnixNano:   uint64(endTime.UnixNano()),
		Status:            status,
	}

	// Add attributes
	attrs := make([]*commonv1.KeyValue, 0)

	// Standard attributes
	attrs = append(attrs, &commonv1.KeyValue{
		Key: "service.name",
		Value: &commonv1.AnyValue{
			Value: &commonv1.AnyValue_StringValue{
				StringValue: serviceName,
			},
		},
	})

	attrs = append(attrs, &commonv1.KeyValue{
		Key: "span.depth",
		Value: &commonv1.AnyValue{
			Value: &commonv1.AnyValue_IntValue{
				IntValue: int64(depth),
			},
		},
	})

	// Add semantic attributes if enabled
	if config.UseSemanticAttributes {
		semanticAttrs := generateSemanticAttributes(kind, serviceName, rng)
		attrs = append(attrs, semanticAttrs...)
	}

	// Generate custom attributes
	for i := 0; i < config.AttributeCount; i++ {
		key := fmt.Sprintf("attribute.%d", i)
		value := generateAttributeValue(config.AttributeValueSize)
		attrs = append(attrs, &commonv1.KeyValue{
			Key: key,
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: value,
				},
			},
		})
	}

	span.Attributes = attrs

	// Add events if configured
	if config.EventCount > 0 {
		events := make([]*tracev1.Span_Event, 0, config.EventCount)
		for i := 0; i < config.EventCount; i++ {
			eventTime := startTime.Add(time.Duration(i) * duration / time.Duration(config.EventCount))
			events = append(events, &tracev1.Span_Event{
				TimeUnixNano: uint64(eventTime.UnixNano()),
				Name:         fmt.Sprintf("event-%d", i),
				Attributes: []*commonv1.KeyValue{
					{
						Key: "event.type",
						Value: &commonv1.AnyValue{
							Value: &commonv1.AnyValue_StringValue{
								StringValue: "log",
							},
						},
					},
				},
			})
		}
		span.Events = events
	}

	return span
}

// calculateSpanSize estimates the size of a span in bytes
func calculateSpanSize(span *tracev1.Span) int {
	size := 0
	
	// Trace ID, Span ID, Parent Span ID
	size += len(span.TraceId) + len(span.SpanId) + len(span.ParentSpanId)
	
	// Name
	size += len(span.Name)
	
	// Attributes
	for _, attr := range span.Attributes {
		size += len(attr.Key)
		if strVal := attr.Value.GetStringValue(); strVal != "" {
			size += len(strVal)
		}
		if intVal := attr.Value.GetIntValue(); intVal != 0 {
			size += 8 // int64
		}
	}
	
	// Events
	for _, event := range span.Events {
		size += len(event.Name)
		for _, attr := range event.Attributes {
			size += len(attr.Key)
			if strVal := attr.Value.GetStringValue(); strVal != "" {
				size += len(strVal)
			}
		}
	}
	
	return size
}

// generateServiceName generates a service name based on index
func generateServiceName(index int) string {
	services := []string{
		"frontend",
		"backend",
		"database",
		"cache",
		"auth",
		"payment",
		"shipping",
		"analytics",
		"notification",
		"gateway",
	}
	if index < len(services) {
		return services[index]
	}
	return fmt.Sprintf("service-%d", index)
}

