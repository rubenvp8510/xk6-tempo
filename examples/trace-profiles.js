// =============================================================================
// Trace Profiles - Production-like trace configurations of varying complexity
// =============================================================================
// 
// Usage:
//   import { TRACE_PROFILES } from './trace-profiles.js';
//   const trace = tempo.generateTrace({
//     useTraceTree: true,
//     traceTree: TRACE_PROFILES.medium  // or: small, large, xlarge
//   });
//
// Profile Summary:
//   - small:   8-15 spans,   5-8 services   (Startups, MVPs, simple APIs)
//   - medium:  25-40 spans,  15-20 services (E-commerce, SaaS típico)
//   - large:   50-80 spans,  25-35 services (Fintech, Enterprise)
//   - xlarge:  100-150 spans, 40-50 services (FAANG-scale, ultra-distributed)
// =============================================================================

// Shared context configuration with adjustable cardinality
const createContext = (scale) => ({
  propagate: [
    "user_id", 
    "session_id", 
    "correlation_id", 
    "tenant_id", 
    "region",
    "request_id"
  ],
  cardinality: {
    "user_id": scale.users,
    "session_id": scale.sessions,
    "tenant_id": scale.tenants,
    "region": 5
  }
});

const defaultSettings = {
  useSemanticAttributes: true,
  enableTags: true,
  tagDensity: 0.85
};

// =============================================================================
// SMALL PROFILE - Startups, MVPs, Simple APIs
// ~8-15 spans per trace, 5-8 services
// Typical: Simple web app, basic CRUD, small team
// =============================================================================
const smallProfile = {
  seed: 0,
  context: createContext({ users: 1000, sessions: 5000, tenants: 10 }),
  defaults: defaultSettings,
  
  root: {
    service: "api-server",
    operation: "POST /api/orders",
    spanKind: "server",
    tags: { "http.method": "POST", "http.route": "/api/orders" },
    duration: { baseMs: 120, varianceMs: 40 },
    errorRate: 0.02,
    
    children: [
      // Auth check
      {
        weight: 1.0,
        parallel: false,
        node: {
          service: "api-server",
          operation: "ValidateAuth",
          spanKind: "internal",
          duration: { baseMs: 10, varianceMs: 3 },
          errorRate: 0.005,
          children: [
            {
              weight: 0.4,
              node: {
                service: "redis",
                operation: "GET session",
                spanKind: "client",
                tags: { "db.system": "redis" },
                duration: { baseMs: 2, varianceMs: 1 }
              }
            }
          ]
        }
      },
      
      // Main business logic
      {
        weight: 1.0,
        parallel: false,
        node: {
          service: "api-server",
          operation: "ProcessOrder",
          spanKind: "internal",
          duration: { baseMs: 80, varianceMs: 25 },
          errorRate: 0.015,
          children: [
            // Database operations
            {
              weight: 1.0,
              node: {
                service: "postgresql",
                operation: "SELECT products",
                spanKind: "client",
                tags: { "db.system": "postgresql", "db.operation": "SELECT" },
                duration: { baseMs: 15, varianceMs: 5 }
              }
            },
            {
              weight: 1.0,
              node: {
                service: "postgresql",
                operation: "INSERT order",
                spanKind: "client",
                tags: { "db.system": "postgresql", "db.operation": "INSERT" },
                duration: { baseMs: 12, varianceMs: 4 }
              }
            },
            {
              weight: 1.0,
              count: { min: 1, max: 3 },
              node: {
                service: "postgresql",
                operation: "INSERT order_item",
                spanKind: "client",
                tags: { "db.system": "postgresql", "db.operation": "INSERT" },
                duration: { baseMs: 5, varianceMs: 2 }
              }
            }
          ]
        }
      },
      
      // External payment
      {
        weight: 0.95,
        parallel: true,
        node: {
          service: "stripe-client",
          operation: "CreateCharge",
          spanKind: "client",
          tags: { "payment.provider": "stripe" },
          duration: { baseMs: 80, varianceMs: 30 },
          errorRate: 0.02
        }
      },
      
      // Email notification
      {
        weight: 0.8,
        parallel: true,
        node: {
          service: "sendgrid-client",
          operation: "SendEmail",
          spanKind: "client",
          tags: { "email.provider": "sendgrid" },
          duration: { baseMs: 50, varianceMs: 20 },
          errorRate: 0.01
        }
      }
    ]
  }
};

