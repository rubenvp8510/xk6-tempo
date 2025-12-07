import tempo from 'k6/x/tempo';

export const options = {
  scenarios: {
    queries: {
      executor: 'constant-arrival-rate',
      rate: 50,  // Target 50 QPS total (workload will distribute per VU)
      duration: '5m',
      preAllocatedVUs: 20,
      maxVUs: 100,
    },
  },
  thresholds: {
    'tempo_query_duration_seconds': ['p(95)<2'],
    'tempo_query_requests_total': ['rate>0'],
    'tempo_query_failures_total': ['rate<1'],
    'tempo_query_backoff_events_total': ['rate<5'], // Backoff should be rare
  },
};

const client = new tempo.Client({
  endpoint: __ENV.TEMPO_ENDPOINT || 'http://localhost:3200',
  protocol: __ENV.TEMPO_PROTOCOL || 'otlp-http',
  tenant: __ENV.TEMPO_TENANT || '',
  timeout: 30,
});

// Define query workload configuration
const workloadConfig = {
  targetQPS: 50,           // Target QPS (will be distributed across VUs)
  burstMultiplier: 2.0,     // Allow bursts up to 2x target QPS
  qpsMultiplier: 1.0,       // QPS multiplier (for compensation)
  enableBackoff: true,      // Enable adaptive backoff on 429/5xx
  minBackoffMs: 200,        // Minimum backoff: 200ms
  maxBackoffMs: 30000,      // Maximum backoff: 30s
  backoffJitter: true,      // Add jitter to backoff
  traceFetchProbability: 0.1, // 10% chance to fetch full trace after search
  timeWindowJitterMs: 1000,  // Add 1s jitter to time windows to defeat caching
  
  // Time buckets for query distribution
  timeBuckets: [
    {
      name: 'recent',
      ageStart: '0m',
      ageEnd: '1h',
      weight: 0.5,  // 50% of queries
    },
    {
      name: 'medium',
      ageStart: '1h',
      ageEnd: '6h',
      weight: 0.3,  // 30% of queries
    },
    {
      name: 'old',
      ageStart: '6h',
      ageEnd: '24h',
      weight: 0.2,  // 20% of queries
    },
  ],
  
  // Execution plan - defines which queries to run with which time buckets
  executionPlan: [
    { queryName: 'frontend', bucketName: 'recent', weight: 0.4 },
    { queryName: 'backend', bucketName: 'recent', weight: 0.3 },
    { queryName: 'errors', bucketName: 'medium', weight: 0.2 },
    { queryName: 'slow', bucketName: 'old', weight: 0.1 },
  ],
};

// Define query definitions
const queries = {
  frontend: {
    query: '{service.name="frontend"}',
    limit: 20,
  },
  backend: {
    query: '{service.name="backend"}',
    limit: 20,
  },
  errors: {
    query: '{status=error}',
    limit: 50,
  },
  slow: {
    query: '{duration>100ms}',
    limit: 30,
  },
};

// Initialize workload
let workload = null;

export function setup() {
  // Create workload (called once per VU)
  workload = client.createQueryWorkload(workloadConfig, queries);
  if (!workload) {
    throw new Error('Failed to create query workload');
  }
}

export default function() {
  // Execute search and fetch workflow
  // This will:
  // 1. Apply rate limiting (per-VU QPS)
  // 2. Select query and time bucket from execution plan (weighted random)
  // 3. Apply time window jitter if configured
  // 4. Execute search query
  // 5. Probabilistically fetch full trace (based on traceFetchProbability)
  // 6. Handle backoff on 429/5xx responses
  const err = client.executeWorkloadSearchAndFetch();
  if (err) {
    console.error('Workload query failed:', err);
  }
}

