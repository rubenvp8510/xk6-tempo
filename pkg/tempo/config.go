package tempo

import "time"

// Config represents the configuration for the Tempo client
type Config struct {
	Endpoint string `js:"endpoint"`
	Protocol string `js:"protocol"` // "otlp-http" or "otlp-grpc"
	Tenant   string `js:"tenant"`
	Timeout  int    `js:"timeout"` // seconds, default 30

	// Test context for metric tagging
	TestName   string  `js:"testName"`   // Test name for metric tags
	TargetQPS  int     `js:"targetQPS"`  // Target QPS for metric tags
	TargetMBps float64 `js:"targetMBps"` // Target MB/s for metric tags
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() Config {
	return Config{
		Endpoint: "http://localhost:4318",
		Protocol: "otlp-http",
		Timeout:  30,
	}
}

// QueryWorkloadConfig represents configuration for query workload testing
type QueryWorkloadConfig struct {
	// Rate limiting
	TargetQPS      float64 `js:"targetQPS"`      // Target queries per second
	BurstMultiplier float64 `js:"burstMultiplier"` // Burst multiplier (default: 2.0)
	QPSMultiplier   float64 `js:"qpsMultiplier"`  // QPS multiplier for compensation (default: 1.0)
	
	// Backoff configuration
	EnableBackoff   bool    `js:"enableBackoff"`   // Enable adaptive backoff (default: true)
	MinBackoffMs    int     `js:"minBackoffMs"`    // Minimum backoff in ms (default: 200)
	MaxBackoffMs    int     `js:"maxBackoffMs"`    // Maximum backoff in ms (default: 30000)
	BackoffJitter   bool    `js:"backoffJitter"`   // Add jitter to backoff (default: true)
	
	// Time buckets for query distribution
	TimeBuckets []TimeBucketConfig `js:"timeBuckets"`
	
	// Execution plan
	ExecutionPlan []PlanEntry `js:"executionPlan"`
	
	// Search and fetch workflow
	TraceFetchProbability float64 `js:"traceFetchProbability"` // Probability of fetching trace after search (0.0-1.0, default: 0.1)
	
	// Time window jitter
	TimeWindowJitterMs int `js:"timeWindowJitterMs"` // Jitter to add to time windows in ms (default: 0)
}

// TimeBucketConfig represents a time bucket for query distribution
type TimeBucketConfig struct {
	Name     string `js:"name"`     // Bucket name/identifier
	AgeStart string `js:"ageStart"` // Start age (e.g., "1h", "30m")
	AgeEnd   string `js:"ageEnd"`   // End age (e.g., "2h", "1h")
	Weight   float64 `js:"weight"`  // Weight for selection (default: 1.0)
}

// PlanEntry represents an entry in the execution plan
type PlanEntry struct {
	QueryName string  `js:"queryName"` // Name of the query to execute
	BucketName string `js:"bucketName"` // Name of the time bucket to use
	Weight    float64 `js:"weight"`    // Weight for selection (default: 1.0)
}

// QueryDefinition represents a query definition
type QueryDefinition struct {
	Name      string            `js:"name"`      // Query name/identifier
	Query     string            `js:"query"`    // TraceQL query string
	Limit     int               `js:"limit"`    // Result limit (default: 20)
	Options   map[string]interface{} `js:"options"` // Additional options
}

// DefaultQueryWorkloadConfig returns a config with sensible defaults
func DefaultQueryWorkloadConfig() QueryWorkloadConfig {
	return QueryWorkloadConfig{
		TargetQPS:           10.0,
		BurstMultiplier:     2.0,
		QPSMultiplier:       1.0,
		EnableBackoff:       true,
		MinBackoffMs:        200,
		MaxBackoffMs:        30000,
		BackoffJitter:       true,
		TraceFetchProbability: 0.1,
		TimeWindowJitterMs:  0,
		TimeBuckets: []TimeBucketConfig{
			{
				Name:     "recent",
				AgeStart: "0m",
				AgeEnd:   "1h",
				Weight:   1.0,
			},
		},
		ExecutionPlan: []PlanEntry{
			{
				QueryName: "default",
				BucketName: "recent",
				Weight: 1.0,
			},
		},
	}
}

// ParseTimeBucket parses age strings and returns time ranges
func (tb *TimeBucketConfig) ParseTimeRanges(elapsed time.Duration) (start time.Time, end time.Time, eligible bool, err error) {
	ageStart, err := time.ParseDuration(tb.AgeStart)
	if err != nil {
		return time.Time{}, time.Time{}, false, err
	}
	
	ageEnd, err := time.ParseDuration(tb.AgeEnd)
	if err != nil {
		return time.Time{}, time.Time{}, false, err
	}
	
	// Bucket is only eligible if enough time has elapsed
	if elapsed < ageEnd {
		return time.Time{}, time.Time{}, false, nil
	}
	
	now := time.Now()
	end = now.Add(-ageStart)
	start = now.Add(-ageEnd)
	
	return start, end, true, nil
}

