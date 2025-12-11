package generator

import (
	"fmt"
)

const (
	bytesPerMegabyte         = 1024 * 1024
	defaultFallbackTraceSize = 1000
)

// Config represents the configuration for trace generation.
// All fields are optional and will use defaults from DefaultConfig() if not specified.
type Config struct {
	// Basic span configuration
	Services           int               `js:"services"`           // Number of distinct services (default: 3, must be > 0)
	SpanDepth          int               `js:"spanDepth"`          // Max span tree depth (default: 4, must be > 0)
	SpansPerTrace      int               `js:"spansPerTrace"`      // Total spans per trace (default: 10, must be > 0)
	AttributeCount     int               `js:"attributeCount"`     // Number of attributes per span (default: 5, must be >= 0)
	AttributeValueSize int               `js:"attributeValueSize"` // Size of attribute values in bytes (default: 32, must be >= 0)
	EventCount         int               `js:"eventCount"`         // Number of events/logs per span (default: 0, must be >= 0)
	ResourceAttributes map[string]string `js:"resourceAttributes"` // Resource-level attributes (default: empty map, auto-generated if empty)

	// Duration/timing configuration
	DurationBaseMs     int `js:"durationBaseMs"`     // Base duration in milliseconds (default: 50, must be > 0)
	DurationVarianceMs int `js:"durationVarianceMs"` // Standard deviation for duration in milliseconds (default: 30, must be >= 0)

	// Error injection
	ErrorRate float64 `js:"errorRate"` // Probability of error status (default: 0.02, range: 0.0-1.0)

	// Span kind distribution (weights are normalized internally if they don't sum to 1.0)
	SpanKindWeights map[string]float64 `js:"spanKindWeights"` // Distribution weights, e.g., {"server": 0.35, "client": 0.35, "internal": 0.20, "producer": 0.05, "consumer": 0.05}

	// Trace shape variance
	MaxFanOut      int     `js:"maxFanOut"`      // Max children per span (default: 5, must be > 0)
	FanOutVariance float64 `js:"fanOutVariance"` // Variance in fan-out (default: 0.5, range: 0.0-1.0)

	// Semantic attributes
	UseSemanticAttributes bool `js:"useSemanticAttributes"` // Use OpenTelemetry semantic conventions (default: true)

	// Workflow-based generation (mutually exclusive with tree-based generation)
	UseWorkflows              bool               `js:"useWorkflows"`              // Enable workflow-based trace generation (default: false)
	WorkflowWeights           map[string]float64 `js:"workflowWeights"`           // Distribution of workflows (default: empty map)
	BusinessAttributesDensity float64            `js:"businessAttributesDensity"` // How many business attrs per span (default: 0.8, range: 0.0-1.0)

	// Cardinality and tags
	CardinalityConfig map[string]int `js:"cardinalityConfig"` // Override cardinality per attribute (default: empty map, optional)
	EnableTags        bool           `js:"enableTags"`        // Enable additional tag generation (default: false)
	TagDensity        float64        `js:"tagDensity"`        // Probability of adding tags (default: 0.9, range: 0.0-1.0)

	// Tree-based generation (mutually exclusive with workflow-based generation)
	UseTraceTree    bool             `js:"useTraceTree"` // Enable tree-based trace generation (default: false)
	TraceTreeConfig *TraceTreeConfig `js:"traceTree"`    // Tree configuration (default: nil, required if UseTraceTree is true)
}

