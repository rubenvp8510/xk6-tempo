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

const client = new tempo.Client({
  endpoint: __ENV.TEMPO_ENDPOINT || 'http://localhost:4318',
  protocol: __ENV.TEMPO_PROTOCOL || 'otlp-http',
  tenant: __ENV.TEMPO_TENANT || '',
  timeout: 30,
});

export default function() {
  // Generate a batch targeting 10KB per iteration
  // At 100 iterations/sec, this gives us ~1MB/s
  const batch = tempo.generateBatch({
    targetSizeBytes: 10240,  // 10KB
    traceConfig: {
      services: 3,
      spansPerTrace: 15,
      spanDepth: 3,
      attributeCount: 5,
      attributeValueSize: 32,
    }
  });
  
  const err = client.pushBatch(batch);
  if (err) {
    console.error('Failed to push batch:', err);
  }
}