// =============================================================================
// MEDIUM PROFILE - E-commerce, SaaS típico
// ~25-40 spans per trace, 15-20 services
// Typical: Mid-size company, microservices architecture
// =============================================================================
const mediumProfile = {
  seed: 0,
  context: createContext({ users: 10000, sessions: 50000, tenants: 100 }),
  defaults: defaultSettings,
  
  root: {
    service: "api-gateway",
    operation: "POST /api/v1/checkout",
    spanKind: "server",
    tags: { "http.method": "POST", "http.route": "/api/v1/checkout", "http.scheme": "https" },
    duration: { baseMs: 250, varianceMs: 80 },
    errorRate: 0.015,
    
    children: [
      // Auth validation
      {
        weight: 1.0,
        parallel: false,
        node: {
          service: "auth-service",
          operation: "ValidateJWT",
          spanKind: "client",
          tags: { "auth.type": "jwt" },
          duration: { baseMs: 15, varianceMs: 5 },
          errorRate: 0.005,
          children: [
            {
              weight: 0.3,
              node: {
                service: "redis-cluster",
                operation: "GET session",
                spanKind: "client",
                tags: { "db.system": "redis" },
                duration: { baseMs: 2, varianceMs: 1 }
              }
            },
            {
              weight: 0.7,
              node: {
                service: "auth-db",
                operation: "SELECT user_tokens",
                spanKind: "client",
                tags: { "db.system": "postgresql" },
                duration: { baseMs: 8, varianceMs: 3 }
              }
            }
          ]
        }
      },
      
      // Checkout processing
      {
        weight: 0.98,
        parallel: true,
        node: {
          service: "checkout-service",
          operation: "ProcessCheckout",
          spanKind: "server",
          duration: { baseMs: 180, varianceMs: 50 },
          errorRate: 0.02,
          errorPropagates: true,
          
          children: [
            // Inventory
            {
              weight: 1.0,
              parallel: false,
              node: {
                service: "inventory-service",
                operation: "CheckAvailability",
                spanKind: "client",
                duration: { baseMs: 35, varianceMs: 15 },
                errorRate: 0.008,
                children: [
                  {
                    weight: 1.0,
                    count: { min: 1, max: 5 },
                    node: {
                      service: "inventory-db",
                      operation: "SELECT product_stock",
                      spanKind: "client",
                      tags: { "db.system": "postgresql" },
                      duration: { baseMs: 6, varianceMs: 2 }
                    }
                  }
                ]
              }
            },
            
            // Pricing
            {
              weight: 1.0,
              parallel: true,
              node: {
                service: "pricing-service",
                operation: "CalculateTotal",
                spanKind: "client",
                duration: { baseMs: 25, varianceMs: 8 },
                children: [
                  {
                    weight: 0.6,
                    node: {
                      service: "promotions-service",
                      operation: "ValidateDiscount",
                      spanKind: "client",
                      duration: { baseMs: 12, varianceMs: 4 }
                    }
                  },
                  {
                    weight: 1.0,
                    node: {
                      service: "tax-service",
                      operation: "CalculateTax",
                      spanKind: "client",
                      duration: { baseMs: 8, varianceMs: 3 }
                    }
                  }
                ]
              }
            },
            
            // Payment
            {
              weight: 0.95,
              parallel: false,
              node: {
                service: "payment-service",
                operation: "ProcessPayment",
                spanKind: "client",
                duration: { baseMs: 120, varianceMs: 40 },
                errorRate: 0.025,
                errorPropagates: true,
                children: [
                  {
                    weight: 1.0,
                    node: {
                      service: "payment-gateway",
                      operation: "ChargeCard",
                      spanKind: "client",
                      duration: { baseMs: 80, varianceMs: 30 },
                      errorRate: 0.02
                    }
                  },
                  {
                    weight: 0.9,
                    node: {
                      service: "fraud-detection",
                      operation: "AnalyzeTransaction",
                      spanKind: "client",
                      duration: { baseMs: 15, varianceMs: 5 }
                    }
                  }
                ]
              }
            },
            
            // Order creation
            {
              weight: 0.92,
              parallel: false,
              node: {
                service: "order-service",
                operation: "CreateOrder",
                spanKind: "client",
                duration: { baseMs: 45, varianceMs: 15 },
                errorRate: 0.005,
                children: [
                  {
                    weight: 1.0,
                    node: {
                      service: "orders-db",
                      operation: "INSERT order",
                      spanKind: "client",
                      tags: { "db.system": "postgresql" },
                      duration: { baseMs: 12, varianceMs: 4 }
                    }
                  },
                  {
                    weight: 1.0,
                    count: { min: 1, max: 8 },
                    node: {
                      service: "orders-db",
                      operation: "INSERT order_item",
                      spanKind: "client",
                      tags: { "db.system": "postgresql" },
                      duration: { baseMs: 5, varianceMs: 2 }
                    }
                  }
                ]
              }
            }
          ]
        }
      },
      
      // Notifications
      {
        weight: 0.85,
        parallel: true,
        node: {
          service: "notification-service",
          operation: "SendOrderConfirmation",
          spanKind: "producer",
          tags: { "messaging.system": "kafka" },
          duration: { baseMs: 8, varianceMs: 3 },
          children: [
            {
              weight: 0.95,
              parallel: true,
              node: {
                service: "email-service",
                operation: "SendEmail",
                spanKind: "client",
                duration: { baseMs: 25, varianceMs: 10 },
                errorRate: 0.01
              }
            },
            {
              weight: 0.6,
              parallel: true,
              node: {
                service: "sms-service",
                operation: "SendSMS",
                spanKind: "client",
                duration: { baseMs: 35, varianceMs: 15 },
                errorRate: 0.02
              }
            }
          ]
        }
      },
      
      // Analytics
      {
        weight: 1.0,
        parallel: true,
        node: {
          service: "analytics-service",
          operation: "TrackCheckout",
          spanKind: "producer",
          duration: { baseMs: 5, varianceMs: 2 },
          children: [
            {
              weight: 1.0,
              node: {
                service: "kafka",
                operation: "PRODUCE analytics-events",
                spanKind: "producer",
                tags: { "messaging.system": "kafka" },
                duration: { baseMs: 3, varianceMs: 1 }
              }
            }
          ]
        }
      },
      
      // Cache
      {
        weight: 0.9,
        parallel: true,
        node: {
          service: "cache-service",
          operation: "UpdateUserCache",
          spanKind: "client",
          duration: { baseMs: 4, varianceMs: 2 },
          children: [
            {
              weight: 1.0,
              count: { min: 1, max: 3 },
              node: {
                service: "redis-cluster",
                operation: "SET",
                spanKind: "client",
                tags: { "db.system": "redis" },
                duration: { baseMs: 2, varianceMs: 1 }
              }
            }
          ]
        }
      }
    ]
  }
};

