import tempo from 'k6/x/tempo';
import { TRACE_PROFILES } from './trace-profiles.js';

// =============================================================================
// CONFIGURATION - Override via environment variables
// =============================================================================

// Trace profile: 'small' | 'medium' | 'large' | 'xlarge'
const PROFILE = __ENV.TRACE_PROFILE || 'medium';

// Rates
const INGESTION_RATE = parseInt(__ENV.INGESTION_RATE || '50');  // traces/sec
const QUERY_RATE = parseInt(__ENV.QUERY_RATE || '25');          // queries/sec
const DURATION = __ENV.DURATION || '5m';

// Endpoints
const INGEST_ENDPOINT = __ENV.TEMPO_ENDPOINT || 'tempo-simplest:4317';
const INGEST_PROTOCOL = __ENV.TEMPO_PROTOCOL || 'otlp-grpc';
const QUERY_ENDPOINT = __ENV.TEMPO_QUERY_ENDPOINT || 'http://tempo-query-frontend:3200';
const TENANT = __ENV.TEMPO_TENANT || 'tenant-1';

// =============================================================================
// K6 OPTIONS
// =============================================================================

export const options = {
  scenarios: {
    ingestion: {
      executor: 'constant-arrival-rate',
      rate: INGESTION_RATE,
      duration: DURATION,
      preAllocatedVUs: 5,
      maxVUs: 20,
      exec: 'ingest',
    },
    queries: {
      executor: 'constant-arrival-rate',
      rate: QUERY_RATE,
      duration: DURATION,
      preAllocatedVUs: 10,
      maxVUs: 50,
      exec: 'query',
    },
  },
  thresholds: {
    'tempo_ingestion_rate_bytes_per_sec': ['rate>0'],
    'tempo_ingestion_traces_total': ['count>0'],
    'tempo_query_duration_seconds': ['p(95)<2'],
    'tempo_query_failures_total': ['rate<1'],
  },
};

// =============================================================================
// CLIENTS SETUP
// =============================================================================

const ingestionClient = tempo.IngestClient({
  endpoint: INGEST_ENDPOINT,
  protocol: INGEST_PROTOCOL,
  tenant: TENANT,
  timeout: 30,
});

const queryClient = tempo.QueryClient({
  endpoint: QUERY_ENDPOINT,
  tenant: TENANT,
  timeout: 30,
});

// Get selected profile
const traceProfile = TRACE_PROFILES[PROFILE];
if (!traceProfile) {
  throw new Error(`Invalid TRACE_PROFILE: ${PROFILE}. Valid options: small, medium, large, xlarge`);
}

// Query patterns based on the services in our trace profiles
const queries = [
  '{service.name="api-gateway"}',
  '{service.name="checkout-service"}',
  '{service.name="payment-service"}',
  '{service.name="order-service"}',
  '{status=error}',
  '{duration>100ms}',
  '{service.name="api-gateway" && status=error}',
  '{service.name="payment-service" && duration>50ms}',
];

// Log configuration on startup
console.log(`
================================================================================
xk6-tempo Combined Test (Ingestion + Queries)
================================================================================
Profile:          ${PROFILE}
Ingestion Rate:   ${INGESTION_RATE} traces/sec
Query Rate:       ${QUERY_RATE} QPS
Duration:         ${DURATION}
Ingest Endpoint:  ${INGEST_ENDPOINT}
Query Endpoint:   ${QUERY_ENDPOINT}
Tenant:           ${TENANT}
================================================================================
`);

// =============================================================================
// TEST FUNCTIONS
// =============================================================================

export function ingest() {
  const trace = tempo.generateTrace({
    useTraceTree: true,
    traceTree: traceProfile
  });
  
  const err = ingestionClient.push(trace);
  if (err) {
    console.error('Failed to push trace:', err);
  }
}

export function query() {
  const q = queries[Math.floor(Math.random() * queries.length)];
  
  const result = queryClient.Search(q, {
    start: '1h',
    end: 'now',
    limit: 20,
  });
  
  if (!result) {
    console.error('Query failed:', q);
    return;
  }
  
  // 10% chance to fetch full trace details
  if (result.traces && result.traces.length > 0 && Math.random() < 0.1) {
    const traceId = result.traces[0].traceID;
    const fullTrace = queryClient.GetTrace(traceId);
    if (!fullTrace) {
      console.error('Failed to fetch trace:', traceId);
    }
  }
}

// =============================================================================
// USAGE EXAMPLES
// =============================================================================
// 
// Basic usage (medium profile, default rates):
//   ./k6 run examples/combined-test.js
//
// High ingestion, low query:
//   TRACE_PROFILE=small INGESTION_RATE=200 QUERY_RATE=10 ./k6 run examples/combined-test.js
//
// Enterprise load test:
//   TRACE_PROFILE=large INGESTION_RATE=100 QUERY_RATE=50 DURATION=30m ./k6 run examples/combined-test.js
//
// =============================================================================
