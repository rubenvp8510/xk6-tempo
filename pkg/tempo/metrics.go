package tempo

import (
	"context"
	"time"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

var (
	// Ingestion metrics
	IngestionBytesTotal = metrics.NewRegistry().MustNewMetric(
		"tempo_ingestion_bytes_total",
		metrics.Counter,
		metrics.Data,
	)
	
	IngestionRateBytesPerSec = metrics.NewRegistry().MustNewMetric(
		"tempo_ingestion_rate_bytes_per_sec",
		metrics.Gauge,
		metrics.Data,
	)
	
	IngestionTracesTotal = metrics.NewRegistry().MustNewMetric(
		"tempo_ingestion_traces_total",
		metrics.Counter,
		metrics.Default,
	)
	
	IngestionDuration = metrics.NewRegistry().MustNewMetric(
		"tempo_ingestion_duration_seconds",
		metrics.Trend,
		metrics.Time,
	)
	
	// Query metrics
	QueryDuration = metrics.NewRegistry().MustNewMetric(
		"tempo_query_duration_seconds",
		metrics.Trend,
		metrics.Time,
	)
	
	QueryRequestsTotal = metrics.NewRegistry().MustNewMetric(
		"tempo_query_requests_total",
		metrics.Counter,
		metrics.Default,
	)
	
	QueryFailuresTotal = metrics.NewRegistry().MustNewMetric(
		"tempo_query_failures_total",
		metrics.Counter,
		metrics.Default,
	)
	
	QuerySpansReturned = metrics.NewRegistry().MustNewMetric(
		"tempo_query_spans_returned",
		metrics.Trend,
		metrics.Default,
	)
	
	// Enhanced query metrics
	QueryFailuresByStatus = metrics.NewRegistry().MustNewMetric(
		"tempo_query_failures_by_status",
		metrics.Counter,
		metrics.Default,
	)
	
	QueryBackoffEvents = metrics.NewRegistry().MustNewMetric(
		"tempo_query_backoff_events_total",
		metrics.Counter,
		metrics.Default,
	)
	
	QueryBackoffDuration = metrics.NewRegistry().MustNewMetric(
		"tempo_query_backoff_duration_seconds",
		metrics.Trend,
		metrics.Time,
	)
	
	TraceFetchLatency = metrics.NewRegistry().MustNewMetric(
		"tempo_trace_fetch_latency_seconds",
		metrics.Trend,
		metrics.Time,
	)
	
	TraceFetchFailures = metrics.NewRegistry().MustNewMetric(
		"tempo_trace_fetch_failures_total",
		metrics.Counter,
		metrics.Default,
	)
	
	QueryTimeBucketQueries = metrics.NewRegistry().MustNewMetric(
		"tempo_query_time_bucket_queries_total",
		metrics.Counter,
		metrics.Default,
	)
	
	QueryTimeBucketDuration = metrics.NewRegistry().MustNewMetric(
		"tempo_query_time_bucket_duration_seconds",
		metrics.Trend,
		metrics.Time,
	)
)

// TestContext holds test identification information for metric tagging
type TestContext struct {
	TestName   string
	TargetQPS  int
	TargetMBps float64
}

// RecordIngestion records ingestion metrics
func RecordIngestion(state *lib.State, bytes int64, traces int, duration time.Duration) {
	RecordIngestionWithContext(state, nil, bytes, traces, duration)
}

// RecordIngestionWithContext records ingestion metrics with test context tags
func RecordIngestionWithContext(state *lib.State, testCtx *TestContext, bytes int64, traces int, duration time.Duration) {
	if state == nil || state.Samples == nil {
		return
	}
	
	now := time.Now()
	ctx := context.Background()
	
	// k6 automatically includes tags from options.tags in metrics
	// Tags from the k6 script's options.tags (test_name, target_qps, target_mbps)
	// will be automatically included by k6's metrics system
	// We don't need to manually set Tags here - k6 handles it via state
	
	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		Time: now,
		TimeSeries: metrics.TimeSeries{
			Metric: IngestionBytesTotal,
		},
		Value: float64(bytes),
	})
	
	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		Time: now,
		TimeSeries: metrics.TimeSeries{
			Metric: IngestionTracesTotal,
		},
		Value: float64(traces),
	})
	
	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		Time: now,
		TimeSeries: metrics.TimeSeries{
			Metric: IngestionDuration,
		},
		Value: metrics.D(duration),
	})
	
	// Calculate rate (bytes per second)
	if duration.Seconds() > 0 {
		rate := float64(bytes) / duration.Seconds()
		metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
			Time: now,
			TimeSeries: metrics.TimeSeries{
				Metric: IngestionRateBytesPerSec,
			},
			Value: rate,
		})
	}
}

