package generator

import (
	"fmt"
	"math/rand"
	"sync"
)

// CardinalityTier represents the cardinality level for an attribute
type CardinalityTier int

const (
	CardinalityLow CardinalityTier = iota
	CardinalityMedium
	CardinalityHigh
	CardinalityVeryHigh
)

// CardinalityManager manages value pools for different cardinality tiers
type CardinalityManager struct {
	mu          sync.RWMutex
	valuePools  map[string][]string
	cardinality map[string]int // Current cardinality per attribute
}

var globalCardinalityManager *CardinalityManager
var cardinalityOnce sync.Once

// GetCardinalityManager returns the global cardinality manager
func GetCardinalityManager() *CardinalityManager {
	cardinalityOnce.Do(func() {
		globalCardinalityManager = &CardinalityManager{
			valuePools:  make(map[string][]string),
			cardinality: make(map[string]int),
		}
	})
	return globalCardinalityManager
}

// DefaultCardinality returns the default cardinality for an attribute
func DefaultCardinality(attrName string) int {
	// Define default cardinality tiers
	defaults := map[string]int{
		// Low cardinality (5-10 values)
		"region":                 8,
		"datacenter":             6,
		"environment":            3,
		"http.method":            5,
		"deployment.environment": 3,
		"canary":                 2,
		"user_tier":              4,
		"priority":               3,
		"version":                4,

		// Medium cardinality (50-100 values)
		"http.status_code":  10,
		"error_type":        15,
		"availability_zone": 50,
		"cluster":           75,
		"tenant_id":         50,
		"org_id":            50,
		"git_commit":        100,
		"feature_flags":     20,

		// High cardinality (1000-10000 values)
		"customer_id":  5000,
		"pod_name":     2000,
		"k8s.pod.name": 2000,
		"host.name":    1000,

		// Very high (unique per trace/span) - return 0 to indicate unique
		"trace_id":       0,
		"span_id":        0,
		"order_id":       0,
		"request_id":     0,
		"correlation_id": 0,
		"payment_id":     0,
		"shipment_id":    0,
		"session_id":     0,
	}

	if val, ok := defaults[attrName]; ok {
		return val
	}

	// Default to medium cardinality for unknown attributes
	return 50
}

// GetValue returns a value for an attribute with appropriate cardinality
func (cm *CardinalityManager) GetValue(attrName string, rng *rand.Rand, cardConfig map[string]int) string {
	// Check user override first
	cardinality := 0
	if val, ok := cardConfig[attrName]; ok {
		cardinality = val
	} else {
		// Use default
		cardinality = DefaultCardinality(attrName)
	}

	// Very high cardinality means unique value
	if cardinality == 0 {
		return cm.generateUniqueValue(attrName, rng)
	}

	// Try with read lock first
	cm.mu.RLock()
	pool, exists := cm.valuePools[attrName]
	poolLen := len(pool)
	cm.mu.RUnlock()

	if exists && poolLen >= cardinality {
		return pool[rng.Intn(poolLen)]
	}

	// Need to generate/update pool, switch to write lock
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Double check
	pool, exists = cm.valuePools[attrName]
	if !exists || len(pool) < cardinality {
		// Generate pool
		pool = cm.generateValuePool(attrName, cardinality, rng)
		cm.valuePools[attrName] = pool
		cm.cardinality[attrName] = len(pool)
	}

	// Return random value from pool
	return pool[rng.Intn(len(pool))]
}

