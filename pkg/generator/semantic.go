package generator

import (
	"fmt"
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

// generateBusinessAttributes generates business domain attributes based on workflow context
func generateBusinessAttributes(ctx *WorkflowContext, serviceName string, config Config, rng *rand.Rand) []*commonv1.KeyValue {
	attrs := make([]*commonv1.KeyValue, 0)
	
	if ctx == nil {
		return attrs
	}
	
	density := config.BusinessAttributesDensity
	if density <= 0 {
		density = 0.8 // Default 80%
	}
	if density > 1 {
		density = 1
	}
	
	// Add user_id to most services
	if rng.Float64() < density && ctx.UserID != "" {
		attrs = append(attrs, &commonv1.KeyValue{
			Key: "user.id",
			Value: &commonv1.AnyValue{
				Value: &commonv1.AnyValue_StringValue{
					StringValue: ctx.UserID,
				},
			},
		})
	}
	
	// Service-specific business attributes
	switch serviceName {
	case "auth":
		if rng.Float64() < density && ctx.SessionID != "" {
			attrs = append(attrs, &commonv1.KeyValue{
				Key: "session.id",
				Value: &commonv1.AnyValue{
					Value: &commonv1.AnyValue_StringValue{
						StringValue: ctx.SessionID,
					},
				},
			})
		}
		if rng.Float64() < density*0.6 {
			authMethods := []string{"password", "oauth", "jwt", "saml"}
			attrs = append(attrs, &commonv1.KeyValue{
				Key: "auth.method",
				Value: &commonv1.AnyValue{
					Value: &commonv1.AnyValue_StringValue{
						StringValue: authMethods[rng.Intn(len(authMethods))],
					},
				},
			})
		}
		if rng.Float64() < density*0.5 {
			tokenTypes := []string{"bearer", "api_key", "session"}
			attrs = append(attrs, &commonv1.KeyValue{
				Key: "auth.token_type",
				Value: &commonv1.AnyValue{
					Value: &commonv1.AnyValue_StringValue{
						StringValue: tokenTypes[rng.Intn(len(tokenTypes))],
					},
				},
			})
		}
		
	case "payment":
		if rng.Float64() < density && ctx.PaymentID != "" {
			attrs = append(attrs, &commonv1.KeyValue{
				Key: "payment.id",
				Value: &commonv1.AnyValue{
					Value: &commonv1.AnyValue_StringValue{
						StringValue: ctx.PaymentID,
					},
				},
			})
		}
		if rng.Float64() < density {
			amount := float64(rng.Intn(10000)+100) / 100.0 // $1.00 to $100.00
			attrs = append(attrs, &commonv1.KeyValue{
				Key: "payment.amount",
				Value: &commonv1.AnyValue{
					Value: &commonv1.AnyValue_DoubleValue{
						DoubleValue: amount,
					},
				},
			})
		}
		if rng.Float64() < density {
			currencies := []string{"USD", "EUR", "GBP", "JPY"}
			attrs = append(attrs, &commonv1.KeyValue{
				Key: "payment.currency",
				Value: &commonv1.AnyValue{
					Value: &commonv1.AnyValue_StringValue{
						StringValue: currencies[rng.Intn(len(currencies))],
					},
				},
			})
		}
		if rng.Float64() < density*0.8 {
			methods := []string{"credit_card", "debit_card", "paypal", "bank_transfer"}
			attrs = append(attrs, &commonv1.KeyValue{
				Key: "payment.method",
				Value: &commonv1.AnyValue{
					Value: &commonv1.AnyValue_StringValue{
						StringValue: methods[rng.Intn(len(methods))],
					},
				},
			})
		}
		if rng.Float64() < density*0.7 {
			statuses := []string{"pending", "completed", "failed", "refunded"}
			attrs = append(attrs, &commonv1.KeyValue{
				Key: "payment.status",
				Value: &commonv1.AnyValue{
					Value: &commonv1.AnyValue_StringValue{
						StringValue: statuses[rng.Intn(len(statuses))],
					},
				},
			})
		}
		
	case "database":
		if rng.Float64() < density*0.8 {
			tables := []string{"users", "orders", "products", "sessions", "payments", "shipments"}
			table := tables[rng.Intn(len(tables))]
			attrs = append(attrs, &commonv1.KeyValue{
				Key: "db.table",
				Value: &commonv1.AnyValue{
					Value: &commonv1.AnyValue_StringValue{
						StringValue: table,
					},
				},
			})
		}
		if rng.Float64() < density*0.6 {
			queryTypes := []string{"SELECT", "INSERT", "UPDATE", "DELETE"}
			attrs = append(attrs, &commonv1.KeyValue{
				Key: "db.query_type",
				Value: &commonv1.AnyValue{
					Value: &commonv1.AnyValue_StringValue{
						StringValue: queryTypes[rng.Intn(len(queryTypes))],
					},
				},
			})
		}
		if rng.Float64() < density*0.5 {
			rowsAffected := rng.Intn(1000) + 1
			attrs = append(attrs, &commonv1.KeyValue{
				Key: "db.rows_affected",
				Value: &commonv1.AnyValue{
					Value: &commonv1.AnyValue_IntValue{
						IntValue: int64(rowsAffected),
					},
				},
			})
		}
		if rng.Float64() < density*0.4 {
			cacheHit := rng.Float64() < 0.3 // 30% cache hit rate
			attrs = append(attrs, &commonv1.KeyValue{
				Key: "db.cache_hit",
				Value: &commonv1.AnyValue{
					Value: &commonv1.AnyValue_BoolValue{
						BoolValue: cacheHit,
					},
				},
			})
		}
		
	case "cache":
		if rng.Float64() < density {
			cacheKey := fmt.Sprintf("cache:%s:%d", serviceName, rng.Intn(10000))
			attrs = append(attrs, &commonv1.KeyValue{
				Key: "cache.key",
				Value: &commonv1.AnyValue{
					Value: &commonv1.AnyValue_StringValue{
						StringValue: cacheKey,
					},
				},
			})
		}
		if rng.Float64() < density*0.8 {
			cacheHit := rng.Float64() < 0.7 // 70% cache hit rate
			attrs = append(attrs, &commonv1.KeyValue{
				Key: "cache.hit",
				Value: &commonv1.AnyValue{
					Value: &commonv1.AnyValue_BoolValue{
						BoolValue: cacheHit,
					},
				},
			})
		}
		if rng.Float64() < density*0.5 {
			ttl := rng.Intn(3600) + 60 // 60 to 3660 seconds
			attrs = append(attrs, &commonv1.KeyValue{
				Key: "cache.ttl",
				Value: &commonv1.AnyValue{
					Value: &commonv1.AnyValue_IntValue{
						IntValue: int64(ttl),
					},
				},
			})
		}
		
	case "shipping":
		if rng.Float64() < density && ctx.ShipmentID != "" {
			attrs = append(attrs, &commonv1.KeyValue{
				Key: "shipment.id",
				Value: &commonv1.AnyValue{
					Value: &commonv1.AnyValue_StringValue{
						StringValue: ctx.ShipmentID,
					},
				},
			})
		}
		if rng.Float64() < density*0.8 {
			carriers := []string{"UPS", "FedEx", "DHL", "USPS"}
			attrs = append(attrs, &commonv1.KeyValue{
				Key: "shipment.carrier",
				Value: &commonv1.AnyValue{
					Value: &commonv1.AnyValue_StringValue{
						StringValue: carriers[rng.Intn(len(carriers))],
					},
				},
			})
		}
		if rng.Float64() < density*0.6 {
			trackingNumber := fmt.Sprintf("TRK%012d", rng.Intn(1000000000000))
			attrs = append(attrs, &commonv1.KeyValue{
				Key: "shipment.tracking_number",
				Value: &commonv1.AnyValue{
					Value: &commonv1.AnyValue_StringValue{
						StringValue: trackingNumber,
					},
				},
			})
		}
		if rng.Float64() < density*0.5 {
			destinations := []string{"US", "CA", "UK", "DE", "FR", "JP"}
			attrs = append(attrs, &commonv1.KeyValue{
				Key: "shipment.destination",
				Value: &commonv1.AnyValue{
					Value: &commonv1.AnyValue_StringValue{
						StringValue: destinations[rng.Intn(len(destinations))],
					},
				},
			})
		}
		
	case "analytics":
		if rng.Float64() < density {
			events := []string{"page_view", "click", "purchase", "search", "login", "logout"}
			attrs = append(attrs, &commonv1.KeyValue{
				Key: "analytics.event_name",
				Value: &commonv1.AnyValue{
					Value: &commonv1.AnyValue_StringValue{
						StringValue: events[rng.Intn(len(events))],
					},
				},
			})
		}
		if rng.Float64() < density*0.6 {
			pageViews := rng.Intn(10) + 1
			attrs = append(attrs, &commonv1.KeyValue{
				Key: "analytics.page_views",
				Value: &commonv1.AnyValue{
					Value: &commonv1.AnyValue_IntValue{
						IntValue: int64(pageViews),
					},
				},
			})
		}
		if rng.Float64() < density*0.5 {
			sessionDuration := rng.Intn(3600) + 60 // 60 to 3660 seconds
			attrs = append(attrs, &commonv1.KeyValue{
				Key: "analytics.session_duration_seconds",
				Value: &commonv1.AnyValue{
					Value: &commonv1.AnyValue_IntValue{
						IntValue: int64(sessionDuration),
					},
				},
			})
		}
		
	case "frontend", "backend":
		if rng.Float64() < density && ctx.OrderID != "" {
			attrs = append(attrs, &commonv1.KeyValue{
				Key: "order.id",
				Value: &commonv1.AnyValue{
					Value: &commonv1.AnyValue_StringValue{
						StringValue: ctx.OrderID,
					},
				},
			})
		}
		if rng.Float64() < density && ctx.ProductID != "" {
			attrs = append(attrs, &commonv1.KeyValue{
				Key: "product.id",
				Value: &commonv1.AnyValue{
					Value: &commonv1.AnyValue_StringValue{
						StringValue: ctx.ProductID,
					},
				},
			})
		}
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

