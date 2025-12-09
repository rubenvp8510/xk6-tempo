import tempo from 'k6/x/tempo';

/**
 * Ejemplo de generación determinista de trazas.
 * 
 * Con el mismo seed, la estructura de las trazas (servicios, operaciones,
 * tags, IDs) será idéntica entre ejecuciones, permitiendo:
 * - Comparar rendimiento entre versiones de Tempo
 * - Debugging reproducible
 * - Tests de regresión
 */

export const options = {
  scenarios: {
    deterministic: {
      executor: 'per-vu-iterations',
      vus: 1,
      iterations: 10,  // Generar 10 trazas idénticas en cada ejecución
    },
  },
};

const client = tempo.IngestClient({
  endpoint: 'tempo-simplest:4317',
  protocol: 'otlp-grpc',
  tenant: 'tenant-1',
  timeout: 30,
});

// Configuración del árbol con seed fijo para reproducibilidad
const BASE_SEED = 42;

function createDeterministicTree(iterationSeed) {
  return {
    // Seed único por iteración pero reproducible
    seed: BASE_SEED + iterationSeed,
    
    context: {
      propagate: ["user_id", "order_id", "correlation_id"],
      cardinality: {
        "user_id": 100,      // Pool pequeño para facilitar comparación
        "tenant_id": 10,
        "region": 3
      }
    },
    
    defaults: {
      useSemanticAttributes: true,
      enableTags: true,
      tagDensity: 0.9
    },
    
    root: {
      service: "api-gateway",
      operation: "POST /checkout",
      spanKind: "server",
      tags: { "http.method": "POST" },
      duration: { baseMs: 150, varianceMs: 20 },
      errorRate: 0.02,
      
      children: [
        {
          weight: 1.0,
          parallel: false,
          node: {
            service: "auth-service",
            operation: "ValidateSession",
            spanKind: "client",
            duration: { baseMs: 30, varianceMs: 5 }
          }
        },
        {
          weight: 1.0,
          parallel: true,
          node: {
            service: "inventory-service",
            operation: "CheckStock",
            spanKind: "server",
            duration: { baseMs: 50, varianceMs: 15 },
            children: [
              {
                weight: 1.0,
                node: {
                  service: "postgres",
                  operation: "SELECT inventory",
                  spanKind: "client",
                  duration: { baseMs: 20, varianceMs: 5 }
                }
              }
            ]
          }
        },
        {
          weight: 1.0,
          parallel: true,
          node: {
            service: "pricing-service",
            operation: "CalculateTotal",
            spanKind: "server",
            duration: { baseMs: 40, varianceMs: 10 },
            children: [
              {
                weight: 0.7,
                node: {
                  service: "redis",
                  operation: "GET pricing_cache",
                  spanKind: "client",
                  duration: { baseMs: 5, varianceMs: 2 }
                }
              }
            ]
          }
        },
        {
          weight: 0.95,
          parallel: false,
          node: {
            service: "payment-service",
            operation: "ProcessPayment",
            spanKind: "server",
            duration: { baseMs: 200, varianceMs: 50 },
            errorRate: 0.03,
            errorPropagates: true,
            children: [
              {
                weight: 1.0,
                node: {
                  service: "stripe-client",
                  operation: "Charge",
                  spanKind: "client",
                  duration: { baseMs: 150, varianceMs: 30 }
                }
              }
            ]
          }
        }
      ]
    }
  };
}

export default function() {
  // Usar __ITER para crear seeds únicos pero reproducibles por iteración
  const iterationSeed = __ITER;
  
  const traceTree = createDeterministicTree(iterationSeed);
  
  const trace = tempo.generateTrace({
    useTraceTree: true,
    traceTree: traceTree
  });
  
  console.log(`Iteration ${__ITER}: Generated trace with seed ${BASE_SEED + iterationSeed}`);
  
  const err = client.push(trace);
  if (err) {
    console.error('Failed to push trace:', err);
  }
}

export function handleSummary(data) {
  return {
    'stdout': `
=== Deterministic Test Summary ===
Seeds used: ${BASE_SEED} to ${BASE_SEED + 9}

To reproduce the same trace structure, run this test again.
The trace IDs, span IDs, services, operations, and tags will be identical.
`,
  };
}

