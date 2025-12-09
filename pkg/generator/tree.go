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

// DurationConfig configuración de duración para un nodo
type DurationConfig struct {
	BaseMs     int `js:"baseMs"`
	VarianceMs int `js:"varianceMs"`
}

// CountConfig configuración de repeticiones para un hijo
type CountConfig struct {
	Min int `js:"min"`
	Max int `js:"max"`
}

// TraceTreeNode representa un nodo del árbol
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

// TraceTreeEdge representa una arista con peso y configuración
type TraceTreeEdge struct {
	Weight   float64        `js:"weight"`   // 0 = equiprobable
	Parallel bool           `js:"parallel"` // Ejecutar en paralelo
	Count    CountConfig    `js:"count"`    // Repeticiones
	Node     *TraceTreeNode `js:"node"`
}

// TreeContext contexto propagado en la traza
type TreeContext struct {
	Propagate   []string       `js:"propagate"`
	Cardinality map[string]int `js:"cardinality"`
}

// TreeDefaults configuración por defecto
type TreeDefaults struct {
	UseSemanticAttributes bool    `js:"useSemanticAttributes"`
	EnableTags            bool    `js:"enableTags"`
	TagDensity            float64 `js:"tagDensity"`
}

// TraceTreeConfig configuración completa del árbol
type TraceTreeConfig struct {
	Seed     int64          `js:"seed"` // Seed para reproducibilidad (0 = aleatorio)
	Context  TreeContext    `js:"context"`
	Defaults TreeDefaults   `js:"defaults"`
	Root     *TraceTreeNode `js:"root"`
}

// NormalizeWeights normaliza los pesos de las aristas a 1
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
		// Todos sin definir: equiprobables
		equal := 1.0 / float64(len(edges))
		for i := range edges {
			edges[i].Weight = equal
		}
	} else {
		// Normalizar definidos a 1
		for i := range edges {
			if edges[i].Weight > 0 {
				edges[i].Weight /= total
			}
		}
	}
}

