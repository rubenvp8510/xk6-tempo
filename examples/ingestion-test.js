import tempo from 'k6/x/tempo';
import { TRACE_PROFILES } from './trace-profiles.js';

// =============================================================================
// CONFIGURATION - Change these values as needed
// =============================================================================

// Select trace profile: 'small' | 'medium' | 'large' | 'xlarge'
// 
// Profile Summary:
//   small:   8-15 spans,    5-8 services   - Startups, MVPs, simple APIs
//   medium:  25-40 spans,   15-20 services - E-commerce, SaaS típico (DEFAULT)
//   large:   50-80 spans,   25-35 services - Fintech, Enterprise
//   xlarge:  100-150 spans, 40-50 services - FAANG-scale, ultra-distributed
//
const PROFILE = __ENV.TRACE_PROFILE || 'medium';

// Test parameters (can be overridden via environment variables)
// Option 1: Use TARGET_BYTES_PER_SEC to automatically calculate trace rate (default: 1 MB/s = 1048576 bytes/s)
const TARGET_BYTES_PER_SEC = parseInt(__ENV.TARGET_BYTES_PER_SEC || '1048576');
// Option 2: Use fixed RATE (ignored if TARGET_BYTES_PER_SEC > 0)
const RATE = parseInt(__ENV.RATE || '100');           // iterations/sec (fallback if TARGET_BYTES_PER_SEC = 0)
const DURATION = __ENV.DURATION || '5m';              // test duration
const PRE_ALLOCATED_VUS = parseInt(__ENV.VUS || '10');
const MAX_VUS = parseInt(__ENV.MAX_VUS || '50');

// Tempo endpoint configuration
const ENDPOINT = __ENV.TEMPO_ENDPOINT || 'tempo-simplest:4317';
const PROTOCOL = __ENV.TEMPO_PROTOCOL || 'otlp-grpc';
const TENANT = __ENV.TEMPO_TENANT || 'tenant-1';

// =============================================================================
// TRACE CONFIGURATION & SIZE ESTIMATION
// =============================================================================

// Get selected profile
const traceProfile = TRACE_PROFILES[PROFILE];
if (!traceProfile) {
  throw new Error(`Invalid TRACE_PROFILE: ${PROFILE}. Valid options: small, medium, large, xlarge`);
}

// Build trace configuration for estimation
const traceConfig = {
  useTraceTree: true,
  traceTree: traceProfile
};

// Estimate trace size (in bytes)
const estimatedSizeB = tempo.estimateTraceSize(traceConfig);
const estimatedSizeKB = (estimatedSizeB / 1024).toFixed(2);

// Calculate throughput
let finalRate = RATE;
let throughputInfo = '';
if (TARGET_BYTES_PER_SEC > 0) {
  const throughput = tempo.calculateThroughput(traceConfig, TARGET_BYTES_PER_SEC, PRE_ALLOCATED_VUS);
  finalRate = Math.ceil(throughput.totalTracesPerSec);
  const targetKBps = (TARGET_BYTES_PER_SEC / 1024).toFixed(2);
  throughputInfo = `
Target:          ${TARGET_BYTES_PER_SEC} bytes/s (${targetKBps} KB/s)
Traces/VU/sec:   ${throughput.tracesPerVU.toFixed(2)}
Total traces/sec: ${throughput.totalTracesPerSec.toFixed(2)}`;
} else {
  // If TARGET_BYTES_PER_SEC is 0, use fixed RATE
  throughputInfo = `
Using fixed rate: ${RATE} traces/sec`;
}

// =============================================================================
// K6 OPTIONS
// =============================================================================

export const options = {
  scenarios: {
    ingestion: {
      executor: 'constant-arrival-rate',
      rate: finalRate,
      duration: DURATION,
      preAllocatedVUs: PRE_ALLOCATED_VUS,
      maxVUs: MAX_VUS,
    },
  },
  thresholds: {
    'tempo_ingestion_rate_bytes_per_sec': ['rate>0'],
    'tempo_ingestion_traces_total': ['count>0'],
  },
};

// =============================================================================
// CLIENT SETUP
// =============================================================================

const client = tempo.IngestClient({
  endpoint: ENDPOINT,
  protocol: PROTOCOL,
  tenant: TENANT,
  timeout: 30,
});

// Log configuration on startup
console.log(`
================================================================================
xk6-tempo Ingestion Test
================================================================================
Profile:         ${PROFILE}
Estimated Size:  ${estimatedSizeB} bytes (${estimatedSizeKB} KB)
Rate:            ${finalRate} traces/sec${TARGET_BYTES_PER_SEC > 0 ? ' (calculated from target bytes/s)' : ''}
Duration:        ${DURATION}
VUs:             ${PRE_ALLOCATED_VUS} (max: ${MAX_VUS})
Endpoint:        ${ENDPOINT}
Protocol:        ${PROTOCOL}
Tenant:          ${TENANT}${throughputInfo}
================================================================================
`);

// =============================================================================
// TEST EXECUTION
// =============================================================================

export default function() {
  // Generate trace using the selected profile
  const trace = tempo.generateTrace({
    useTraceTree: true,
    traceTree: traceProfile
  });
  
  const err = client.push(trace);
  if (err) {
    console.error('Failed to push trace:', err);
  }
}

// =============================================================================
// USAGE EXAMPLES
// =============================================================================
// 
// Basic usage (medium profile, default 1 MB/s):
//   ./k6 run examples/ingestion-test.js
//
// With specific profile:
//   TRACE_PROFILE=large ./k6 run examples/ingestion-test.js
//
// Custom throughput target (2 MB/s = 2097152 bytes/s):
//   TARGET_BYTES_PER_SEC=2097152 ./k6 run examples/ingestion-test.js
//
// High throughput (5 MB/s = 5242880 bytes/s):
//   TARGET_BYTES_PER_SEC=5242880 VUS=20 MAX_VUS=100 ./k6 run examples/ingestion-test.js
//
// Custom endpoint:
//   TEMPO_ENDPOINT=tempo.example.com:4317 TEMPO_TENANT=prod ./k6 run examples/ingestion-test.js
//
// =============================================================================
