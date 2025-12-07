package generator

import (
	"math/rand"

	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	tracev1 "go.opentelemetry.io/proto/otlp/trace/v1"
)

// Operation name templates per service type
var operationTemplates = map[string][]string{
	"frontend": {
		"GET /",
		"GET /api/users",
		"GET /api/orders",
		"POST /api/orders",
		"PUT /api/users/{id}",
		"DELETE /api/users/{id}",
		"GET /static/*",
		"GET /health",
		"POST /api/auth/login",
		"GET /api/products",
	},
	"backend": {
		"ProcessRequest",
		"ValidateInput",
		"HandleWebhook",
		"ProcessPayment",
		"SendNotification",
		"UpdateInventory",
		"CalculateShipping",
		"GenerateReport",
	},
	"database": {
		"SELECT",
		"INSERT",
		"UPDATE",
		"DELETE",
		"BEGIN TRANSACTION",
		"COMMIT",
		"ROLLBACK",
		"EXECUTE PROCEDURE",
	},
	"cache": {
		"GET",
		"SET",
		"MGET",
		"MSET",
		"EXPIRE",
		"DEL",
		"HGET",
		"HSET",
	},
	"auth": {
		"Authenticate",
		"Authorize",
		"ValidateToken",
		"RefreshToken",
		"RevokeToken",
	},
	"payment": {
		"ProcessPayment",
		"Refund",
		"Authorize",
		"Capture",
		"Void",
	},
	"shipping": {
		"CalculateShipping",
		"CreateShipment",
		"TrackShipment",
		"UpdateStatus",
	},
	"analytics": {
		"TrackEvent",
		"RecordMetric",
		"GenerateReport",
		"QueryData",
	},
	"notification": {
		"SendEmail",
		"SendSMS",
		"SendPush",
		"ScheduleNotification",
	},
	"gateway": {
		"RouteRequest",
		"LoadBalance",
		"RateLimit",
		"Authenticate",
	},
}

// Error messages for realistic error injection
var errorMessages = []string{
	"connection timeout",
	"database connection failed",
	"invalid request",
	"authentication failed",
	"rate limit exceeded",
	"service unavailable",
	"internal server error",
	"not found",
	"permission denied",
	"request timeout",
}

// generateOperationName generates a realistic operation name based on service
func generateOperationName(serviceName string, rng *rand.Rand) string {
	templates, ok := operationTemplates[serviceName]
	if !ok || len(templates) == 0 {
		return serviceName + "-operation"
	}
	return templates[rng.Intn(len(templates))]
}

// generateSemanticAttributes generates OTel semantic convention attributes
func generateSemanticAttributes(kind tracev1.Span_SpanKind, serviceName string, rng *rand.Rand) []*commonv1.KeyValue {
	attrs := make([]*commonv1.KeyValue, 0)

	switch kind {
	case tracev1.Span_SPAN_KIND_SERVER, tracev1.Span_SPAN_KIND_CLIENT:
		// HTTP attributes
		methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
		method := methods[rng.Intn(len(methods))]
		attrs = append(attrs, &commonv1.KeyValue{
			Key: "http.method",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: method,
				},
			},
		})

		statusCodes := []int{200, 201, 204, 400, 401, 403, 404, 500, 502, 503}
		statusCode := statusCodes[rng.Intn(len(statusCodes))]
		attrs = append(attrs, &commonv1.KeyValue{
			Key: "http.status_code",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_IntValue{
					IntValue: int64(statusCode),
				},
			},
		})

		// URL based on service
		var url string
		switch serviceName {
		case "frontend":
			urls := []string{"/api/users", "/api/orders", "/api/products", "/health", "/static/app.js"}
			url = urls[rng.Intn(len(urls))]
		case "backend":
			urls := []string{"/v1/process", "/v1/validate", "/v1/webhook"}
			url = urls[rng.Intn(len(urls))]
		default:
			url = "/api/" + serviceName
		}
		attrs = append(attrs, &commonv1.KeyValue{
			Key: "http.url",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: url,
				},
			},
		})

		attrs = append(attrs, &commonv1.KeyValue{
			Key: "http.scheme",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: "https",
				},
			},
		})

	case tracev1.Span_SPAN_KIND_INTERNAL:
		// Internal service attributes
		attrs = append(attrs, &commonv1.KeyValue{
			Key: "service.operation",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: "internal-process",
				},
			},
		})
	}

	// Database attributes
	if serviceName == "database" {
		dbSystems := []string{"postgresql", "mysql", "mongodb", "redis"}
		dbSystem := dbSystems[rng.Intn(len(dbSystems))]
		attrs = append(attrs, &commonv1.KeyValue{
			Key: "db.system",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: dbSystem,
				},
			},
		})

		statements := []string{
			"SELECT * FROM users WHERE id = ?",
			"INSERT INTO orders (user_id, total) VALUES (?, ?)",
			"UPDATE products SET stock = ? WHERE id = ?",
			"DELETE FROM sessions WHERE expires_at < ?",
		}
		statement := statements[rng.Intn(len(statements))]
		attrs = append(attrs, &commonv1.KeyValue{
			Key: "db.statement",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: statement,
				},
			},
		})
	}

	// Cache attributes
	if serviceName == "cache" {
		attrs = append(attrs, &commonv1.KeyValue{
			Key: "db.system",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: "redis",
				},
			},
		})

		operations := []string{"GET", "SET", "MGET", "MSET", "DEL"}
		operation := operations[rng.Intn(len(operations))]
		attrs = append(attrs, &commonv1.KeyValue{
			Key: "db.operation",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: operation,
				},
			},
		})
	}

	// RPC attributes for backend services
	if serviceName == "backend" || serviceName == "gateway" {
		attrs = append(attrs, &commonv1.KeyValue{
			Key: "rpc.service",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: serviceName + ".Service",
				},
			},
		})

		methods := []string{"Process", "Validate", "Handle", "Execute"}
		method := methods[rng.Intn(len(methods))]
		attrs = append(attrs, &commonv1.KeyValue{
			Key: "rpc.method",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: method,
				},
			},
		})
	}

	return attrs
}

// generateResourceAttributes generates realistic resource attributes
func generateResourceAttributes(serviceName string, rng *rand.Rand) map[string]string {
	attrs := make(map[string]string)

	// Service version
	versions := []string{"1.0.0", "1.1.0", "1.2.0", "2.0.0"}
	attrs["service.version"] = versions[rng.Intn(len(versions))]

	// Host name
	hosts := []string{"host-01", "host-02", "host-03", "pod-abc123", "pod-def456"}
	attrs["host.name"] = hosts[rng.Intn(len(hosts))]

	// Container/Pod attributes (for K8s)
	if rng.Float64() < 0.7 { // 70% chance of K8s attributes
		pods := []string{"pod-abc123", "pod-def456", "pod-ghi789"}
		attrs["k8s.pod.name"] = pods[rng.Intn(len(pods))]

		namespaces := []string{"production", "staging", "default"}
		attrs["k8s.namespace.name"] = namespaces[rng.Intn(len(namespaces))]

		containers := []string{"app", "sidecar", "init"}
		attrs["k8s.container.name"] = containers[rng.Intn(len(containers))]
	}

	// Deployment environment
	envs := []string{"production", "staging", "development"}
	attrs["deployment.environment"] = envs[rng.Intn(len(envs))]

	return attrs
}