// generateValuePool creates a pool of values for an attribute
func (cm *CardinalityManager) generateValuePool(attrName string, size int, rng *rand.Rand) []string {
	pool := make([]string, 0, size)

	// Generate values based on attribute name patterns
	for i := 0; i < size; i++ {
		var value string
		switch attrName {
		case "region":
			regions := []string{"us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1", "sa-east-1", "eu-central-1", "ap-northeast-1", "us-central-1"}
			if i < len(regions) {
				value = regions[i]
			} else {
				value = fmt.Sprintf("region-%d", i)
			}
		case "datacenter":
			value = fmt.Sprintf("dc-%02d", i+1)
		case "environment", "deployment.environment":
			envs := []string{"production", "staging", "development"}
			if i < len(envs) {
				value = envs[i]
			} else {
				value = fmt.Sprintf("env-%d", i)
			}
		case "http.method":
			methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
			if i < len(methods) {
				value = methods[i]
			} else {
				value = fmt.Sprintf("METHOD-%d", i)
			}
		case "http.status_code":
			codes := []int{200, 201, 204, 301, 302, 400, 401, 403, 404, 500, 502, 503, 504}
			if i < len(codes) {
				value = fmt.Sprintf("%d", codes[i])
			} else {
				value = fmt.Sprintf("%d", 200+i)
			}
		case "availability_zone":
			value = fmt.Sprintf("az-%02d", i+1)
		case "cluster":
			value = fmt.Sprintf("cluster-%03d", i+1)
		case "tenant_id":
			value = fmt.Sprintf("tenant-%04d", i+1)
		case "org_id":
			value = fmt.Sprintf("org-%04d", i+1)
		case "customer_id":
			value = fmt.Sprintf("customer-%06d", i+1)
		case "pod_name", "k8s.pod.name":
			value = fmt.Sprintf("pod-%s-%05d", randomString(5, rng), i+1)
		case "host.name":
			value = fmt.Sprintf("host-%05d", i+1)
		case "version":
			versions := []string{"1.0.0", "1.1.0", "1.2.0", "2.0.0"}
			if i < len(versions) {
				value = versions[i]
			} else {
				value = fmt.Sprintf("1.%d.0", i)
			}
		case "git_commit":
			value = randomHexString(7, rng)
		case "canary":
			canaries := []string{"true", "false"}
			if i < len(canaries) {
				value = canaries[i]
			} else {
				value = "false"
			}
		case "user_tier":
			tiers := []string{"free", "basic", "premium", "enterprise"}
			if i < len(tiers) {
				value = tiers[i]
			} else {
				value = fmt.Sprintf("tier-%d", i)
			}
		case "priority":
			priorities := []string{"low", "medium", "high", "critical"}
			if i < len(priorities) {
				value = priorities[i]
			} else {
				value = fmt.Sprintf("priority-%d", i)
			}
		case "feature_flags":
			value = fmt.Sprintf("feature-%03d", i+1)
		case "error_type":
			errors := []string{"timeout", "connection_error", "validation_error", "auth_error", "not_found", "rate_limit", "server_error"}
			if i < len(errors) {
				value = errors[i]
			} else {
				value = fmt.Sprintf("error-%d", i)
			}
		default:
			// Generic fallback
			value = fmt.Sprintf("%s-%d", attrName, i+1)
		}
		pool = append(pool, value)
	}

	return pool
}

// generateUniqueValue generates a unique value for very high cardinality attributes
func (cm *CardinalityManager) generateUniqueValue(attrName string, rng *rand.Rand) string {
	switch attrName {
	case "trace_id", "span_id", "request_id", "correlation_id":
		return randomHexString(16, rng)
	case "order_id", "payment_id", "shipment_id":
		return fmt.Sprintf("%s-%s", attrName[:len(attrName)-3], randomHexString(12, rng))
	case "session_id":
		return randomHexString(24, rng)
	default:
		return randomHexString(16, rng)
	}
}

// randomString generates a random string of given length
func randomString(length int, rng *rand.Rand) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rng.Intn(len(charset))]
	}
	return string(b)
}

// randomHexString generates a random hex string of given length
func randomHexString(length int, rng *rand.Rand) string {
	const charset = "0123456789abcdef"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rng.Intn(len(charset))]
	}
	return string(b)
}

// GetCardinalityStats returns current cardinality statistics
func (cm *CardinalityManager) GetCardinalityStats() map[string]int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	stats := make(map[string]int)
	for attr, count := range cm.cardinality {
		stats[attr] = count
	}
	return stats
}

// ResetPools clears all value pools for reproducibility with seeds
func (cm *CardinalityManager) ResetPools() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.valuePools = make(map[string][]string)
	cm.cardinality = make(map[string]int)
}
