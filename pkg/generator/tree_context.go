package generator

import (
	"math/rand"

	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
)

// TreeTraceContext mantiene IDs consistentes durante la generación de una traza
type TreeTraceContext struct {
	UserID         string
	OrderID        string
	CorrelationID  string
	SessionID      string
	TenantID       string
	Region         string
	Datacenter     string
	AvailabilityZone string
	Cluster        string
	OrgID          string
	CustomerID     string
	Version        string
	GitCommit      string
	Canary         string
	UserTier       string
	Priority       string
	RequestID      string
	PaymentID      string
	ShipmentID     string
	ProductID      string
}

// NewTreeTraceContext crea un nuevo contexto de traza desde configuración
func NewTreeTraceContext(config TreeContext, rng *rand.Rand) *TreeTraceContext {
	cm := GetCardinalityManager()
	ctx := &TreeTraceContext{}

	// Generar valores basados en lo que se debe propagar
	for _, propKey := range config.Propagate {
		switch propKey {
		case "user_id":
			ctx.UserID = cm.GetValue("customer_id", rng)
		case "order_id":
			ctx.OrderID = cm.GetValue("order_id", rng)
		case "correlation_id":
			ctx.CorrelationID = cm.GetValue("correlation_id", rng)
		case "session_id":
			ctx.SessionID = cm.GetValue("session_id", rng)
		case "tenant_id":
			ctx.TenantID = cm.GetValue("tenant_id", rng)
		case "region":
			ctx.Region = cm.GetValue("region", rng)
		case "datacenter":
			ctx.Datacenter = cm.GetValue("datacenter", rng)
		case "availability_zone":
			ctx.AvailabilityZone = cm.GetValue("availability_zone", rng)
		case "cluster":
			ctx.Cluster = cm.GetValue("cluster", rng)
		case "org_id":
			ctx.OrgID = cm.GetValue("org_id", rng)
		case "customer_id":
			ctx.CustomerID = cm.GetValue("customer_id", rng)
		case "version":
			ctx.Version = cm.GetValue("version", rng)
		case "git_commit":
			ctx.GitCommit = cm.GetValue("git_commit", rng)
		case "canary":
			ctx.Canary = cm.GetValue("canary", rng)
		case "user_tier":
			ctx.UserTier = cm.GetValue("user_tier", rng)
		case "priority":
			ctx.Priority = cm.GetValue("priority", rng)
		case "request_id":
			ctx.RequestID = cm.GetValue("request_id", rng)
		case "payment_id":
			ctx.PaymentID = cm.GetValue("payment_id", rng)
		case "shipment_id":
			ctx.ShipmentID = cm.GetValue("shipment_id", rng)
		case "product_id":
			ctx.ProductID = cm.GetValue("product_id", rng)
		}
	}

	return ctx
}

// GetPropagatedTags retorna los tags propagados como atributos
func (ctx *TreeTraceContext) GetPropagatedTags(tagDensity float64, rng *rand.Rand) []*commonv1.KeyValue {
	tags := make([]*commonv1.KeyValue, 0)

	if tagDensity <= 0 {
		tagDensity = 0.9
	}
	if tagDensity > 1 {
		tagDensity = 1
	}

	// Infrastructure tags
	if ctx.Region != "" && rng.Float64() < tagDensity {
		tags = append(tags, &commonv1.KeyValue{
			Key: "infrastructure.region",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.Region,
				},
			},
		})
	}

	if ctx.Datacenter != "" && rng.Float64() < tagDensity {
		tags = append(tags, &commonv1.KeyValue{
			Key: "infrastructure.datacenter",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.Datacenter,
				},
			},
		})
	}

	if ctx.AvailabilityZone != "" && rng.Float64() < tagDensity {
		tags = append(tags, &commonv1.KeyValue{
			Key: "infrastructure.availability_zone",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.AvailabilityZone,
				},
			},
		})
	}

	if ctx.Cluster != "" && rng.Float64() < tagDensity {
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
	if ctx.TenantID != "" && rng.Float64() < tagDensity {
		tags = append(tags, &commonv1.KeyValue{
			Key: "tenant.id",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.TenantID,
				},
			},
		})
	}

	if ctx.CustomerID != "" && rng.Float64() < tagDensity*0.7 {
		tags = append(tags, &commonv1.KeyValue{
			Key: "tenant.customer_id",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.CustomerID,
				},
			},
		})
	}

	if ctx.OrgID != "" && rng.Float64() < tagDensity {
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
	if ctx.Version != "" && rng.Float64() < tagDensity {
		tags = append(tags, &commonv1.KeyValue{
			Key: "deployment.version",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.Version,
				},
			},
		})
	}

	if ctx.GitCommit != "" && rng.Float64() < tagDensity*0.8 {
		tags = append(tags, &commonv1.KeyValue{
			Key: "deployment.git_commit",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.GitCommit,
				},
			},
		})
	}

	if ctx.Canary != "" && rng.Float64() < tagDensity*0.3 {
		tags = append(tags, &commonv1.KeyValue{
			Key: "deployment.canary",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.Canary,
				},
			},
		})
	}

	// Request context tags
	if ctx.RequestID != "" && rng.Float64() < tagDensity {
		tags = append(tags, &commonv1.KeyValue{
			Key: "request.id",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.RequestID,
				},
			},
		})
	}

	if ctx.CorrelationID != "" && rng.Float64() < tagDensity*0.8 {
		tags = append(tags, &commonv1.KeyValue{
			Key: "request.correlation_id",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.CorrelationID,
				},
			},
		})
	}

	if ctx.UserTier != "" && rng.Float64() < tagDensity {
		tags = append(tags, &commonv1.KeyValue{
			Key: "request.user_tier",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.UserTier,
				},
			},
		})
	}

	if ctx.Priority != "" && rng.Float64() < tagDensity*0.5 {
		tags = append(tags, &commonv1.KeyValue{
			Key: "request.priority",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.Priority,
				},
			},
		})
	}

	// Business context tags
	if ctx.UserID != "" && rng.Float64() < tagDensity {
		tags = append(tags, &commonv1.KeyValue{
			Key: "user.id",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.UserID,
				},
			},
		})
	}

	if ctx.OrderID != "" && rng.Float64() < tagDensity {
		tags = append(tags, &commonv1.KeyValue{
			Key: "order.id",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.OrderID,
				},
			},
		})
	}

	if ctx.SessionID != "" && rng.Float64() < tagDensity*0.8 {
		tags = append(tags, &commonv1.KeyValue{
			Key: "session.id",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.SessionID,
				},
			},
		})
	}

	if ctx.PaymentID != "" && rng.Float64() < tagDensity {
		tags = append(tags, &commonv1.KeyValue{
			Key: "payment.id",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.PaymentID,
				},
			},
		})
	}

	if ctx.ShipmentID != "" && rng.Float64() < tagDensity {
		tags = append(tags, &commonv1.KeyValue{
			Key: "shipment.id",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.ShipmentID,
				},
			},
		})
	}

	if ctx.ProductID != "" && rng.Float64() < tagDensity {
		tags = append(tags, &commonv1.KeyValue{
			Key: "product.id",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.ProductID,
				},
			},
		})
	}

	return tags
}