// RecordQuery records query metrics
func RecordQuery(state *lib.State, duration time.Duration, spans int, success bool) {
	RecordQueryDetailed(state, duration, spans, success, "", 0)
}

// RecordQueryDetailed records query metrics with additional context
func RecordQueryDetailed(state *lib.State, duration time.Duration, spans int, success bool, queryName string, statusCode int) {
	if state == nil || state.Samples == nil {
		return
	}
	
	now := time.Now()
	ctx := context.Background()
	
	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		Time: now,
		TimeSeries: metrics.TimeSeries{
			Metric: QueryDuration,
		},
		Value: metrics.D(duration),
	})
	
	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		Time: now,
		TimeSeries: metrics.TimeSeries{
			Metric: QueryRequestsTotal,
		},
		Value: 1,
	})
	
	if !success {
		metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
			Time: now,
			TimeSeries: metrics.TimeSeries{
				Metric: QueryFailuresTotal,
			},
			Value: 1,
		})
		
		if statusCode > 0 {
			metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
				Time: now,
				TimeSeries: metrics.TimeSeries{
					Metric: QueryFailuresByStatus,
				},
				Value: float64(statusCode),
			})
		}
	}
	
	if spans > 0 {
		metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
			Time: now,
			TimeSeries: metrics.TimeSeries{
				Metric: QuerySpansReturned,
			},
			Value: float64(spans),
		})
	}
}

// RecordBackoff records backoff events
func RecordBackoff(state *lib.State, duration time.Duration) {
	if state == nil || state.Samples == nil {
		return
	}
	
	now := time.Now()
	ctx := context.Background()
	
	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		Time: now,
		TimeSeries: metrics.TimeSeries{
			Metric: QueryBackoffEvents,
		},
		Value: 1,
	})
	
	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		Time: now,
		TimeSeries: metrics.TimeSeries{
			Metric: QueryBackoffDuration,
		},
		Value: metrics.D(duration),
	})
}

// MetricsState wraps lib.State for trace fetch
type MetricsState struct {
	State *lib.State
}

// RecordTraceFetch records trace fetch metrics
func RecordTraceFetch(metricsState *MetricsState, duration time.Duration, success bool) {
	if metricsState == nil || metricsState.State == nil || metricsState.State.Samples == nil {
		return
	}
	
	now := time.Now()
	state := metricsState.State
	ctx := context.Background()
	
	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		Time: now,
		TimeSeries: metrics.TimeSeries{
			Metric: TraceFetchLatency,
		},
		Value: metrics.D(duration),
	})
	
	if !success {
		metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
			Time: now,
			TimeSeries: metrics.TimeSeries{
				Metric: TraceFetchFailures,
			},
			Value: 1,
		})
	}
}

// RecordTimeBucketQuery records time bucket query metrics
func RecordTimeBucketQuery(state *lib.State, bucketName string, duration time.Duration) {
	if state == nil || state.Samples == nil {
		return
	}
	
	now := time.Now()
	ctx := context.Background()
	
	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		Time: now,
		TimeSeries: metrics.TimeSeries{
			Metric: QueryTimeBucketQueries,
		},
		Value: 1,
	})
	
	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		Time: now,
		TimeSeries: metrics.TimeSeries{
			Metric: QueryTimeBucketDuration,
		},
		Value: metrics.D(duration),
	})
}

