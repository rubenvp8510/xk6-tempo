package tempo

import (
	"context"
	"time"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

// TestContext holds test identification information for metric tagging
type TestContext struct {
	TestName   string
	TargetQPS  int
	TargetMBps float64
}

// RecordIngestion records ingestion metrics
func RecordIngestion(state *lib.State, m *tempoMetrics, bytes int64, traces int, duration time.Duration) {
	RecordIngestionWithContext(state, m, nil, bytes, traces, duration)
}

// RecordIngestionWithContext records ingestion metrics with test context tags
func RecordIngestionWithContext(state *lib.State, m *tempoMetrics, testCtx *TestContext, bytes int64, traces int, duration time.Duration) {
	if state == nil || state.Samples == nil || m == nil {
		return
	}

	now := time.Now()
	ctx := context.Background()

	// Get tags from state
	// Tags must not be nil to avoid nil pointer dereference in k6 metrics system
	tags := state.Tags.GetCurrentValues().Tags

	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		Time: now,
		TimeSeries: metrics.TimeSeries{
			Metric: m.IngestionBytesTotal,
			Tags:   tags,
		},
		Value: float64(bytes),
	})

	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		Time: now,
		TimeSeries: metrics.TimeSeries{
			Metric: m.IngestionTracesTotal,
			Tags:   tags,
		},
		Value: float64(traces),
	})

	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		Time: now,
		TimeSeries: metrics.TimeSeries{
			Metric: m.IngestionDuration,
			Tags:   tags,
		},
		Value: metrics.D(duration),
	})

	// Calculate rate (bytes per second)
	if duration.Seconds() > 0 {
		rate := float64(bytes) / duration.Seconds()
		metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
			Time: now,
			TimeSeries: metrics.TimeSeries{
				Metric: m.IngestionRateBytesPerSec,
				Tags:   tags,
			},
			Value: rate,
		})
	}
}

// RecordQuery records query metrics
func RecordQuery(state *lib.State, m *tempoMetrics, duration time.Duration, spans int, success bool) {
	RecordQueryDetailed(state, m, duration, spans, success, "", 0)
}

// RecordQueryDetailed records query metrics with additional context
func RecordQueryDetailed(state *lib.State, m *tempoMetrics, duration time.Duration, spans int, success bool, queryName string, statusCode int) {
	if state == nil || state.Samples == nil || m == nil {
		return
	}

	now := time.Now()
	ctx := context.Background()

	// Get tags from state
	tags := state.Tags.GetCurrentValues().Tags

	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		Time: now,
		TimeSeries: metrics.TimeSeries{
			Metric: m.QueryDuration,
			Tags:   tags,
		},
		Value: metrics.D(duration),
	})

	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		Time: now,
		TimeSeries: metrics.TimeSeries{
			Metric: m.QueryRequestsTotal,
			Tags:   tags,
		},
		Value: 1,
	})

	if !success {
		metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
			Time: now,
			TimeSeries: metrics.TimeSeries{
				Metric: m.QueryFailuresTotal,
				Tags:   tags,
			},
			Value: 1,
		})

		if statusCode > 0 {
			metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
				Time: now,
				TimeSeries: metrics.TimeSeries{
					Metric: m.QueryFailuresByStatus,
					Tags:   tags,
				},
				Value: float64(statusCode),
			})
		}
	}

	if spans > 0 {
		metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
			Time: now,
			TimeSeries: metrics.TimeSeries{
				Metric: m.QuerySpansReturned,
				Tags:   tags,
			},
			Value: float64(spans),
		})
	}
}

// RecordBackoff records backoff events
func RecordBackoff(state *lib.State, m *tempoMetrics, duration time.Duration) {
	if state == nil || state.Samples == nil || m == nil {
		return
	}

	now := time.Now()
	ctx := context.Background()

	// Get tags from state
	tags := state.Tags.GetCurrentValues().Tags

	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		Time: now,
		TimeSeries: metrics.TimeSeries{
			Metric: m.QueryBackoffEvents,
			Tags:   tags,
		},
		Value: 1,
	})

	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		Time: now,
		TimeSeries: metrics.TimeSeries{
			Metric: m.QueryBackoffDuration,
			Tags:   tags,
		},
		Value: metrics.D(duration),
	})
}

// MetricsState wraps lib.State and metrics for trace fetch
type MetricsState struct {
	State   *lib.State
	Metrics *tempoMetrics
}

// RecordTraceFetch records trace fetch metrics
func RecordTraceFetch(metricsState *MetricsState, duration time.Duration, success bool) {
	if metricsState == nil || metricsState.State == nil || metricsState.State.Samples == nil || metricsState.Metrics == nil {
		return
	}

	now := time.Now()
	state := metricsState.State
	m := metricsState.Metrics
	ctx := context.Background()

	// Get tags from state
	tags := state.Tags.GetCurrentValues().Tags

	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		Time: now,
		TimeSeries: metrics.TimeSeries{
			Metric: m.TraceFetchLatency,
			Tags:   tags,
		},
		Value: metrics.D(duration),
	})

	if !success {
		metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
			Time: now,
			TimeSeries: metrics.TimeSeries{
				Metric: m.TraceFetchFailures,
				Tags:   tags,
			},
			Value: 1,
		})
	}
}

// RecordTimeBucketQuery records time bucket query metrics
func RecordTimeBucketQuery(state *lib.State, m *tempoMetrics, bucketName string, duration time.Duration) {
	if state == nil || state.Samples == nil || m == nil {
		return
	}

	now := time.Now()
	ctx := context.Background()

	// Get tags from state
	tags := state.Tags.GetCurrentValues().Tags

	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		Time: now,
		TimeSeries: metrics.TimeSeries{
			Metric: m.QueryTimeBucketQueries,
			Tags:   tags,
		},
		Value: 1,
	})

	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		Time: now,
		TimeSeries: metrics.TimeSeries{
			Metric: m.QueryTimeBucketDuration,
			Tags:   tags,
		},
		Value: metrics.D(duration),
	})
}
