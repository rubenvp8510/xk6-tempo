package tempo

import (
	"fmt"

	"github.com/rvargasp/xk6-tempo/pkg/generator"
	"go.k6.io/k6/js/modules"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func init() {
	modules.Register("k6/x/tempo", New)
}

// RootModule is the global module instance
type RootModule struct{}

// ModuleInstance represents an instance of the module
type ModuleInstance struct {
	vu modules.VU
}

// New returns a new root module
func New() modules.Module {
	return &RootModule{}
}

// NewModuleInstance implements the modules.Module interface
func (r *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &ModuleInstance{vu: vu}
}

// Exports returns the exports of the module
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]interface{}{
			"Client":          mi.newClient,
			"generateTrace":   mi.generateTrace,
			"generateBatch":   mi.generateBatch,
			"createRateLimiter": mi.createRateLimiter,
		},
	}
}

// newClient creates a new Tempo client
func (mi *ModuleInstance) newClient(config map[string]interface{}) (*Client, error) {
	state := mi.vu.State()
	if state == nil {
		return nil, fmt.Errorf("k6 state is nil")
	}

	// Convert map to Config struct
	cfg := DefaultConfig()
	if endpoint, ok := config["endpoint"].(string); ok && endpoint != "" {
		cfg.Endpoint = endpoint
	}
	if protocol, ok := config["protocol"].(string); ok && protocol != "" {
		cfg.Protocol = protocol
	}
	if tenant, ok := config["tenant"].(string); ok {
		cfg.Tenant = tenant
	}
	if timeout, ok := config["timeout"].(int); ok && timeout > 0 {
		cfg.Timeout = timeout
	}
	if bearerToken, ok := config["bearerToken"].(string); ok {
		cfg.BearerToken = bearerToken
	}
	if bearerTokenFile, ok := config["bearerTokenFile"].(string); ok {
		cfg.BearerTokenFile = bearerTokenFile
	}
	if testName, ok := config["testName"].(string); ok {
		cfg.TestName = testName
	}
	if targetQPS, ok := config["targetQPS"].(int); ok && targetQPS > 0 {
		cfg.TargetQPS = targetQPS
	}
	if targetMBps, ok := config["targetMBps"].(float64); ok && targetMBps > 0 {
		cfg.TargetMBps = targetMBps
	}

	return NewClient(state, cfg)
}

// generateTrace generates a single trace
func (mi *ModuleInstance) generateTrace(config map[string]interface{}) (ptrace.Traces, error) {
	cfg := generator.DefaultConfig()
	
	if services, ok := config["services"].(int); ok && services > 0 {
		cfg.Services = services
	}
	if spanDepth, ok := config["spanDepth"].(int); ok && spanDepth > 0 {
		cfg.SpanDepth = spanDepth
	}
	if spansPerTrace, ok := config["spansPerTrace"].(int); ok && spansPerTrace > 0 {
		cfg.SpansPerTrace = spansPerTrace
	}
	if attributeCount, ok := config["attributeCount"].(int); ok && attributeCount > 0 {
		cfg.AttributeCount = attributeCount
	}
	if attributeValueSize, ok := config["attributeValueSize"].(int); ok && attributeValueSize > 0 {
		cfg.AttributeValueSize = attributeValueSize
	}
	if eventCount, ok := config["eventCount"].(int); ok {
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
	if durationBaseMs, ok := config["durationBaseMs"].(int); ok && durationBaseMs > 0 {
		cfg.DurationBaseMs = durationBaseMs
	}
	if durationVarianceMs, ok := config["durationVarianceMs"].(int); ok && durationVarianceMs >= 0 {
		cfg.DurationVarianceMs = durationVarianceMs
	}
	if errorRate, ok := config["errorRate"].(float64); ok && errorRate >= 0 && errorRate <= 1 {
		cfg.ErrorRate = errorRate
	}
	if maxFanOut, ok := config["maxFanOut"].(int); ok && maxFanOut > 0 {
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

	return generator.GenerateTrace(cfg), nil
}

// generateBatch generates a batch of traces
func (mi *ModuleInstance) generateBatch(config map[string]interface{}) ([]ptrace.Traces, error) {
	batchConfig := generator.BatchConfig{}
	
	if targetSize, ok := config["targetSizeBytes"].(int); ok && targetSize > 0 {
		batchConfig.TargetSizeBytes = targetSize
	} else {
		return nil, fmt.Errorf("targetSizeBytes is required")
	}
	
	// Parse traceConfig
	traceConfig := generator.DefaultConfig()
	if traceCfgMap, ok := config["traceConfig"].(map[string]interface{}); ok {
		if services, ok := traceCfgMap["services"].(int); ok && services > 0 {
			traceConfig.Services = services
		}
		if spanDepth, ok := traceCfgMap["spanDepth"].(int); ok && spanDepth > 0 {
			traceConfig.SpanDepth = spanDepth
		}
		if spansPerTrace, ok := traceCfgMap["spansPerTrace"].(int); ok && spansPerTrace > 0 {
			traceConfig.SpansPerTrace = spansPerTrace
		}
		if attributeCount, ok := traceCfgMap["attributeCount"].(int); ok && attributeCount > 0 {
			traceConfig.AttributeCount = attributeCount
		}
		if attributeValueSize, ok := traceCfgMap["attributeValueSize"].(int); ok && attributeValueSize > 0 {
			traceConfig.AttributeValueSize = attributeValueSize
		}
		if eventCount, ok := traceCfgMap["eventCount"].(int); ok {
			traceConfig.EventCount = eventCount
		}
		if resourceAttrs, ok := traceCfgMap["resourceAttributes"].(map[string]interface{}); ok {
			traceConfig.ResourceAttributes = make(map[string]string)
			for k, v := range resourceAttrs {
				if str, ok := v.(string); ok {
					traceConfig.ResourceAttributes[k] = str
				}
			}
		}
		if durationBaseMs, ok := traceCfgMap["durationBaseMs"].(int); ok && durationBaseMs > 0 {
			traceConfig.DurationBaseMs = durationBaseMs
		}
		if durationVarianceMs, ok := traceCfgMap["durationVarianceMs"].(int); ok && durationVarianceMs >= 0 {
			traceConfig.DurationVarianceMs = durationVarianceMs
		}
		if errorRate, ok := traceCfgMap["errorRate"].(float64); ok && errorRate >= 0 && errorRate <= 1 {
			traceConfig.ErrorRate = errorRate
		}
		if maxFanOut, ok := traceCfgMap["maxFanOut"].(int); ok && maxFanOut > 0 {
			traceConfig.MaxFanOut = maxFanOut
		}
		if fanOutVariance, ok := traceCfgMap["fanOutVariance"].(float64); ok && fanOutVariance >= 0 && fanOutVariance <= 1 {
			traceConfig.FanOutVariance = fanOutVariance
		}
		if useSemantic, ok := traceCfgMap["useSemanticAttributes"].(bool); ok {
			traceConfig.UseSemanticAttributes = useSemantic
		}
		if spanKindWeights, ok := traceCfgMap["spanKindWeights"].(map[string]interface{}); ok {
			traceConfig.SpanKindWeights = make(map[string]float64)
			for k, v := range spanKindWeights {
				if weight, ok := v.(float64); ok {
					traceConfig.SpanKindWeights[k] = weight
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