// DefaultConfig returns a config with sensible defaults.
// All fields are initialized to their default values.
func DefaultConfig() Config {
	return Config{
		// Basic span configuration
		Services:           3,
		SpanDepth:          4,
		SpansPerTrace:      10,
		AttributeCount:     5,
		AttributeValueSize: 32,
		EventCount:         0,
		ResourceAttributes: make(map[string]string),

		// Duration/timing configuration
		DurationBaseMs:     50,
		DurationVarianceMs: 30,

		// Error injection
		ErrorRate: 0.02,

		// Span kind distribution
		SpanKindWeights: map[string]float64{
			"server":   0.35,
			"client":   0.35,
			"internal": 0.20,
			"producer": 0.05,
			"consumer": 0.05,
		},

		// Trace shape variance
		MaxFanOut:      5,
		FanOutVariance: 0.5,

		// Semantic attributes
		UseSemanticAttributes: true,

		// Workflow-based generation
		UseWorkflows:              false,
		WorkflowWeights:           make(map[string]float64),
		BusinessAttributesDensity: 0.8,

		// Cardinality and tags
		CardinalityConfig: make(map[string]int),
		EnableTags:        false,
		TagDensity:        0.9,

		// Tree-based generation
		UseTraceTree:    false,
		TraceTreeConfig: nil,
	}
}

// Validate validates the configuration and returns an error if any field has an invalid value.
// It checks:
//   - Required numeric fields are > 0 where applicable
//   - Probability fields are in range [0.0, 1.0]
//   - Mutually exclusive options (UseWorkflows and UseTraceTree)
//   - Required fields when features are enabled
func (c *Config) Validate() error {
	// Basic span configuration validation
	if c.Services <= 0 {
		return fmt.Errorf("services must be > 0, got %d", c.Services)
	}
	if c.SpanDepth <= 0 {
		return fmt.Errorf("spanDepth must be > 0, got %d", c.SpanDepth)
	}
	if c.SpansPerTrace <= 0 {
		return fmt.Errorf("spansPerTrace must be > 0, got %d", c.SpansPerTrace)
	}
	if c.AttributeCount < 0 {
		return fmt.Errorf("attributeCount must be >= 0, got %d", c.AttributeCount)
	}
	if c.AttributeValueSize < 0 {
		return fmt.Errorf("attributeValueSize must be >= 0, got %d", c.AttributeValueSize)
	}
	if c.EventCount < 0 {
		return fmt.Errorf("eventCount must be >= 0, got %d", c.EventCount)
	}

	// Duration/timing validation
	if c.DurationBaseMs <= 0 {
		return fmt.Errorf("durationBaseMs must be > 0, got %d", c.DurationBaseMs)
	}
	if c.DurationVarianceMs < 0 {
		return fmt.Errorf("durationVarianceMs must be >= 0, got %d", c.DurationVarianceMs)
	}

	// Error rate validation
	if c.ErrorRate < 0.0 || c.ErrorRate > 1.0 {
		return fmt.Errorf("errorRate must be in range [0.0, 1.0], got %f", c.ErrorRate)
	}

	// Trace shape variance validation
	if c.MaxFanOut <= 0 {
		return fmt.Errorf("maxFanOut must be > 0, got %d", c.MaxFanOut)
	}
	if c.FanOutVariance < 0.0 || c.FanOutVariance > 1.0 {
		return fmt.Errorf("fanOutVariance must be in range [0.0, 1.0], got %f", c.FanOutVariance)
	}

	// Workflow configuration validation
	if c.BusinessAttributesDensity < 0.0 || c.BusinessAttributesDensity > 1.0 {
		return fmt.Errorf("businessAttributesDensity must be in range [0.0, 1.0], got %f", c.BusinessAttributesDensity)
	}

	// Tag density validation
	if c.TagDensity < 0.0 || c.TagDensity > 1.0 {
		return fmt.Errorf("tagDensity must be in range [0.0, 1.0], got %f", c.TagDensity)
	}

	// Mutually exclusive options
	if c.UseWorkflows && c.UseTraceTree {
		return fmt.Errorf("useWorkflows and useTraceTree are mutually exclusive - only one can be enabled")
	}

	// Tree-based generation validation
	if c.UseTraceTree && c.TraceTreeConfig == nil {
		return fmt.Errorf("traceTreeConfig is required when useTraceTree is true")
	}

	return nil
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
