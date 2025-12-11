package generator

import (
	"math/rand"

	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
)

// TagContext holds tag values that should be consistent within a trace
type TagContext struct {
	Region           string
	Datacenter       string
	AvailabilityZone string
	Cluster          string
	TenantID         string
	CustomerID       string
	OrgID            string
	Version          string
	GitCommit        string
	Canary           string
	FeatureFlags     []string
	RequestID        string
	CorrelationID    string
	UserTier         string
	Priority         string
}

// GenerateTagContext creates a new tag context for a trace
func GenerateTagContext(config Config, rng *rand.Rand) *TagContext {
	cm := GetCardinalityManager()

	ctx := &TagContext{
		Region:           cm.GetValue("region", rng, config.CardinalityConfig),
		Datacenter:       cm.GetValue("datacenter", rng, config.CardinalityConfig),
		AvailabilityZone: cm.GetValue("availability_zone", rng, config.CardinalityConfig),
		Cluster:          cm.GetValue("cluster", rng, config.CardinalityConfig),
		TenantID:         cm.GetValue("tenant_id", rng, config.CardinalityConfig),
		CustomerID:       cm.GetValue("customer_id", rng, config.CardinalityConfig),
		OrgID:            cm.GetValue("org_id", rng, config.CardinalityConfig),
		Version:          cm.GetValue("version", rng, config.CardinalityConfig),
		GitCommit:        cm.GetValue("git_commit", rng, config.CardinalityConfig),
		Canary:           cm.GetValue("canary", rng, config.CardinalityConfig),
		UserTier:         cm.GetValue("user_tier", rng, config.CardinalityConfig),
		Priority:         cm.GetValue("priority", rng, config.CardinalityConfig),
		RequestID:        cm.GetValue("request_id", rng, config.CardinalityConfig),
		CorrelationID:    cm.GetValue("correlation_id", rng, config.CardinalityConfig),
	}

	// Generate feature flags (multiple possible)
	numFlags := rng.Intn(3) + 1 // 1-3 flags
	ctx.FeatureFlags = make([]string, 0, numFlags)
	for i := 0; i < numFlags; i++ {
		ctx.FeatureFlags = append(ctx.FeatureFlags, cm.GetValue("feature_flags", rng, config.CardinalityConfig))
	}

	return ctx
}

// GenerateTags generates tag attributes based on context and density
func GenerateTags(ctx *TagContext, config Config, rng *rand.Rand) []*commonv1.KeyValue {
	if !config.EnableTags {
		return []*commonv1.KeyValue{}
	}

	tags := make([]*commonv1.KeyValue, 0)
	tagDensity := config.TagDensity
	if tagDensity <= 0 {
		tagDensity = 0.9 // Default 90%
	}
	if tagDensity > 1 {
		tagDensity = 1
	}

	// Infrastructure tags (always included if tags enabled, consistent per trace)
	if rng.Float64() < tagDensity {
		tags = append(tags, newStringKeyValue("infrastructure.region", ctx.Region))
	}

	if rng.Float64() < tagDensity {
		tags = append(tags, newStringKeyValue("infrastructure.datacenter", ctx.Datacenter))
	}

	if rng.Float64() < tagDensity {
		tags = append(tags, newStringKeyValue("infrastructure.availability_zone", ctx.AvailabilityZone))
	}

	if rng.Float64() < tagDensity {
		tags = append(tags, newStringKeyValue("infrastructure.cluster", ctx.Cluster))
	}

	// Tenant tags
	if rng.Float64() < tagDensity {
		tags = append(tags, newStringKeyValue("tenant.id", ctx.TenantID))
	}

	if rng.Float64() < tagDensity*DensityMediumHigh { // 70% of tag density for customer_id
		tags = append(tags, newStringKeyValue("tenant.customer_id", ctx.CustomerID))
	}

	if rng.Float64() < tagDensity {
		tags = append(tags, newStringKeyValue("tenant.org_id", ctx.OrgID))
	}

	// Deployment tags
	if rng.Float64() < tagDensity {
		tags = append(tags, newStringKeyValue("deployment.version", ctx.Version))
	}

	if rng.Float64() < tagDensity*DensityHigh { // 80% of tag density for git commit
		tags = append(tags, newStringKeyValue("deployment.git_commit", ctx.GitCommit))
	}

	if rng.Float64() < tagDensity*DensityVeryLow { // 30% chance for canary
		tags = append(tags, newStringKeyValue("deployment.canary", ctx.Canary))
	}

	// Feature flags
	if len(ctx.FeatureFlags) > 0 && rng.Float64() < tagDensity*DensityMedium {
		for _, flag := range ctx.FeatureFlags {
			tags = append(tags, newStringKeyValue("deployment.feature_flag", flag))
		}
	}

	// Request context tags (unique per trace but consistent across spans)
	if rng.Float64() < tagDensity {
		tags = append(tags, newStringKeyValue("request.id", ctx.RequestID))
	}

	if rng.Float64() < tagDensity*DensityHigh {
		tags = append(tags, newStringKeyValue("request.correlation_id", ctx.CorrelationID))
	}

	if rng.Float64() < tagDensity {
		tags = append(tags, newStringKeyValue("request.user_tier", ctx.UserTier))
	}

	if rng.Float64() < tagDensity*DensityMediumLow {
		tags = append(tags, newStringKeyValue("request.priority", ctx.Priority))
	}

	return tags
}
