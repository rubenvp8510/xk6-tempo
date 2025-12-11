package generator

import (
	"math/rand"

	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
)

// TreeTraceContext maintains consistent IDs during trace generation
type TreeTraceContext struct {
	UserID           string
	OrderID          string
	CorrelationID    string
	SessionID        string
	TenantID         string
	Region           string
	Datacenter       string
	AvailabilityZone string
	Cluster          string
	OrgID            string
	CustomerID       string
	Version          string
	GitCommit        string
	Canary           string
	UserTier         string
	Priority         string
	RequestID        string
	PaymentID        string
	ShipmentID       string
	ProductID        string
}

// NewTreeTraceContext creates a new trace context from configuration
func NewTreeTraceContext(config TreeContext, rng *rand.Rand) *TreeTraceContext {
	cm := GetCardinalityManager()
	ctx := &TreeTraceContext{}

	// Generate values based on what should be propagated
	for _, propKey := range config.Propagate {
		switch propKey {
		case "user_id":
			ctx.UserID = cm.GetValue("customer_id", rng, config.Cardinality)
		case "order_id":
			ctx.OrderID = cm.GetValue("order_id", rng, config.Cardinality)
		case "correlation_id":
			ctx.CorrelationID = cm.GetValue("correlation_id", rng, config.Cardinality)
		case "session_id":
			ctx.SessionID = cm.GetValue("session_id", rng, config.Cardinality)
		case "tenant_id":
			ctx.TenantID = cm.GetValue("tenant_id", rng, config.Cardinality)
		case "region":
			ctx.Region = cm.GetValue("region", rng, config.Cardinality)
		case "datacenter":
			ctx.Datacenter = cm.GetValue("datacenter", rng, config.Cardinality)
		case "availability_zone":
			ctx.AvailabilityZone = cm.GetValue("availability_zone", rng, config.Cardinality)
		case "cluster":
			ctx.Cluster = cm.GetValue("cluster", rng, config.Cardinality)
		case "org_id":
			ctx.OrgID = cm.GetValue("org_id", rng, config.Cardinality)
		case "customer_id":
			ctx.CustomerID = cm.GetValue("customer_id", rng, config.Cardinality)
		case "version":
			ctx.Version = cm.GetValue("version", rng, config.Cardinality)
		case "git_commit":
			ctx.GitCommit = cm.GetValue("git_commit", rng, config.Cardinality)
		case "canary":
			ctx.Canary = cm.GetValue("canary", rng, config.Cardinality)
		case "user_tier":
			ctx.UserTier = cm.GetValue("user_tier", rng, config.Cardinality)
		case "priority":
			ctx.Priority = cm.GetValue("priority", rng, config.Cardinality)
		case "request_id":
			ctx.RequestID = cm.GetValue("request_id", rng, config.Cardinality)
		case "payment_id":
			ctx.PaymentID = cm.GetValue("payment_id", rng, config.Cardinality)
		case "shipment_id":
			ctx.ShipmentID = cm.GetValue("shipment_id", rng, config.Cardinality)
		case "product_id":
			ctx.ProductID = cm.GetValue("product_id", rng, config.Cardinality)
		}
	}

	return ctx
}

// GetPropagatedTags returns propagated tags as attributes
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
		tags = append(tags, newStringKeyValue("infrastructure.region", ctx.Region))
	}

	if ctx.Datacenter != "" && rng.Float64() < tagDensity {
		tags = append(tags, newStringKeyValue("infrastructure.datacenter", ctx.Datacenter))
	}

	if ctx.AvailabilityZone != "" && rng.Float64() < tagDensity {
		tags = append(tags, newStringKeyValue("infrastructure.availability_zone", ctx.AvailabilityZone))
	}

	if ctx.Cluster != "" && rng.Float64() < tagDensity {
		tags = append(tags, newStringKeyValue("infrastructure.cluster", ctx.Cluster))
	}

	// Tenant tags
	if ctx.TenantID != "" && rng.Float64() < tagDensity {
		tags = append(tags, newStringKeyValue("tenant.id", ctx.TenantID))
	}

	if ctx.CustomerID != "" && rng.Float64() < tagDensity*DensityMediumHigh {
		tags = append(tags, newStringKeyValue("tenant.customer_id", ctx.CustomerID))
	}

	if ctx.OrgID != "" && rng.Float64() < tagDensity {
		tags = append(tags, newStringKeyValue("tenant.org_id", ctx.OrgID))
	}

	// Deployment tags
	if ctx.Version != "" && rng.Float64() < tagDensity {
		tags = append(tags, newStringKeyValue("deployment.version", ctx.Version))
	}

	if ctx.GitCommit != "" && rng.Float64() < tagDensity*DensityHigh {
		tags = append(tags, newStringKeyValue("deployment.git_commit", ctx.GitCommit))
	}

	if ctx.Canary != "" && rng.Float64() < tagDensity*DensityVeryLow {
		tags = append(tags, newStringKeyValue("deployment.canary", ctx.Canary))
	}

	// Request context tags
	if ctx.RequestID != "" && rng.Float64() < tagDensity {
		tags = append(tags, newStringKeyValue("request.id", ctx.RequestID))
	}

	if ctx.CorrelationID != "" && rng.Float64() < tagDensity*DensityHigh {
		tags = append(tags, newStringKeyValue("request.correlation_id", ctx.CorrelationID))
	}

	if ctx.UserTier != "" && rng.Float64() < tagDensity {
		tags = append(tags, newStringKeyValue("request.user_tier", ctx.UserTier))
	}

	if ctx.Priority != "" && rng.Float64() < tagDensity*DensityMediumLow {
		tags = append(tags, newStringKeyValue("request.priority", ctx.Priority))
	}

	// Business context tags
	if ctx.UserID != "" && rng.Float64() < tagDensity {
		tags = append(tags, newStringKeyValue("user.id", ctx.UserID))
	}

	if ctx.OrderID != "" && rng.Float64() < tagDensity {
		tags = append(tags, newStringKeyValue("order.id", ctx.OrderID))
	}

	if ctx.SessionID != "" && rng.Float64() < tagDensity*DensityHigh {
		tags = append(tags, newStringKeyValue("session.id", ctx.SessionID))
	}

	if ctx.PaymentID != "" && rng.Float64() < tagDensity {
		tags = append(tags, newStringKeyValue("payment.id", ctx.PaymentID))
	}

	if ctx.ShipmentID != "" && rng.Float64() < tagDensity {
		tags = append(tags, newStringKeyValue("shipment.id", ctx.ShipmentID))
	}

	if ctx.ProductID != "" && rng.Float64() < tagDensity {
		tags = append(tags, newStringKeyValue("product.id", ctx.ProductID))
	}

	return tags
}
