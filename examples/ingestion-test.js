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
      useSemanticAttributes: true,
      useWorkflows: true,  // Enable workflow-based generation
      workflowWeights: {    // Distribution of workflows
        'place_order': 0.3,
        'user_login': 0.2,
        'browse_products': 0.2,
        'search_products': 0.15,
        'view_dashboard': 0.1,
        'process_refund': 0.05
      },
      businessAttributesDensity: 0.8,  // 80% of spans get business attributes
      enableTags: true,  // Enable tag generation
      tagDensity: 0.9,   // 90% of spans get tags
      cardinalityConfig: {  // Optional cardinality overrides
        'customer_id': 5000,
        'tenant_id': 50,
        'region': 5
      }
    }
  });
  
  const err = client.pushBatch(batch);
  if (err) {
    console.error('Failed to push batch:', err);
  }
}

