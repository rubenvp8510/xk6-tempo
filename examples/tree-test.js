import tempo from 'k6/x/tempo';

export const options = {
  scenarios: {
    ingestion: {
      executor: 'constant-arrival-rate',
      rate: 100,  // 100 iterations/sec
      duration: '5m',
      preAllocatedVUs: 10,
      maxVUs: 50,
    },
  },
  thresholds: {
    'tempo_ingestion_rate_bytes_per_sec': ['rate>0'],
    'tempo_ingestion_traces_total': ['count>0'],
  },
};

const client = tempo.IngestClient({
  endpoint: 'tempo-simplest:4317',
  protocol: 'otlp-grpc',
  tenant:  'tenant-1',
  timeout: 30,
});

// Definir árbol de trazas con seed para reproducibilidad
const traceTree = {
  seed: 12345,  // Mismo seed = misma estructura de trazas (servicios, tags, IDs)
  
  context: {
    propagate: ["user_id", "order_id", "correlation_id", "session_id", "tenant_id", "region"],
    cardinality: {
      "user_id": 5000,
      "tenant_id": 50,
      "region": 5
    }
  },
  
  defaults: {
    useSemanticAttributes: true,
    enableTags: true,
    tagDensity: 0.9
  },
  
  root: {
    service: "frontend",
    operation: "POST /api/orders",
    spanKind: "server",
    tags: { "http.method": "POST", "http.route": "/api/orders" },
    duration: { baseMs: 200, varianceMs: 50 },
    errorRate: 0.02,
    
    children: [
      {
        weight: 1.0,          // Siempre se ejecuta
        parallel: false,       // Secuencial
        node: {
          service: "auth",
          operation: "ValidateToken",
          spanKind: "client",
          duration: { baseMs: 25, varianceMs: 5 },
          errorRate: 0.01
        }
      },
      {
        weight: 0.95,         // 95% probabilidad
        parallel: true,        // En paralelo con el siguiente
        node: {
          service: "backend",
          operation: "ProcessOrder",
          spanKind: "server",
          duration: { baseMs: 150, varianceMs: 40 },
          errorRate: 0.01,
          errorPropagates: true,  // Si falla, el padre también falla
          children: [
            {
              count: { min: 1, max: 3 },  // 1-3 queries a DB
              node: {
                service: "database",
                operation: "SELECT products",
                spanKind: "client",
                duration: { baseMs: 30, varianceMs: 10 }
              }
            },
            {
              weight: 0.8,
              node: {
                service: "cache",
                operation: "SET",
                spanKind: "client",
                duration: { baseMs: 5, varianceMs: 2 }
              }
            }
          ]
        }
      },
      {
        weight: 0.7,          // 70% probabilidad
        parallel: true,        // En paralelo con el anterior
        node: {
          service: "notification",
          operation: "SendEmail",
          spanKind: "client",
          duration: { baseMs: 80, varianceMs: 20 }
        }
      }
    ]
  }
};

export default function() {
  // Generar traza usando el árbol
  const trace = tempo.generateTrace({
    useTraceTree: true,
    traceTree: traceTree
  });
  
  const err = client.push(trace);
  if (err) {
    console.error('Failed to push trace:', err);
  }
}

