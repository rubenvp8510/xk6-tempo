package generator

import (
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
)

// Density multipliers used across the codebase for probabilistic attribute generation
const (
	DensityFull       = 1.0  // 100% probability
	DensityVeryHigh   = 0.9  // 90% probability
	DensityHigh       = 0.8  // 80% probability
	DensityMediumHigh = 0.7  // 70% probability
	DensityMedium     = 0.6  // 60% probability
	DensityMediumLow  = 0.5  // 50% probability
	DensityLow        = 0.4  // 40% probability
	DensityVeryLow    = 0.3  // 30% probability
)

// newStringKeyValue creates a KeyValue with a string value
func newStringKeyValue(key, value string) *commonv1.KeyValue {
	return &commonv1.KeyValue{
		Key: key,
		Value: &commonv1.AnyValue{
			Value: &commonv1.AnyValue_StringValue{
				StringValue: value,
			},
		},
	}
}

// newIntKeyValue creates a KeyValue with an integer value
func newIntKeyValue(key string, value int64) *commonv1.KeyValue {
	return &commonv1.KeyValue{
		Key: key,
		Value: &commonv1.AnyValue{
			Value: &commonv1.AnyValue_IntValue{
				IntValue: value,
			},
		},
	}
}

// newBoolKeyValue creates a KeyValue with a boolean value
func newBoolKeyValue(key string, value bool) *commonv1.KeyValue {
	return &commonv1.KeyValue{
		Key: key,
		Value: &commonv1.AnyValue{
			Value: &commonv1.AnyValue_BoolValue{
				BoolValue: value,
			},
		},
	}
}

// newDoubleKeyValue creates a KeyValue with a double value
func newDoubleKeyValue(key string, value float64) *commonv1.KeyValue {
	return &commonv1.KeyValue{
		Key: key,
		Value: &commonv1.AnyValue{
			Value: &commonv1.AnyValue_DoubleValue{
				DoubleValue: value,
			},
		},
	}
}