// =============================================================================
// LARGE PROFILE - Fintech, Enterprise
// ~50-80 spans per trace, 25-35 services
// Typical: Financial services, healthcare, large enterprise
// =============================================================================
const largeProfile = {
  seed: 0,
  context: createContext({ users: 100000, sessions: 500000, tenants: 500 }),
  defaults: defaultSettings,
  
  root: {
    service: "edge-gateway",
    operation: "POST /api/v2/transactions",
    spanKind: "server",
    tags: { 
      "http.method": "POST", 
      "http.route": "/api/v2/transactions",
      "http.scheme": "https",
      "gateway.type": "edge"
    },
    duration: { baseMs: 400, varianceMs: 120 },
    errorRate: 0.01,
    
    children: [
      // WAF / Security scanning
      {
        weight: 1.0,
        parallel: false,
        node: {
          service: "waf-service",
          operation: "ScanRequest",
          spanKind: "internal",
          tags: { "security.scanner": "waf" },
          duration: { baseMs: 5, varianceMs: 2 },
          errorRate: 0.001,
          children: [
            {
              weight: 0.1,
              node: {
                service: "threat-intel",
                operation: "CheckIPReputation",
                spanKind: "client",
                duration: { baseMs: 8, varianceMs: 3 }
              }
            }
          ]
        }
      },
      
      // Rate limiting
      {
        weight: 1.0,
        parallel: false,
        node: {
          service: "rate-limiter",
          operation: "CheckQuota",
          spanKind: "internal",
          duration: { baseMs: 3, varianceMs: 1 },
          errorRate: 0.005,
          children: [
            {
              weight: 1.0,
              node: {
                service: "redis-cluster",
                operation: "INCR rate_limit",
                spanKind: "client",
                tags: { "db.system": "redis" },
                duration: { baseMs: 1, varianceMs: 0.5 }
              }
            }
          ]
        }
      },
      
      // Authentication & Authorization
      {
        weight: 1.0,
        parallel: false,
        node: {
          service: "auth-gateway",
          operation: "Authenticate",
          spanKind: "server",
          duration: { baseMs: 25, varianceMs: 8 },
          errorRate: 0.008,
          children: [
            {
              weight: 1.0,
              node: {
                service: "identity-service",
                operation: "ValidateToken",
                spanKind: "client",
                duration: { baseMs: 12, varianceMs: 4 },
                children: [
                  {
                    weight: 0.4,
                    node: {
                      service: "token-cache",
                      operation: "GET token",
                      spanKind: "client",
                      tags: { "db.system": "redis" },
                      duration: { baseMs: 2, varianceMs: 1 }
                    }
                  },
                  {
                    weight: 0.6,
                    node: {
                      service: "identity-db",
                      operation: "SELECT tokens",
                      spanKind: "client",
                      tags: { "db.system": "postgresql" },
                      duration: { baseMs: 6, varianceMs: 2 }
                    }
                  }
                ]
              }
            },
            {
              weight: 1.0,
              node: {
                service: "authorization-service",
                operation: "CheckPermissions",
                spanKind: "client",
                duration: { baseMs: 8, varianceMs: 3 },
                children: [
                  {
                    weight: 1.0,
                    node: {
                      service: "policy-engine",
                      operation: "EvaluatePolicy",
                      spanKind: "internal",
                      duration: { baseMs: 4, varianceMs: 2 }
                    }
                  }
                ]
              }
            }
          ]
        }
      },
      
      // Main transaction processing
      {
        weight: 0.98,
        parallel: true,
        node: {
          service: "transaction-orchestrator",
          operation: "ProcessTransaction",
          spanKind: "server",
          duration: { baseMs: 300, varianceMs: 100 },
          errorRate: 0.015,
          errorPropagates: true,
          
          children: [
            // Account validation
            {
              weight: 1.0,
              parallel: false,
              node: {
                service: "account-service",
                operation: "ValidateAccounts",
                spanKind: "client",
                duration: { baseMs: 40, varianceMs: 15 },
                errorRate: 0.005,
                children: [
                  {
                    weight: 1.0,
                    node: {
                      service: "account-db",
                      operation: "SELECT source_account",
                      spanKind: "client",
                      tags: { "db.system": "postgresql" },
                      duration: { baseMs: 8, varianceMs: 3 }
                    }
                  },
                  {
                    weight: 1.0,
                    node: {
                      service: "account-db",
                      operation: "SELECT dest_account",
                      spanKind: "client",
                      tags: { "db.system": "postgresql" },
                      duration: { baseMs: 8, varianceMs: 3 }
                    }
                  },
                  {
                    weight: 0.8,
                    node: {
                      service: "kyc-service",
                      operation: "VerifyCustomer",
                      spanKind: "client",
                      duration: { baseMs: 15, varianceMs: 5 }
                    }
                  }
                ]
              }
            },
            
            // Compliance checks (parallel)
            {
              weight: 1.0,
              parallel: true,
              node: {
                service: "compliance-gateway",
                operation: "RunComplianceChecks",
                spanKind: "server",
                duration: { baseMs: 60, varianceMs: 20 },
                children: [
                  {
                    weight: 1.0,
                    parallel: true,
                    node: {
                      service: "aml-service",
                      operation: "CheckAML",
                      spanKind: "client",
                      tags: { "compliance.type": "aml" },
                      duration: { baseMs: 25, varianceMs: 10 },
                      errorRate: 0.002,
                      children: [
                        {
                          weight: 1.0,
                          node: {
                            service: "sanctions-db",
                            operation: "CheckSanctionsList",
                            spanKind: "client",
                            duration: { baseMs: 10, varianceMs: 3 }
                          }
                        }
                      ]
                    }
                  },
                  {
                    weight: 1.0,
                    parallel: true,
                    node: {
                      service: "fraud-service",
                      operation: "AnalyzeFraud",
                      spanKind: "client",
                      tags: { "compliance.type": "fraud" },
                      duration: { baseMs: 35, varianceMs: 15 },
                      errorRate: 0.001,
                      children: [
                        {
                          weight: 1.0,
                          node: {
                            service: "ml-scoring",
                            operation: "PredictFraudScore",
                            spanKind: "client",
                            duration: { baseMs: 20, varianceMs: 8 }
                          }
                        },
                        {
                          weight: 0.7,
                          node: {
                            service: "rules-engine",
                            operation: "EvaluateRules",
                            spanKind: "client",
                            duration: { baseMs: 8, varianceMs: 3 }
                          }
                        }
                      ]
                    }
                  },
                  {
                    weight: 0.6,
                    parallel: true,
                    node: {
                      service: "velocity-checker",
                      operation: "CheckVelocity",
                      spanKind: "client",
                      duration: { baseMs: 12, varianceMs: 4 },
                      children: [
                        {
                          weight: 1.0,
                          node: {
                            service: "redis-cluster",
                            operation: "GET velocity_counters",
                            spanKind: "client",
                            tags: { "db.system": "redis" },
                            duration: { baseMs: 2, varianceMs: 1 }
                          }
                        }
                      ]
                    }
                  }
                ]
              }
            },
            
            // Core banking transaction
            {
              weight: 0.95,
              parallel: false,
              node: {
                service: "core-banking",
                operation: "ExecuteTransfer",
                spanKind: "client",
                duration: { baseMs: 80, varianceMs: 30 },
                errorRate: 0.008,
                errorPropagates: true,
                children: [
                  {
                    weight: 1.0,
                    node: {
                      service: "ledger-service",
                      operation: "DebitAccount",
                      spanKind: "client",
                      duration: { baseMs: 25, varianceMs: 8 },
                      children: [
                        {
                          weight: 1.0,
                          node: {
                            service: "ledger-db",
                            operation: "UPDATE balance",
                            spanKind: "client",
                            tags: { "db.system": "postgresql" },
                            duration: { baseMs: 10, varianceMs: 3 }
                          }
                        },
                        {
                          weight: 1.0,
                          node: {
                            service: "ledger-db",
                            operation: "INSERT transaction",
                            spanKind: "client",
                            tags: { "db.system": "postgresql" },
                            duration: { baseMs: 8, varianceMs: 2 }
                          }
                        }
                      ]
                    }
                  },
                  {
                    weight: 1.0,
                    node: {
                      service: "ledger-service",
                      operation: "CreditAccount",
                      spanKind: "client",
                      duration: { baseMs: 25, varianceMs: 8 },
                      children: [
                        {
                          weight: 1.0,
                          node: {
                            service: "ledger-db",
                            operation: "UPDATE balance",
                            spanKind: "client",
                            tags: { "db.system": "postgresql" },
                            duration: { baseMs: 10, varianceMs: 3 }
                          }
                        },
                        {
                          weight: 1.0,
                          node: {
                            service: "ledger-db",
                            operation: "INSERT transaction",
                            spanKind: "client",
                            tags: { "db.system": "postgresql" },
                            duration: { baseMs: 8, varianceMs: 2 }
                          }
                        }
                      ]
                    }
                  }
                ]
              }
            },
            
            // External settlement (parallel)
            {
              weight: 0.7,
              parallel: true,
              node: {
                service: "settlement-service",
                operation: "InitiateSettlement",
                spanKind: "client",
                duration: { baseMs: 100, varianceMs: 40 },
                errorRate: 0.01,
                children: [
                  {
                    weight: 0.5,
                    node: {
                      service: "swift-gateway",
                      operation: "SendSWIFTMessage",
                      spanKind: "client",
                      tags: { "settlement.network": "swift" },
                      duration: { baseMs: 60, varianceMs: 25 }
                    }
                  },
                  {
                    weight: 0.5,
                    node: {
                      service: "ach-gateway",
                      operation: "SubmitACH",
                      spanKind: "client",
                      tags: { "settlement.network": "ach" },
                      duration: { baseMs: 40, varianceMs: 15 }
                    }
                  }
                ]
              }
            }
          ]
        }
      },
      
      // Event publishing
      {
        weight: 1.0,
        parallel: true,
        node: {
          service: "event-publisher",
          operation: "PublishTransactionEvents",
          spanKind: "producer",
          duration: { baseMs: 10, varianceMs: 4 },
          children: [
            {
              weight: 1.0,
              node: {
                service: "kafka",
                operation: "PRODUCE transactions",
                spanKind: "producer",
                tags: { "messaging.system": "kafka" },
                duration: { baseMs: 4, varianceMs: 2 }
              }
            },
            {
              weight: 1.0,
              node: {
                service: "kafka",
                operation: "PRODUCE audit-log",
                spanKind: "producer",
                tags: { "messaging.system": "kafka" },
                duration: { baseMs: 3, varianceMs: 1 }
              }
            }
          ]
        }
      },
      
      // Notifications
      {
        weight: 0.9,
        parallel: true,
        node: {
          service: "notification-orchestrator",
          operation: "SendNotifications",
          spanKind: "server",
          duration: { baseMs: 15, varianceMs: 5 },
          children: [
            {
              weight: 0.95,
              parallel: true,
              node: {
                service: "email-service",
                operation: "SendTransactionEmail",
                spanKind: "client",
                duration: { baseMs: 30, varianceMs: 12 }
              }
            },
            {
              weight: 0.8,
              parallel: true,
              node: {
                service: "push-service",
                operation: "SendPushNotification",
                spanKind: "client",
                duration: { baseMs: 20, varianceMs: 8 }
              }
            },
            {
              weight: 0.3,
              parallel: true,
              node: {
                service: "sms-service",
                operation: "SendSMS",
                spanKind: "client",
                duration: { baseMs: 40, varianceMs: 15 }
              }
            }
          ]
        }
      },
      
      // Reporting
      {
        weight: 1.0,
        parallel: true,
        node: {
          service: "reporting-service",
          operation: "UpdateReports",
          spanKind: "client",
          duration: { baseMs: 8, varianceMs: 3 },
          children: [
            {
              weight: 1.0,
              node: {
                service: "clickhouse",
                operation: "INSERT analytics",
                spanKind: "client",
                tags: { "db.system": "clickhouse" },
                duration: { baseMs: 5, varianceMs: 2 }
              }
            }
          ]
        }
      }
    ]
  }
};

