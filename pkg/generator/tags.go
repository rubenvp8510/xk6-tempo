package generator

import (
	"math/rand"

	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
)

// TagContext holds tag values that should be consistent within a trace
type TagContext struct {
	Region          string
	Datacenter      string
	AvailabilityZone string
	Cluster         string
	TenantID        string
	CustomerID      string
	OrgID           string
	Version         string
	GitCommit       string
	Canary          string
	FeatureFlags    []string
	RequestID       string
	CorrelationID   string
	UserTier        string
	Priority        string
}

// GenerateTagContext creates a new tag context for a trace
func GenerateTagContext(config Config, rng *rand.Rand) *TagContext {
	cm := GetCardinalityManager()
	
	ctx := &TagContext{
		Region:          cm.GetValue("region", rng),
		Datacenter:      cm.GetValue("datacenter", rng),
		AvailabilityZone: cm.GetValue("availability_zone", rng),
		Cluster:         cm.GetValue("cluster", rng),
		TenantID:        cm.GetValue("tenant_id", rng),
		CustomerID:      cm.GetValue("customer_id", rng),
		OrgID:           cm.GetValue("org_id", rng),
		Version:         cm.GetValue("version", rng),
		GitCommit:       cm.GetValue("git_commit", rng),
		Canary:          cm.GetValue("canary", rng),
		UserTier:        cm.GetValue("user_tier", rng),
		Priority:        cm.GetValue("priority", rng),
		RequestID:       cm.GetValue("request_id", rng),
		CorrelationID:   cm.GetValue("correlation_id", rng),
	}
	
	// Generate feature flags (multiple possible)
	numFlags := rng.Intn(3) + 1 // 1-3 flags
	ctx.FeatureFlags = make([]string, 0, numFlags)
	for i := 0; i < numFlags; i++ {
		ctx.FeatureFlags = append(ctx.FeatureFlags, cm.GetValue("feature_flags", rng))
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
		tags = append(tags, &commonv1.KeyValue{
			Key: "infrastructure.region",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.Region,
				},
			},
		})
	}
	
	if rng.Float64() < tagDensity {
		tags = append(tags, &commonv1.KeyValue{
			Key: "infrastructure.datacenter",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.Datacenter,
				},
			},
		})
	}
	
	if rng.Float64() < tagDensity {
		tags = append(tags, &commonv1.KeyValue{
			Key: "infrastructure.availability_zone",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.AvailabilityZone,
				},
			},
		})
	}
	
	if rng.Float64() < tagDensity {
		tags = append(tags, &commonv1.KeyValue{
			Key: "infrastructure.cluster",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.Cluster,
				},
			},
		})
	}
	
	// Tenant tags
	if rng.Float64() < tagDensity {
		tags = append(tags, &commonv1.KeyValue{
			Key: "tenant.id",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.TenantID,
				},
			},
		})
	}
	
	if rng.Float64() < tagDensity*0.7 { // 70% of tag density for customer_id
		tags = append(tags, &commonv1.KeyValue{
			Key: "tenant.customer_id",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.CustomerID,
				},
			},
		})
	}
	
	if rng.Float64() < tagDensity {
		tags = append(tags, &commonv1.KeyValue{
			Key: "tenant.org_id",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.OrgID,
				},
			},
		})
	}
	
	// Deployment tags
	if rng.Float64() < tagDensity {
		tags = append(tags, &commonv1.KeyValue{
			Key: "deployment.version",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.Version,
				},
			},
		})
	}
	
	if rng.Float64() < tagDensity*0.8 { // 80% of tag density for git commit
		tags = append(tags, &commonv1.KeyValue{
			Key: "deployment.git_commit",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.GitCommit,
				},
			},
		})
	}
	
	if rng.Float64() < tagDensity*0.3 { // 30% chance for canary
		tags = append(tags, &commonv1.KeyValue{
			Key: "deployment.canary",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.Canary,
				},
			},
		})
	}
	
	// Feature flags
	if len(ctx.FeatureFlags) > 0 && rng.Float64() < tagDensity*0.6 {
		for _, flag := range ctx.FeatureFlags {
			tags = append(tags, &commonv1.KeyValue{
				Key: "deployment.feature_flag",
				Value: &commonv1.AnyValue{
					Value: &commonv1.AnyValue_StringValue{
						StringValue: flag,
					},
				},
			})
		}
	}
	
	// Request context tags (unique per trace but consistent across spans)
	if rng.Float64() < tagDensity {
		tags = append(tags, &commonv1.KeyValue{
			Key: "request.id",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.RequestID,
				},
			},
		})
	}
	
	if rng.Float64() < tagDensity*0.8 {
		tags = append(tags, &commonv1.KeyValue{
			Key: "request.correlation_id",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.CorrelationID,
				},
			},
		})
	}
	
	if rng.Float64() < tagDensity {
		tags = append(tags, &commonv1.KeyValue{
			Key: "request.user_tier",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.UserTier,
				},
			},
		})
	}
	
	if rng.Float64() < tagDensity*0.5 {
		tags = append(tags, &commonv1.KeyValue{
			Key: "request.priority",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.Priority,
				},
			},
		})
	}
	
	return tags
}

