import tempo from 'k6/x/tempo';

// All parameters from environment
const CONFIG = {
  name: __ENV.TEST_NAME || 'default',
  targetQPS: parseInt(__ENV.TARGET_QPS) || 100,
  targetMBps: parseFloat(__ENV.TARGET_MBPS) || 1.0,
  duration: __ENV.DURATION || '5m',
  vus: parseInt(__ENV.VUS) || 10,
};

export const options = {
  scenarios: {
    ingestion: {
      executor: 'constant-arrival-rate',
      rate: CONFIG.targetQPS,
      duration: CONFIG.duration,
      preAllocatedVUs: CONFIG.vus,
      maxVUs: CONFIG.vus * 5,
    },
  },
  tags: {
    test_name: CONFIG.name,
    target_qps: String(CONFIG.targetQPS),
    target_mbps: String(CONFIG.targetMBps),
  },
  thresholds: {
    'tempo_ingestion_duration_seconds': ['p(95)<1'],
    'tempo_ingestion_rate_bytes_per_sec': ['rate>0'],
  },
};

const client = tempo.IngestClient({
  endpoint: __ENV.TEMPO_ENDPOINT || 'http://localhost:4318',
  protocol: __ENV.TEMPO_PROTOCOL || 'otlp-http',
  tenant: __ENV.TEMPO_TENANT || '',
  timeout: 30,
  testName: CONFIG.name,
  targetQPS: CONFIG.targetQPS,
  targetMBps: CONFIG.targetMBps,
});

// Create rate limiter for MB/s control
const rateLimiter = tempo.createRateLimiter({
  targetMBps: CONFIG.targetMBps,
  burstMultiplier: 1.5,
});

export default function() {
  // Generate a batch with realistic trace configuration
  const batch = tempo.generateBatch({
    targetSizeBytes: Math.floor((CONFIG.targetMBps * 1024 * 1024) / CONFIG.targetQPS), // Size per iteration
    traceConfig: {
      services: parseInt(__ENV.SERVICES) || 3,
      spansPerTrace: parseInt(__ENV.SPANS_PER_TRACE) || 15,
      spanDepth: parseInt(__ENV.SPAN_DEPTH) || 4,
      attributeCount: parseInt(__ENV.ATTRIBUTE_COUNT) || 5,
      attributeValueSize: parseInt(__ENV.ATTRIBUTE_VALUE_SIZE) || 32,
      durationBaseMs: parseInt(__ENV.DURATION_BASE_MS) || 50,
      durationVarianceMs: parseInt(__ENV.DURATION_VARIANCE_MS) || 30,
      errorRate: parseFloat(__ENV.ERROR_RATE) || 0.02,
      maxFanOut: parseInt(__ENV.MAX_FAN_OUT) || 5,
      fanOutVariance: parseFloat(__ENV.FAN_OUT_VARIANCE) || 0.5,
      useSemanticAttributes: __ENV.USE_SEMANTIC_ATTRS !== 'false',
      spanKindWeights: {
        server: 0.35,
        client: 0.35,
        internal: 0.20,
        producer: 0.05,
        consumer: 0.05,
      },
    }
  });
  
  // Push with rate limiting
  const err = client.pushBatchWithRateLimit(batch, rateLimiter);
  if (err) {
    console.error('Failed to push batch:', err);
  }
}

