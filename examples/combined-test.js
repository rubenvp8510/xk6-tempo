import tempo from 'k6/x/tempo';

export const options = {
  scenarios: {
    ingestion: {
      executor: 'constant-arrival-rate',
      rate: 50,  // 50 iterations/sec for ingestion
      duration: '5m',
      preAllocatedVUs: 5,
      maxVUs: 20,
    },
    queries: {
      executor: 'constant-arrival-rate',
      rate: 25,  // 25 QPS for queries
      duration: '5m',
      preAllocatedVUs: 10,
      maxVUs: 50,
    },
  },
  thresholds: {
    'tempo_ingestion_rate_bytes_per_sec': ['rate>0'],
    'tempo_query_duration_seconds': ['p(95)<2'],
    'tempo_query_failures_total': ['rate<1'],
  },
};

const ingestionClient = tempo.IngestClient({
  endpoint: 'tempo-simplest:4317',
  protocol: 'otlp-grpc',
  tenant: 'tenant-1',
  timeout: 30,
});

const queryClient = tempo.QueryClient({
  endpoint: 'https://tempo-simplest-gateway:8080',
  tenant: 'tenant-1',
  timeout: 30,
});

const queries = [
  '{service.name="frontend"}',
  '{service.name="backend"}',
  '{status=error}',
];

export default function() {
  // Determine which scenario this VU is running
  const scenario = __ENV.__SCENARIO || 'ingestion';
  
  if (scenario === 'ingestion') {
    // Ingestion workload
    const batch = tempo.generateBatch({
      targetSizeBytes: 10240,  // 10KB per batch
      traceConfig: {
        services: 3,
        spansPerTrace: 15,
        spanDepth: 3,
        attributeCount: 5,
        attributeValueSize: 32,
      }
    });
    
    const err = ingestionClient.pushBatch(batch);
    if (err) {
      console.error('Failed to push batch:', err);
    }
  } else {
    // Query workload
    const query = queries[Math.floor(Math.random() * queries.length)];
    const result = queryClient.search(query, {
      start: '1h',
      end: 'now',
      limit: 20,
    });
    
    if (!result) {
      console.error('Query failed');
    }
  }
}

