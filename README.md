# xk6-tempo: k6 Extension for Tempo Performance Testing

A k6 extension for performance testing Grafana Tempo, supporting both trace ingestion (measured in MB/s) and query load testing (measured in QPS). This extension enables you to determine optimal resource allocation for your Tempo deployment.

## Features

- **Trace Ingestion Testing**: Generate and push traces via OTLP (HTTP/gRPC), measured in MB/s
- **Query Performance Testing**: Execute TraceQL queries, measured in QPS (Queries Per Second)
- **Advanced Query Workloads**: 
  - Per-VU rate limiting with burst support
  - Adaptive backoff on 429/5xx responses with Retry-After support
  - Time bucket distribution for realistic query patterns
  - Weighted execution plans for query variety
  - Automatic search→fetch workflow with configurable probability
  - Time window jitter to defeat caching
- **Configurable Trace Generation**: Control trace size, depth, attributes, and services
- **Dual Protocol Support**: OTLP HTTP and gRPC protocols
- **Built-in Metrics**: Automatic collection of ingestion and query metrics including per-bucket and backoff metrics
- **Multi-tenancy Support**: Configure tenant headers for multi-tenant Tempo deployments

## Installation

### Prerequisites

- Go 1.21 or later
- [xk6](https://github.com/grafana/xk6) installed: `go install go.k6.io/xk6/cmd/xk6@latest`

### Build

```bash
# Clone the repository
git clone https://github.com/rvargasp/xk6-tempo.git
cd xk6-tempo

# Build custom k6 binary
make build
# or manually:
xk6 build --with github.com/rvargasp/xk6-tempo=.
```

This creates a custom `k6` binary in the current directory with the xk6-tempo extension included.

## Usage

### Basic Example: Ingestion Test

```javascript
import tempo from 'k6/x/tempo';

export const options = {
  scenarios: {
    ingestion: {
      executor: 'constant-arrival-rate',
      rate: 100,  // 100 iterations/sec
      duration: '5m',
      preAllocatedVUs: 10,
    },
  },
};

const client = new tempo.Client({
  endpoint: 'http://localhost:4318',
  protocol: 'otlp-http',
});

export default function() {
  const batch = tempo.generateBatch({
    targetSizeBytes: 10240,  // 10KB per iteration = ~1MB/s at 100 iter/s
    traceConfig: {
      services: 3,
      spansPerTrace: 15,
    }
  });
  
  client.pushBatch(batch);
}
```

### Basic Example: Query Test

```javascript
import tempo from 'k6/x/tempo';

export const options = {
  scenarios: {
    queries: {
      executor: 'constant-arrival-rate',
      rate: 50,  // 50 QPS
      duration: '5m',
      preAllocatedVUs: 20,
    },
  },
};

const client = new tempo.Client({
  endpoint: 'http://localhost:3200',
});

export default function() {
  const result = client.search('{service.name="frontend"}', {
    start: '1h',
    end: 'now',
    limit: 20,
  });
  
  if (result && result.traces.length > 0) {
    const traceId = result.traces[0].traceID;
    const fullTrace = client.getTrace(traceId);
  }
}
```

### Advanced Example: Query Workload with Rate Limiting and Time Buckets

For sophisticated query load testing with rate limiting, adaptive backoff, time bucket distribution, and automatic search→fetch workflows, use the query workload feature:

```javascript
import tempo from 'k6/x/tempo';

export const options = {
  scenarios: {
    queries: {
      executor: 'constant-arrival-rate',
      rate: 50,  // Target 50 QPS total
      duration: '5m',
      preAllocatedVUs: 20,
      maxVUs: 100,
    },
  },
};

const client = new tempo.Client({
  endpoint: __ENV.TEMPO_ENDPOINT || 'http://localhost:3200',
});

const workloadConfig = {
  targetQPS: 50,
  burstMultiplier: 2.0,
  enableBackoff: true,
  minBackoffMs: 200,
  maxBackoffMs: 30000,
  traceFetchProbability: 0.1,
  timeBuckets: [
    { name: 'recent', ageStart: '0m', ageEnd: '1h', weight: 0.5 },
    { name: 'medium', ageStart: '1h', ageEnd: '6h', weight: 0.3 },
    { name: 'old', ageStart: '6h', ageEnd: '24h', weight: 0.2 },
  ],
  executionPlan: [
    { queryName: 'frontend', bucketName: 'recent', weight: 0.4 },
    { queryName: 'errors', bucketName: 'medium', weight: 0.2 },
  ],
};

const queries = {
  frontend: { query: '{service.name="frontend"}', limit: 20 },
  errors: { query: '{status=error}', limit: 50 },
};

let workload = null;

export function setup() {
  workload = client.createQueryWorkload(workloadConfig, queries);
}

export default function() {
  // Executes with rate limiting, time bucket selection, and search→fetch
  client.executeWorkloadSearchAndFetch();
}
```

## API Reference

### `tempo.Client`

Creates a new Tempo client instance.

**Constructor Options:**
- `endpoint` (string, required): Tempo endpoint URL
- `protocol` (string, optional): `"otlp-http"` (default) or `"otlp-grpc"`
- `tenant` (string, optional): Tenant ID for multi-tenant deployments
- `timeout` (int, optional): Request timeout in seconds (default: 30)

**Methods:**

#### `client.push(trace)`
Pushes a single trace to Tempo.

**Parameters:**
- `trace` (ptrace.Traces): Trace generated by `tempo.generateTrace()`

**Returns:** Error if push fails

#### `client.pushBatch(traces)`
Pushes a batch of traces to Tempo (more efficient for high throughput).

**Parameters:**
- `traces` (array): Array of traces generated by `tempo.generateBatch()`

**Returns:** Error if push fails

#### `client.search(query, options)`
Performs a TraceQL search query.

**Parameters:**
- `query` (string): TraceQL query string (e.g., `'{service.name="frontend"}'`)
- `options` (object):
  - `start` (string): Start time (relative like `"1h"` or absolute timestamp)
  - `end` (string): End time (default: `"now"`)
  - `limit` (int): Maximum number of results

**Returns:** SearchResponse object with traces and metrics

#### `client.getTrace(traceID)`
Retrieves a full trace by trace ID.

**Parameters:**
- `traceID` (string): Trace ID to retrieve

**Returns:** Trace object with full span details

#### `client.createQueryWorkload(workloadConfig, queries)`
Creates a query workload manager with advanced features for realistic query load testing.

**Parameters:**
- `workloadConfig` (object): Workload configuration
  - `targetQPS` (float): Target queries per second (distributed across VUs)
  - `burstMultiplier` (float, default: 2.0): Burst multiplier for rate limiter
  - `qpsMultiplier` (float, default: 1.0): QPS multiplier for compensation
  - `enableBackoff` (bool, default: true): Enable adaptive backoff on 429/5xx
  - `minBackoffMs` (int, default: 200): Minimum backoff duration in milliseconds
  - `maxBackoffMs` (int, default: 30000): Maximum backoff duration in milliseconds
  - `backoffJitter` (bool, default: true): Add jitter to backoff delays
  - `traceFetchProbability` (float, default: 0.1): Probability of fetching full trace after search (0.0-1.0)
  - `timeWindowJitterMs` (int, default: 0): Jitter to add to time windows in milliseconds
  - `timeBuckets` (array): Time bucket configurations
    - `name` (string): Bucket identifier
    - `ageStart` (string): Start age (e.g., "1h", "30m")
    - `ageEnd` (string): End age (e.g., "2h", "1h")
    - `weight` (float, default: 1.0): Weight for selection
  - `executionPlan` (array): Execution plan entries
    - `queryName` (string): Name of query to execute
    - `bucketName` (string): Name of time bucket to use
    - `weight` (float, default: 1.0): Weight for selection

- `queries` (object): Query definitions map
  - Key: Query name (string)
  - Value: Query definition object
    - `query` (string): TraceQL query string
    - `limit` (int, default: 20): Maximum number of results
    - `options` (object, optional): Additional options

**Returns:** QueryWorkload object (or null on error)

#### `client.executeWorkloadQuery()`
Executes the next query from the workload execution plan with rate limiting and time bucket selection.

**Returns:** SearchResponse object or error

#### `client.executeWorkloadSearchAndFetch()`
Executes search and fetch workflow: performs search query, then probabilistically fetches full trace details.

**Returns:** Error if execution fails

### `tempo.generateTrace(config)`

Generates a single trace with configurable properties.

**Configuration Options:**
- `services` (int, default: 3): Number of distinct services
- `spanDepth` (int, default: 3): Maximum span tree depth
- `spansPerTrace` (int, default: 10): Total spans per trace
- `attributeCount` (int, default: 5): Number of attributes per span
- `attributeValueSize` (int, default: 32): Size of attribute values in bytes
- `eventCount` (int, default: 0): Number of events/logs per span
- `resourceAttributes` (object, default: {}): Resource-level attributes

**Returns:** ptrace.Traces object

### `tempo.generateBatch(config)`

Generates a batch of traces targeting a specific size in bytes.

**Configuration Options:**
- `targetSizeBytes` (int, required): Target batch size in bytes
- `traceConfig` (object): Same options as `generateTrace()`

**Returns:** Array of ptrace.Traces objects

## Metrics

The extension automatically exposes the following k6 metrics:

### Ingestion Metrics

- `tempo_ingestion_bytes_total` (Counter): Total bytes ingested
- `tempo_ingestion_rate_bytes_per_sec` (Gauge): Current ingestion rate in bytes/second
- `tempo_ingestion_traces_total` (Counter): Total traces ingested
- `tempo_ingestion_duration_seconds` (Trend): Ingestion latency

### Query Metrics

- `tempo_query_duration_seconds` (Trend): Query latency
- `tempo_query_requests_total` (Counter): Total queries executed
- `tempo_query_failures_total` (Counter): Failed queries
- `tempo_query_spans_returned` (Trend): Number of spans returned
- `tempo_query_failures_by_status` (Counter): Failed queries by HTTP status code
- `tempo_query_backoff_events_total` (Counter): Total backoff events triggered
- `tempo_query_backoff_duration_seconds` (Trend): Backoff duration
- `tempo_trace_fetch_latency_seconds` (Trend): Trace fetch latency
- `tempo_trace_fetch_failures_total` (Counter): Trace fetch failures
- `tempo_query_time_bucket_queries_total` (Counter): Queries per time bucket
- `tempo_query_time_bucket_duration_seconds` (Trend): Duration per time bucket

## Examples

See the `examples/` directory for complete test scripts:

- `ingestion-test.js`: Ingestion performance test targeting MB/s
- `query-test.js`: Simple query performance test targeting QPS
- `query-workload-test.js`: Advanced query workload test with rate limiting, time buckets, and backoff
- `combined-test.js`: Mixed workload with both ingestion and queries

## Running Tests

```bash
# Run ingestion test
./k6 run --env TEMPO_ENDPOINT=http://localhost:4318 examples/ingestion-test.js

# Run simple query test
./k6 run --env TEMPO_ENDPOINT=http://localhost:3200 examples/query-test.js

# Run advanced query workload test
./k6 run --env TEMPO_ENDPOINT=http://localhost:3200 examples/query-workload-test.js

# Run combined test
./k6 run --env TEMPO_ENDPOINT=http://localhost:4318 examples/combined-test.js
```

## Configuration

### Tempo Endpoints

- **OTLP HTTP**: Default port `4318` (e.g., `http://localhost:4318`)
- **OTLP gRPC**: Default port `4317` (e.g., `http://localhost:4317`)
- **Query API**: Default port `3200` (e.g., `http://localhost:3200`)

### Environment Variables

- `TEMPO_ENDPOINT`: Tempo endpoint URL
- `TEMPO_PROTOCOL`: Protocol to use (`otlp-http` or `otlp-grpc`)
- `TEMPO_TENANT`: Tenant ID for multi-tenant deployments

## Resource Planning

Use the metrics collected by this extension to:

1. **Determine Ingestion Capacity**: Monitor `tempo_ingestion_rate_bytes_per_sec` to find maximum sustainable MB/s
2. **Determine Query Capacity**: Monitor `tempo_query_duration_seconds` and `tempo_query_requests_total` to find maximum sustainable QPS
3. **Identify Bottlenecks**: Correlate metrics with Tempo resource utilization (CPU, memory, I/O)
4. **Plan Scaling**: Use test results to determine required resources for production workloads

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.

## License

This project is licensed under the Apache 2.0 License.

## References

- [k6 Documentation](https://grafana.com/docs/k6/latest/)
- [xk6 Extension Development](https://grafana.com/docs/k6/latest/extensions/create/)
- [Grafana Tempo Documentation](https://grafana.com/docs/tempo/latest/)
- [OpenTelemetry OTLP](https://opentelemetry.io/docs/specs/otlp/)