// SelectChildren selecciona hijos basado en probabilidades
func SelectChildren(edges []TraceTreeEdge, rng *rand.Rand) []TraceTreeEdge {
	selected := make([]TraceTreeEdge, 0)

	for _, edge := range edges {
		if rng.Float64() < edge.Weight {
			// Determinar cuántas veces ejecutar
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

// filterParallel filtra aristas por el flag parallel
func filterParallel(edges []TraceTreeEdge, parallel bool) []TraceTreeEdge {
	result := make([]TraceTreeEdge, 0)
	for _, e := range edges {
		if e.Parallel == parallel {
			result = append(result, e)
		}
	}
	return result
}

// GenerateTraceFromTree genera una traza desde un árbol configurado
func GenerateTraceFromTree(config TraceTreeConfig) ptrace.Traces {
	// Inicializar RNG con seed si está definido
	var rng *rand.Rand
	if config.Seed != 0 {
		rng = rand.New(rand.NewSource(config.Seed))
	} else {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	// Inicializar cardinality manager
	cm := GetCardinalityManager()
	if len(config.Context.Cardinality) > 0 {
		cm.SetConfig(config.Context.Cardinality)
	}
	// Resetear pools si hay seed para reproducibilidad
	if config.Seed != 0 {
		cm.ResetPools()
	}

	// Crear contexto de traza
	traceCtx := NewTreeTraceContext(config.Context, rng)

	// Generar trace ID (usar RNG para reproducibilidad si hay seed)
	traceID := make([]byte, 16)
	if config.Seed != 0 {
		// Usar RNG para reproducibilidad
		for i := 0; i < 16; i++ {
			traceID[i] = byte(rng.Intn(256))
		}
	} else {
		// Aleatorio si no hay seed
		cryptoRand.Read(traceID)
	}

	// Crear estructura de trazas
	traces := ptrace.NewTraces()

	// Tiempo de inicio de la traza
	traceStartTime := time.Now().Add(-time.Duration(rng.Intn(3600)) * time.Second)

	// Generar spans desde el árbol
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

	// Agrupar spans por servicio y crear ResourceSpans
	for serviceName, spans := range spansByService {
		rs := traces.ResourceSpans().AppendEmpty()
		resource := rs.Resource()

		// Atributos de recurso para el servicio
		resourceAttrs := generateResourceAttributes(serviceName, rng)
		resourceAttrs["service.name"] = serviceName
		for key, value := range resourceAttrs {
			resource.Attributes().PutStr(key, value)
		}

		// Agregar spans al scope
		scopeSpans := rs.ScopeSpans().AppendEmpty()
		for _, protoSpan := range spans {
			span := scopeSpans.Spans().AppendEmpty()
			spanProtoToPtrace(protoSpan, span)
		}
	}

	return traces
}

// generateSpansFromNode genera spans recursivamente desde un nodo
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

	// Calcular duración
	duration := calculateDurationFromConfig(node.Duration, rng)

	// Determinar tiempo de inicio
	var startTime time.Time
	if parentSpan == nil {
		// Nodo raíz
		startTime = parentStartTime
	} else {
		// Hijo: debe empezar después del padre y terminar antes
		parentStart := time.Unix(0, int64(parentSpan.StartTimeUnixNano))
		parentEnd := time.Unix(0, int64(parentSpan.EndTimeUnixNano))
		parentDuration := parentEnd.Sub(parentStart)

		// Delay aleatorio dentro del padre (hasta 30% del tiempo del padre)
		delay := time.Duration(rng.Float64() * 0.3 * float64(parentDuration))
		startTime = parentStart.Add(delay)

		// Asegurar que el hijo termine antes del padre
		maxEnd := parentEnd.Add(-time.Millisecond * 10)
		if startTime.Add(duration).After(maxEnd) {
			duration = maxEnd.Sub(startTime)
			if duration < time.Millisecond {
				duration = time.Millisecond
			}
		}
	}

	endTime := startTime.Add(duration)

	// Convertir spanKind string a enum
	spanKind := parseSpanKind(node.SpanKind)

	// Determinar si hay error
	hasError := rng.Float64() < node.ErrorRate
	status := &tracev1.Status{
		Code: tracev1.Status_STATUS_CODE_OK,
	}
	if hasError {
		status.Code = tracev1.Status_STATUS_CODE_ERROR
		status.Message = getRandomErrorMessage(rng)
	}

	// Crear span ID (usar RNG para reproducibilidad)
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

	// Agregar atributos
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

	// Tags del nodo
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

	// Atributos semánticos si están habilitados
	if config.Defaults.UseSemanticAttributes {
		semanticAttrs := generateSemanticAttributes(spanKind, node.Service, rng)
		attrs = append(attrs, semanticAttrs...)
	}

	// Tags de infraestructura si están habilitados
	if config.Defaults.EnableTags {
		tagAttrs := traceCtx.GetPropagatedTags(config.Defaults.TagDensity, rng)
		attrs = append(attrs, tagAttrs...)
	}

	span.Attributes = attrs

	// Agregar span a la colección por servicio
	if spansByService[node.Service] == nil {
		spansByService[node.Service] = make([]*tracev1.Span, 0)
	}
	spansByService[node.Service] = append(spansByService[node.Service], span)

	// Procesar hijos
	if len(node.Children) > 0 {
		// Normalizar pesos
		NormalizeWeights(node.Children)

		// Seleccionar hijos
		selectedChildren := SelectChildren(node.Children, rng)

		// Separar paralelos y secuenciales
		parallel := filterParallel(selectedChildren, true)
		sequential := filterParallel(selectedChildren, false)

		// Procesar secuenciales primero
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
				// Actualizar tiempo para el siguiente hijo secuencial
				childEnd := time.Unix(0, int64(childSpan.EndTimeUnixNano))
				if childEnd.After(currentTime) {
					currentTime = childEnd
				}
			}
		}

		// Procesar paralelos (pueden solaparse)
		for _, childEdge := range parallel {
			// Delay aleatorio dentro del tiempo disponible
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

				// Si el hijo falla y errorPropagates está activo, marcar el padre como error
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

// calculateDurationFromConfig calcula duración desde configuración
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

// parseSpanKind convierte string a SpanKind
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

// getRandomErrorMessage retorna un mensaje de error aleatorio
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
