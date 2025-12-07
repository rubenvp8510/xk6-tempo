import tempo from 'k6/x/tempo';

// Simple query test example - for advanced features see query-workload-test.js
export const options = {
  scenarios: {
    queries: {
      executor: 'constant-arrival-rate',
      rate: 50,  // 50 QPS
      duration: '5m',
      preAllocatedVUs: 20,
      maxVUs: 100,
    },
  },
  thresholds: {
    'tempo_query_duration_seconds': ['p(95)<2'],
    'tempo_query_requests_total': ['rate>0'],
    'tempo_query_failures_total': ['rate<1'],
  },
};

const client = new tempo.Client({
  endpoint: __ENV.TEMPO_ENDPOINT || 'http://localhost:3200',
  protocol: __ENV.TEMPO_PROTOCOL || 'otlp-http',
  tenant: __ENV.TEMPO_TENANT || '',
  timeout: 30,
});

const queries = [
  '{service.name="frontend"}',
  '{service.name="backend"}',
  '{status=error}',
  '{duration>100ms}',
  '{service.name="frontend" && status=error}',
];

export default function() {
  const query = queries[Math.floor(Math.random() * queries.length)];
  
  const result = client.search(query, {
    start: '1h',
    end: 'now',
    limit: 20,
  });
  
  if (!result) {
    console.error('Query failed');
    return;
  }
  
  // Optionally fetch full trace details for first result
  if (result.traces && result.traces.length > 0 && Math.random() < 0.1) {
    const traceId = result.traces[0].traceID;
    const fullTrace = client.getTrace(traceId);
    if (!fullTrace) {
      console.error('Failed to fetch trace:', traceId);
    }
  }
}