// =============================================================================
// XLARGE PROFILE - FAANG-scale, Ultra-distributed
// ~100-150 spans per trace, 40-50 services
// Typical: Global platforms, massive scale, complex orchestration
// =============================================================================
const xlargeProfile = {
  seed: 0,
  context: createContext({ users: 1000000, sessions: 5000000, tenants: 2000 }),
  defaults: defaultSettings,
  
  root: {
    service: "global-edge",
    operation: "POST /api/v3/orders/create",
    spanKind: "server",
    tags: { 
      "http.method": "POST", 
      "http.route": "/api/v3/orders/create",
      "edge.region": "auto",
      "edge.pop": "auto"
    },
    duration: { baseMs: 600, varianceMs: 180 },
    errorRate: 0.008,
    
    children: [
      // Edge processing
      {
        weight: 1.0,
        parallel: false,
        node: {
          service: "edge-processor",
          operation: "ProcessEdgeRequest",
          spanKind: "internal",
          duration: { baseMs: 8, varianceMs: 3 },
          children: [
            {
              weight: 1.0,
              node: {
                service: "geo-router",
                operation: "RouteToRegion",
                spanKind: "internal",
                duration: { baseMs: 2, varianceMs: 1 }
              }
            },
            {
              weight: 1.0,
              node: {
                service: "edge-cache",
                operation: "CheckCache",
                spanKind: "client",
                duration: { baseMs: 1, varianceMs: 0.5 }
              }
            }
          ]
        }
      },
      
      // Security layer
      {
        weight: 1.0,
        parallel: false,
        node: {
          service: "security-gateway",
          operation: "SecurityScan",
          spanKind: "server",
          duration: { baseMs: 15, varianceMs: 5 },
          children: [
            {
              weight: 1.0,
              parallel: true,
              node: {
                service: "waf-service",
                operation: "WAFScan",
                spanKind: "internal",
                duration: { baseMs: 3, varianceMs: 1 }
              }
            },
            {
              weight: 1.0,
              parallel: true,
              node: {
                service: "ddos-protection",
                operation: "CheckDDoS",
                spanKind: "internal",
                duration: { baseMs: 2, varianceMs: 1 }
              }
            },
            {
              weight: 1.0,
              parallel: true,
              node: {
                service: "bot-detection",
                operation: "DetectBot",
                spanKind: "client",
                duration: { baseMs: 5, varianceMs: 2 },
                children: [
                  {
                    weight: 0.3,
                    node: {
                      service: "ml-inference",
                      operation: "BotScoring",
                      spanKind: "client",
                      duration: { baseMs: 8, varianceMs: 3 }
                    }
                  }
                ]
              }
            },
            {
              weight: 1.0,
              node: {
                service: "rate-limiter",
                operation: "CheckRateLimit",
                spanKind: "internal",
                duration: { baseMs: 2, varianceMs: 1 },
                children: [
                  {
                    weight: 1.0,
                    node: {
                      service: "redis-cluster",
                      operation: "INCR",
                      spanKind: "client",
                      tags: { "db.system": "redis" },
                      duration: { baseMs: 1, varianceMs: 0.5 }
                    }
                  }
                ]
              }
            }
          ]
        }
      },
      
      // Authentication
      {
        weight: 1.0,
        parallel: false,
        node: {
          service: "auth-platform",
          operation: "Authenticate",
          spanKind: "server",
          duration: { baseMs: 30, varianceMs: 10 },
          errorRate: 0.005,
          children: [
            {
              weight: 1.0,
              node: {
                service: "token-validator",
                operation: "ValidateJWT",
                spanKind: "internal",
                duration: { baseMs: 8, varianceMs: 3 },
                children: [
                  {
                    weight: 0.5,
                    node: {
                      service: "token-cache",
                      operation: "GET",
                      spanKind: "client",
                      tags: { "db.system": "redis" },
                      duration: { baseMs: 1, varianceMs: 0.5 }
                    }
                  },
                  {
                    weight: 0.5,
                    node: {
                      service: "jwks-service",
                      operation: "FetchKeys",
                      spanKind: "client",
                      duration: { baseMs: 5, varianceMs: 2 }
                    }
                  }
                ]
              }
            },
            {
              weight: 1.0,
              node: {
                service: "session-service",
                operation: "GetSession",
                spanKind: "client",
                duration: { baseMs: 6, varianceMs: 2 },
                children: [
                  {
                    weight: 1.0,
                    node: {
                      service: "session-store",
                      operation: "GET",
                      spanKind: "client",
                      tags: { "db.system": "redis" },
                      duration: { baseMs: 2, varianceMs: 1 }
                    }
                  }
                ]
              }
            },
            {
              weight: 1.0,
              node: {
                service: "authz-service",
                operation: "Authorize",
                spanKind: "client",
                duration: { baseMs: 10, varianceMs: 4 },
                children: [
                  {
                    weight: 1.0,
                    node: {
                      service: "policy-service",
                      operation: "EvaluateOPA",
                      spanKind: "client",
                      duration: { baseMs: 5, varianceMs: 2 }
                    }
                  },
                  {
                    weight: 0.8,
                    node: {
                      service: "rbac-service",
                      operation: "CheckRoles",
                      spanKind: "client",
                      duration: { baseMs: 4, varianceMs: 2 }
                    }
                  }
                ]
              }
            }
          ]
        }
      },
      
      // API Gateway routing
      {
        weight: 1.0,
        parallel: false,
        node: {
          service: "api-gateway",
          operation: "RouteRequest",
          spanKind: "server",
          duration: { baseMs: 10, varianceMs: 3 },
          children: [
            {
              weight: 1.0,
              node: {
                service: "service-discovery",
                operation: "ResolveService",
                spanKind: "client",
                duration: { baseMs: 3, varianceMs: 1 }
              }
            },
            {
              weight: 1.0,
              node: {
                service: "load-balancer",
                operation: "SelectInstance",
                spanKind: "internal",
                duration: { baseMs: 1, varianceMs: 0.5 }
              }
            }
          ]
        }
      },
      
      // Main order orchestration
      {
        weight: 0.98,
        parallel: true,
        node: {
          service: "order-orchestrator",
          operation: "CreateOrder",
          spanKind: "server",
          duration: { baseMs: 450, varianceMs: 150 },
          errorRate: 0.012,
          errorPropagates: true,
          
          children: [
            // User context
            {
              weight: 1.0,
              parallel: true,
              node: {
                service: "user-service",
                operation: "GetUserContext",
                spanKind: "client",
                duration: { baseMs: 25, varianceMs: 8 },
                children: [
                  {
                    weight: 0.6,
                    node: {
                      service: "user-cache",
                      operation: "GET",
                      spanKind: "client",
                      tags: { "db.system": "redis" },
                      duration: { baseMs: 2, varianceMs: 1 }
                    }
                  },
                  {
                    weight: 0.4,
                    node: {
                      service: "user-db",
                      operation: "SELECT user",
                      spanKind: "client",
                      tags: { "db.system": "postgresql" },
                      duration: { baseMs: 8, varianceMs: 3 }
                    }
                  },
                  {
                    weight: 1.0,
                    node: {
                      service: "preferences-service",
                      operation: "GetPreferences",
                      spanKind: "client",
                      duration: { baseMs: 10, varianceMs: 4 }
                    }
                  }
                ]
              }
            },
            
            // Product catalog
            {
              weight: 1.0,
              parallel: true,
              node: {
                service: "catalog-service",
                operation: "GetProducts",
                spanKind: "client",
                duration: { baseMs: 40, varianceMs: 15 },
                children: [
                  {
                    weight: 0.7,
                    node: {
                      service: "catalog-cache",
                      operation: "MGET products",
                      spanKind: "client",
                      tags: { "db.system": "redis" },
                      duration: { baseMs: 3, varianceMs: 1 }
                    }
                  },
                  {
                    weight: 0.3,
                    count: { min: 1, max: 5 },
                    node: {
                      service: "catalog-db",
                      operation: "SELECT product",
                      spanKind: "client",
                      tags: { "db.system": "postgresql" },
                      duration: { baseMs: 6, varianceMs: 2 }
                    }
                  },
                  {
                    weight: 1.0,
                    node: {
                      service: "media-service",
                      operation: "GetProductImages",
                      spanKind: "client",
                      duration: { baseMs: 15, varianceMs: 5 }
                    }
                  }
                ]
              }
            },
            
            // Inventory
            {
              weight: 1.0,
              parallel: false,
              node: {
                service: "inventory-platform",
                operation: "CheckAndReserve",
                spanKind: "server",
                duration: { baseMs: 60, varianceMs: 20 },
                errorRate: 0.008,
                children: [
                  {
                    weight: 1.0,
                    count: { min: 1, max: 8 },
                    node: {
                      service: "inventory-service",
                      operation: "CheckStock",
                      spanKind: "client",
                      duration: { baseMs: 8, varianceMs: 3 },
                      children: [
                        {
                          weight: 1.0,
                          node: {
                            service: "inventory-db",
                            operation: "SELECT stock",
                            spanKind: "client",
                            tags: { "db.system": "postgresql" },
                            duration: { baseMs: 4, varianceMs: 2 }
                          }
                        }
                      ]
                    }
                  },
                  {
                    weight: 1.0,
                    count: { min: 1, max: 8 },
                    node: {
                      service: "reservation-service",
                      operation: "ReserveItem",
                      spanKind: "client",
                      duration: { baseMs: 10, varianceMs: 4 },
                      children: [
                        {
                          weight: 1.0,
                          node: {
                            service: "inventory-db",
                            operation: "UPDATE stock",
                            spanKind: "client",
                            tags: { "db.system": "postgresql" },
                            duration: { baseMs: 5, varianceMs: 2 }
                          }
                        }
                      ]
                    }
                  },
                  {
                    weight: 0.4,
                    node: {
                      service: "warehouse-service",
                      operation: "CheckWarehouseStock",
                      spanKind: "client",
                      duration: { baseMs: 25, varianceMs: 10 }
                    }
                  }
                ]
              }
            },
            
            // Pricing engine
            {
              weight: 1.0,
              parallel: true,
              node: {
                service: "pricing-engine",
                operation: "CalculatePrice",
                spanKind: "server",
                duration: { baseMs: 50, varianceMs: 18 },
                children: [
                  {
                    weight: 1.0,
                    node: {
                      service: "base-price-service",
                      operation: "GetBasePrice",
                      spanKind: "client",
                      duration: { baseMs: 8, varianceMs: 3 }
                    }
                  },
                  {
                    weight: 0.8,
                    node: {
                      service: "dynamic-pricing",
                      operation: "ApplyDynamicPricing",
                      spanKind: "client",
                      duration: { baseMs: 12, varianceMs: 5 },
                      children: [
                        {
                          weight: 1.0,
                          node: {
                            service: "ml-pricing",
                            operation: "PredictOptimalPrice",
                            spanKind: "client",
                            duration: { baseMs: 8, varianceMs: 3 }
                          }
                        }
                      ]
                    }
                  },
                  {
                    weight: 0.6,
                    node: {
                      service: "promotions-service",
                      operation: "ApplyPromotions",
                      spanKind: "client",
                      duration: { baseMs: 15, varianceMs: 5 },
                      children: [
                        {
                          weight: 1.0,
                          node: {
                            service: "promo-db",
                            operation: "SELECT promotions",
                            spanKind: "client",
                            tags: { "db.system": "postgresql" },
                            duration: { baseMs: 5, varianceMs: 2 }
                          }
                        },
                        {
                          weight: 0.7,
                          node: {
                            service: "coupon-service",
                            operation: "ValidateCoupon",
                            spanKind: "client",
                            duration: { baseMs: 8, varianceMs: 3 }
                          }
                        }
                      ]
                    }
                  },
                  {
                    weight: 1.0,
                    node: {
                      service: "tax-service",
                      operation: "CalculateTax",
                      spanKind: "client",
                      duration: { baseMs: 12, varianceMs: 4 },
                      children: [
                        {
                          weight: 1.0,
                          node: {
                            service: "tax-api",
                            operation: "GetTaxRate",
                            spanKind: "client",
                            duration: { baseMs: 6, varianceMs: 2 }
                          }
                        }
                      ]
                    }
                  },
                  {
                    weight: 1.0,
                    node: {
                      service: "shipping-calculator",
                      operation: "CalculateShipping",
                      spanKind: "client",
                      duration: { baseMs: 10, varianceMs: 4 }
                    }
                  }
                ]
              }
            },
            
            // Payment processing
            {
              weight: 0.95,
              parallel: false,
              node: {
                service: "payment-platform",
                operation: "ProcessPayment",
                spanKind: "server",
                duration: { baseMs: 150, varianceMs: 50 },
                errorRate: 0.02,
                errorPropagates: true,
                children: [
                  {
                    weight: 1.0,
                    node: {
                      service: "payment-router",
                      operation: "SelectProcessor",
                      spanKind: "internal",
                      duration: { baseMs: 5, varianceMs: 2 }
                    }
                  },
                  {
                    weight: 1.0,
                    node: {
                      service: "fraud-detection",
                      operation: "AnalyzeTransaction",
                      spanKind: "client",
                      duration: { baseMs: 25, varianceMs: 10 },
                      children: [
                        {
                          weight: 1.0,
                          node: {
                            service: "fraud-ml",
                            operation: "ScoreTransaction",
                            spanKind: "client",
                            duration: { baseMs: 15, varianceMs: 5 }
                          }
                        },
                        {
                          weight: 0.8,
                          node: {
                            service: "risk-rules",
                            operation: "EvaluateRules",
                            spanKind: "client",
                            duration: { baseMs: 8, varianceMs: 3 }
                          }
                        }
                      ]
                    }
                  },
                  {
                    weight: 0.7,
                    node: {
                      service: "stripe-gateway",
                      operation: "ChargeCard",
                      spanKind: "client",
                      tags: { "payment.processor": "stripe" },
                      duration: { baseMs: 80, varianceMs: 30 },
                      errorRate: 0.015
                    }
                  },
                  {
                    weight: 0.2,
                    node: {
                      service: "paypal-gateway",
                      operation: "ProcessPayPal",
                      spanKind: "client",
                      tags: { "payment.processor": "paypal" },
                      duration: { baseMs: 100, varianceMs: 40 },
                      errorRate: 0.02
                    }
                  },
                  {
                    weight: 0.1,
                    node: {
                      service: "crypto-gateway",
                      operation: "ProcessCrypto",
                      spanKind: "client",
                      tags: { "payment.processor": "crypto" },
                      duration: { baseMs: 200, varianceMs: 80 }
                    }
                  },
                  {
                    weight: 1.0,
                    node: {
                      service: "payment-db",
                      operation: "INSERT transaction",
                      spanKind: "client",
                      tags: { "db.system": "postgresql" },
                      duration: { baseMs: 10, varianceMs: 3 }
                    }
                  }
                ]
              }
            },
            
            // Order persistence
            {
              weight: 0.93,
              parallel: false,
              node: {
                service: "order-service",
                operation: "PersistOrder",
                spanKind: "client",
                duration: { baseMs: 60, varianceMs: 20 },
                errorRate: 0.003,
                children: [
                  {
                    weight: 1.0,
                    node: {
                      service: "order-db",
                      operation: "INSERT order",
                      spanKind: "client",
                      tags: { "db.system": "postgresql" },
                      duration: { baseMs: 15, varianceMs: 5 }
                    }
                  },
                  {
                    weight: 1.0,
                    count: { min: 1, max: 10 },
                    node: {
                      service: "order-db",
                      operation: "INSERT order_item",
                      spanKind: "client",
                      tags: { "db.system": "postgresql" },
                      duration: { baseMs: 5, varianceMs: 2 }
                    }
                  },
                  {
                    weight: 1.0,
                    node: {
                      service: "search-indexer",
                      operation: "IndexOrder",
                      spanKind: "client",
                      duration: { baseMs: 12, varianceMs: 4 },
                      children: [
                        {
                          weight: 1.0,
                          node: {
                            service: "elasticsearch",
                            operation: "INDEX orders",
                            spanKind: "client",
                            tags: { "db.system": "elasticsearch" },
                            duration: { baseMs: 8, varianceMs: 3 }
                          }
                        }
                      ]
                    }
                  }
                ]
              }
            },
            
            // Fulfillment initiation
            {
              weight: 0.9,
              parallel: true,
              node: {
                service: "fulfillment-orchestrator",
                operation: "InitiateFulfillment",
                spanKind: "producer",
                duration: { baseMs: 20, varianceMs: 8 },
                children: [
                  {
                    weight: 1.0,
                    node: {
                      service: "kafka",
                      operation: "PRODUCE fulfillment-orders",
                      spanKind: "producer",
                      tags: { "messaging.system": "kafka" },
                      duration: { baseMs: 5, varianceMs: 2 }
                    }
                  },
                  {
                    weight: 0.6,
                    node: {
                      service: "warehouse-router",
                      operation: "SelectWarehouse",
                      spanKind: "client",
                      duration: { baseMs: 10, varianceMs: 4 }
                    }
                  }
                ]
              }
            }
          ]
        }
      },
      
      // Event streaming
      {
        weight: 1.0,
        parallel: true,
        node: {
          service: "event-platform",
          operation: "PublishEvents",
          spanKind: "producer",
          duration: { baseMs: 15, varianceMs: 5 },
          children: [
            {
              weight: 1.0,
              node: {
                service: "kafka",
                operation: "PRODUCE order-events",
                spanKind: "producer",
                tags: { "messaging.system": "kafka" },
                duration: { baseMs: 4, varianceMs: 2 }
              }
            },
            {
              weight: 1.0,
              node: {
                service: "kafka",
                operation: "PRODUCE analytics-events",
                spanKind: "producer",
                tags: { "messaging.system": "kafka" },
                duration: { baseMs: 3, varianceMs: 1 }
              }
            },
            {
              weight: 1.0,
              node: {
                service: "kafka",
                operation: "PRODUCE audit-events",
                spanKind: "producer",
                tags: { "messaging.system": "kafka" },
                duration: { baseMs: 3, varianceMs: 1 }
              }
            }
          ]
        }
      },
      
      // Notification fanout
      {
        weight: 0.9,
        parallel: true,
        node: {
          service: "notification-platform",
          operation: "SendNotifications",
          spanKind: "server",
          duration: { baseMs: 25, varianceMs: 10 },
          children: [
            {
              weight: 0.95,
              parallel: true,
              node: {
                service: "email-service",
                operation: "SendOrderEmail",
                spanKind: "client",
                duration: { baseMs: 30, varianceMs: 12 },
                children: [
                  {
                    weight: 1.0,
                    node: {
                      service: "template-engine",
                      operation: "RenderTemplate",
                      spanKind: "internal",
                      duration: { baseMs: 5, varianceMs: 2 }
                    }
                  },
                  {
                    weight: 1.0,
                    node: {
                      service: "sendgrid",
                      operation: "Send",
                      spanKind: "client",
                      duration: { baseMs: 20, varianceMs: 8 }
                    }
                  }
                ]
              }
            },
            {
              weight: 0.7,
              parallel: true,
              node: {
                service: "push-service",
                operation: "SendPush",
                spanKind: "client",
                duration: { baseMs: 20, varianceMs: 8 },
                children: [
                  {
                    weight: 0.6,
                    node: {
                      service: "fcm-gateway",
                      operation: "SendFCM",
                      spanKind: "client",
                      duration: { baseMs: 15, varianceMs: 5 }
                    }
                  },
                  {
                    weight: 0.4,
                    node: {
                      service: "apns-gateway",
                      operation: "SendAPNS",
                      spanKind: "client",
                      duration: { baseMs: 12, varianceMs: 4 }
                    }
                  }
                ]
              }
            },
            {
              weight: 0.4,
              parallel: true,
              node: {
                service: "sms-service",
                operation: "SendSMS",
                spanKind: "client",
                duration: { baseMs: 40, varianceMs: 15 },
                children: [
                  {
                    weight: 1.0,
                    node: {
                      service: "twilio-gateway",
                      operation: "SendMessage",
                      spanKind: "client",
                      duration: { baseMs: 30, varianceMs: 12 }
                    }
                  }
                ]
              }
            },
            {
              weight: 0.3,
              parallel: true,
              node: {
                service: "webhook-service",
                operation: "TriggerWebhooks",
                spanKind: "client",
                duration: { baseMs: 50, varianceMs: 20 },
                count: { min: 1, max: 3 }
              }
            }
          ]
        }
      },
      
      // Analytics & ML
      {
        weight: 1.0,
        parallel: true,
        node: {
          service: "analytics-platform",
          operation: "TrackEvent",
          spanKind: "client",
          duration: { baseMs: 12, varianceMs: 4 },
          children: [
            {
              weight: 1.0,
              node: {
                service: "clickhouse",
                operation: "INSERT events",
                spanKind: "client",
                tags: { "db.system": "clickhouse" },
                duration: { baseMs: 5, varianceMs: 2 }
              }
            },
            {
              weight: 0.8,
              node: {
                service: "recommendation-engine",
                operation: "UpdateModel",
                spanKind: "client",
                duration: { baseMs: 8, varianceMs: 3 }
              }
            }
          ]
        }
      },
      
      // Cache warming
      {
        weight: 0.85,
        parallel: true,
        node: {
          service: "cache-warmer",
          operation: "WarmCaches",
          spanKind: "client",
          duration: { baseMs: 8, varianceMs: 3 },
          children: [
            {
              weight: 1.0,
              count: { min: 2, max: 5 },
              node: {
                service: "redis-cluster",
                operation: "SET",
                spanKind: "client",
                tags: { "db.system": "redis" },
                duration: { baseMs: 2, varianceMs: 1 }
              }
            },
            {
              weight: 0.6,
              node: {
                service: "memcached",
                operation: "SET",
                spanKind: "client",
                tags: { "db.system": "memcached" },
                duration: { baseMs: 1, varianceMs: 0.5 }
              }
            }
          ]
        }
      }
    ]
  }
};

// =============================================================================
// EXPORTS
// =============================================================================
export const TRACE_PROFILES = {
  small: smallProfile,
  medium: mediumProfile,
  large: largeProfile,
  xlarge: xlargeProfile
};

// Default export for convenience
export default TRACE_PROFILES;

