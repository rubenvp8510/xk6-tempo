package generator

const (
	bytesPerMegabyte = 1024 * 1024
	defaultFallbackTraceSize = 1000
)

// Config represents the configuration for trace generation
type Config struct {
	Services           int               `js:"services"`           // Number of distinct services
	SpanDepth          int               `js:"spanDepth"`          // Max span tree depth
	SpansPerTrace      int               `js:"spansPerTrace"`      // Total spans per trace
	AttributeCount     int               `js:"attributeCount"`     // Number of attributes per span
	AttributeValueSize int               `js:"attributeValueSize"` // Size of attribute values in bytes
	EventCount         int               `js:"eventCount"`         // Number of events/logs per span
	ResourceAttributes map[string]string `js:"resourceAttributes"` // Resource-level attributes

	// Variance settings
	DurationBaseMs     int     `js:"durationBaseMs"`     // Base duration in ms (default: 50)
	DurationVarianceMs int     `js:"durationVarianceMs"` // Std deviation for duration (default: 30)

	// Error injection
	ErrorRate float64 `js:"errorRate"` // Probability of error status (0.0-1.0, default: 0.02)

	// Span kind distribution (weights, normalized internally)
	SpanKindWeights map[string]float64 `js:"spanKindWeights"` // e.g., {"server": 0.4, "client": 0.4, "internal": 0.2}

	// Trace shape variance
	MaxFanOut      int     `js:"maxFanOut"`      // Max children per span (default: 5)
	FanOutVariance float64 `js:"fanOutVariance"` // Variance in fan-out (0.0-1.0)

	// Semantic attributes
	UseSemanticAttributes bool `js:"useSemanticAttributes"` // Use OTel semantic conventions

	// Workflow-based generation
	UseWorkflows            bool              `js:"useWorkflows"`            // Enable workflow-based trace generation
	WorkflowWeights         map[string]float64 `js:"workflowWeights"`         // Distribution of workflows
	BusinessAttributesDensity float64          `js:"businessAttributesDensity"` // How many business attrs per span (0.0-1.0)

	// Cardinality and tags
	CardinalityConfig map[string]int `js:"cardinalityConfig"` // Override cardinality per attribute (optional)
	EnableTags        bool           `js:"enableTags"`        // Enable additional tag generation
	TagDensity        float64        `js:"tagDensity"`       // Probability of adding tags (0.0-1.0)

	// Tree-based generation
	UseTraceTree    bool             `js:"useTraceTree"`    // Enable tree-based trace generation
	TraceTreeConfig *TraceTreeConfig `js:"traceTree"`       // Tree configuration
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() Config {
	return Config{
		Services:           3,
		SpanDepth:          4,
		SpansPerTrace:      10,
		AttributeCount:     5,
		AttributeValueSize: 32,
		EventCount:         0,
		DurationBaseMs:     50,
		DurationVarianceMs: 30,
		ErrorRate:          0.02,
		MaxFanOut:          5,
		FanOutVariance:     0.5,
		UseSemanticAttributes: true,
		SpanKindWeights: map[string]float64{
			"server":   0.35,
			"client":   0.35,
			"internal": 0.20,
			"producer": 0.05,
			"consumer": 0.05,
		},
		ResourceAttributes:      make(map[string]string),
		UseWorkflows:            false,
		WorkflowWeights:         make(map[string]float64),
		BusinessAttributesDensity: 0.8,
		CardinalityConfig:       make(map[string]int),
		EnableTags:              false,
		TagDensity:              0.9,
	}
}

// BatchConfig represents configuration for generating batches
type BatchConfig struct {
	TargetSizeBytes int    `js:"targetSizeBytes"` // Target batch size in bytes
	TraceConfig     Config `js:"traceConfig"`     // Configuration for individual traces
}

// RateLimitConfig represents configuration for MB/s rate limiting
type RateLimitConfig struct {
	TargetMBps      float64 `js:"targetMBps"`      // Target throughput in MB/s
	BurstMultiplier float64 `js:"burstMultiplier"` // Burst allowance (default: 1.5)
}

// ThroughputConfig represents calculated throughput configuration
type ThroughputConfig struct {
	TargetBytesPerSec float64 `js:"targetBytesPerSec"` // Target throughput in bytes/s
	TracesPerVU       float64 `js:"tracesPerVU"`       // Computed traces per second per VU
	EstimatedSizeB    int     `js:"estimatedSizeB"`    // Estimated trace size in bytes
	TotalTracesPerSec float64 `js:"totalTracesPerSec"` // Total traces per second across all VUs
}

// CalculateThroughput calculates the number of traces per second per VU needed to achieve target bytes/s
// Returns a ThroughputConfig with the calculated values
func CalculateThroughput(config Config, targetBytesPerSec float64, numVUs int) ThroughputConfig {
	if targetBytesPerSec <= 0 {
		targetBytesPerSec = bytesPerMegabyte // Default to 1 MB/s
	}
	if numVUs <= 0 {
		numVUs = 1 // Default to 1 VU
	}
	
	// Estimate trace size
	estimatedSizeB := EstimateTraceSizeFromConfig(config)
	if estimatedSizeB == 0 {
		estimatedSizeB = defaultFallbackTraceSize // Fallback estimate
	}
	
	// Calculate total traces per second needed
	totalTracesPerSec := targetBytesPerSec / float64(estimatedSizeB)
	
	// Calculate traces per second per VU
	tracesPerVU := totalTracesPerSec / float64(numVUs)
	
	return ThroughputConfig{
		TargetBytesPerSec: targetBytesPerSec,
		TracesPerVU:       tracesPerVU,
		EstimatedSizeB:    estimatedSizeB,
		TotalTracesPerSec: totalTracesPerSec,
	}
}
