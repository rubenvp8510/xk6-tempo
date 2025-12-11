package tempo

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// CreateQueryWorkload creates a query workload manager
func CreateQueryWorkload(queryClient *QueryClient, vu VU, m *tempoMetrics, workloadConfig map[string]interface{}, queries map[string]interface{}) (*QueryWorkload, error) {
	// Parse workload config
	cfg := DefaultQueryWorkloadConfig()
	if targetQPS, ok := workloadConfig["targetQPS"].(float64); ok {
		cfg.TargetQPS = targetQPS
	}
	if burstMult, ok := workloadConfig["burstMultiplier"].(float64); ok {
		cfg.BurstMultiplier = burstMult
	}
	if qpsMult, ok := workloadConfig["qpsMultiplier"].(float64); ok {
		cfg.QPSMultiplier = qpsMult
	}
	if enableBackoff, ok := workloadConfig["enableBackoff"].(bool); ok {
		cfg.EnableBackoff = enableBackoff
	}
	if minBackoff, ok := workloadConfig["minBackoffMs"].(int); ok {
		cfg.MinBackoffMs = minBackoff
	}
	if maxBackoff, ok := workloadConfig["maxBackoffMs"].(int); ok {
		cfg.MaxBackoffMs = maxBackoff
	}
	if backoffJitter, ok := workloadConfig["backoffJitter"].(bool); ok {
		cfg.BackoffJitter = backoffJitter
	}
	if traceFetchProb, ok := workloadConfig["traceFetchProbability"].(float64); ok {
		cfg.TraceFetchProbability = traceFetchProb
	}
	if timeWindowJitter, ok := workloadConfig["timeWindowJitterMs"].(int); ok {
		cfg.TimeWindowJitterMs = timeWindowJitter
	}

	// Parse time buckets
	if timeBuckets, ok := workloadConfig["timeBuckets"].([]interface{}); ok {
		cfg.TimeBuckets = make([]TimeBucketConfig, 0, len(timeBuckets))
		for _, tb := range timeBuckets {
			if tbMap, ok := tb.(map[string]interface{}); ok {
				bucket := TimeBucketConfig{
					Weight: 1.0,
				}
				if name, ok := tbMap["name"].(string); ok {
					bucket.Name = name
				}
				if ageStart, ok := tbMap["ageStart"].(string); ok {
					bucket.AgeStart = ageStart
				}
				if ageEnd, ok := tbMap["ageEnd"].(string); ok {
					bucket.AgeEnd = ageEnd
				}
				if weight, ok := tbMap["weight"].(float64); ok {
					bucket.Weight = weight
				}
				cfg.TimeBuckets = append(cfg.TimeBuckets, bucket)
			}
		}
	}

	// Parse execution plan
	if executionPlan, ok := workloadConfig["executionPlan"].([]interface{}); ok {
		cfg.ExecutionPlan = make([]PlanEntry, 0, len(executionPlan))
		for _, ep := range executionPlan {
			if epMap, ok := ep.(map[string]interface{}); ok {
				entry := PlanEntry{
					Weight: 1.0,
				}
				if queryName, ok := epMap["queryName"].(string); ok {
					entry.QueryName = queryName
				}
				if bucketName, ok := epMap["bucketName"].(string); ok {
					entry.BucketName = bucketName
				}
				if weight, ok := epMap["weight"].(float64); ok {
					entry.Weight = weight
				}
				cfg.ExecutionPlan = append(cfg.ExecutionPlan, entry)
			}
		}
	}

	// Parse query definitions
	queryDefs := make(map[string]QueryDefinition)
	for name, q := range queries {
		if qMap, ok := q.(map[string]interface{}); ok {
			def := QueryDefinition{
				Name:  name,
				Limit: 20,
			}
			if query, ok := qMap["query"].(string); ok {
				def.Query = query
			}
			if limit, ok := qMap["limit"].(int); ok {
				def.Limit = limit
			}
			if options, ok := qMap["options"].(map[string]interface{}); ok {
				def.Options = options
			}
			queryDefs[name] = def
		}
	}

	// Create state wrapper
	workloadState := &WorkloadState{
		VU: vu,
	}

	// Create workload
	workload := NewQueryWorkload(cfg, queryClient, workloadState, queryDefs, m)

	return workload, nil
}

// QueryWorkload manages query execution with rate limiting, backoff, and execution plans
type QueryWorkload struct {
	config          QueryWorkloadConfig
	queryClient     *QueryClient
	state           *WorkloadState
	queries         map[string]QueryDefinition
	rateLimiter     *rate.Limiter
	backoffDuration time.Duration
	backoffMutex    sync.Mutex
	testStartTime   time.Time
	planIndex       int
	planMutex       sync.Mutex
	metrics         *tempoMetrics
}

// WorkloadState holds k6 VU for metrics in workload
type WorkloadState struct {
	VU VU
}

// NewQueryWorkload creates a new query workload manager
func NewQueryWorkload(
	config QueryWorkloadConfig,
	queryClient *QueryClient,
	state *WorkloadState,
	queries map[string]QueryDefinition,
	m *tempoMetrics,
) *QueryWorkload {
	// Calculate per-VU QPS (k6 handles VU distribution, so we use target QPS directly)
	perVUQPS := config.TargetQPS * config.QPSMultiplier
	burstSize := int(perVUQPS * config.BurstMultiplier)
	if burstSize < 1 {
		burstSize = 1
	}

	limiter := rate.NewLimiter(rate.Limit(perVUQPS), burstSize)

	return &QueryWorkload{
		config:        config,
		queryClient:   queryClient,
		state:         state,
		queries:       queries,
		rateLimiter:   limiter,
		testStartTime: time.Now(),
		metrics:       m,
	}
}

// executeNext executes the next query from the execution plan (internal, requires context)
func (qw *QueryWorkload) executeNext(ctx context.Context) (*SearchResponse, error) {
	// Wait for rate limiter
	if err := qw.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter wait failed: %w", err)
	}

	// Apply backoff if needed
	qw.applyBackoff(ctx)

	// Select next plan entry
	planEntry := qw.selectPlanEntry()
	if planEntry == nil {
		return nil, fmt.Errorf("no eligible plan entry found")
	}

	// Get query definition
	queryDef, ok := qw.queries[planEntry.QueryName]
	if !ok {
		return nil, fmt.Errorf("query definition not found: %s", planEntry.QueryName)
	}

	// Get time bucket
	bucket, err := qw.getTimeBucket(planEntry.BucketName)
	if err != nil {
		return nil, fmt.Errorf("time bucket not found: %s: %w", planEntry.BucketName, err)
	}

	// Calculate time range
	elapsed := time.Since(qw.testStartTime)
	start, end, eligible, err := bucket.ParseTimeRanges(elapsed)
	if err != nil {
		return nil, fmt.Errorf("failed to parse time bucket: %w", err)
	}
	if !eligible {
		// Try to find an eligible bucket or use default
		return qw.executeWithDefaultTimeRange(ctx, &queryDef)
	}

	// Apply bidirectional jitter to shift the entire time window (defeat caching)
	if qw.config.TimeWindowJitterMs > 0 {
		// Random offset between -jitterMs and +jitterMs
		offset := time.Duration((rand.Float64()*2-1)*float64(qw.config.TimeWindowJitterMs)) * time.Millisecond
		start = start.Add(offset)
		end = end.Add(offset)
	}

	// Build query options
	options := QueryOptions{
		Start: fmt.Sprintf("%d", start.UnixNano()),
		End:   fmt.Sprintf("%d", end.UnixNano()),
		Limit: queryDef.Limit,
	}
	if options.Limit == 0 {
		options.Limit = 20
	}

	// Execute search with HTTP response info
	searchStart := time.Now()
	result, httpResp, err := qw.queryClient.searchWithHTTP(ctx, queryDef.Query, options)
	searchDuration := time.Since(searchStart)

	// Record metrics
	statusCode := 0
	if httpResp != nil {
		statusCode = httpResp.StatusCode
	}
	spans := 0
	if result != nil {
		spans = len(result.Traces)
	}
	if qw.state.VU.State() != nil {
		RecordQueryDetailed(qw.state.VU.State(), qw.metrics, searchDuration, spans, err == nil, planEntry.QueryName, statusCode)
		RecordTimeBucketQuery(qw.state.VU.State(), qw.metrics, planEntry.BucketName, searchDuration)
	}

	// Handle HTTP response for backoff
	oldBackoff := qw.backoffDuration
	if httpResp != nil {
		qw.HandleHTTPResponse(httpResp)
	} else if err != nil {
		// Error without HTTP response - reset backoff
		qw.backoffMutex.Lock()
		qw.backoffDuration = 0
		qw.backoffMutex.Unlock()
	} else {
		// Success - reset backoff
		qw.backoffMutex.Lock()
		qw.backoffDuration = 0
		qw.backoffMutex.Unlock()
	}

	// Record backoff if it changed
	if qw.config.EnableBackoff && qw.backoffDuration > oldBackoff && qw.state.VU.State() != nil {
		RecordBackoff(qw.state.VU.State(), qw.metrics, qw.backoffDuration-oldBackoff)
	}

	return result, err
}

// executeSearchAndFetch executes a search and optionally fetches the full trace (internal, requires context)
func (qw *QueryWorkload) executeSearchAndFetch(ctx context.Context) error {
	// Execute search
	result, err := qw.executeNext(ctx)
	if err != nil {
		return err
	}

	if result == nil || len(result.Traces) == 0 {
		return nil
	}

	// Probabilistically fetch trace
	if rand.Float64() < qw.config.TraceFetchProbability {
		traceID := result.Traces[0].TraceID
		fetchStart := time.Now()
		_, httpResp, fetchErr := qw.queryClient.getTraceWithHTTP(ctx, traceID)
		fetchDuration := time.Since(fetchStart)

		// Handle HTTP response for backoff
		if httpResp != nil {
			qw.HandleHTTPResponse(httpResp)
		}

		// Record trace fetch metrics
		metricsState := &MetricsState{
			State:   qw.state.VU.State(),
			Metrics: qw.metrics,
		}
		if fetchErr != nil {
			// Record fetch failure but don't fail the whole operation
			RecordTraceFetch(metricsState, fetchDuration, false)
		} else {
			RecordTraceFetch(metricsState, fetchDuration, true)
		}
	}

	return nil
}

// selectPlanEntry selects the next plan entry using weighted random selection
func (qw *QueryWorkload) selectPlanEntry() *PlanEntry {
	qw.planMutex.Lock()
	defer qw.planMutex.Unlock()

	if len(qw.config.ExecutionPlan) == 0 {
		return nil
	}

	// Calculate total weight
	totalWeight := 0.0
	for _, entry := range qw.config.ExecutionPlan {
		weight := entry.Weight
		if weight <= 0 {
			weight = 1.0
		}
		totalWeight += weight
	}

	if totalWeight == 0 {
		// Fallback to cycling
		entry := &qw.config.ExecutionPlan[qw.planIndex%len(qw.config.ExecutionPlan)]
		qw.planIndex++
		return entry
	}

	// Weighted random selection
	r := rand.Float64() * totalWeight
	currentWeight := 0.0
	for i := range qw.config.ExecutionPlan {
		weight := qw.config.ExecutionPlan[i].Weight
		if weight <= 0 {
			weight = 1.0
		}
		currentWeight += weight
		if r <= currentWeight {
			return &qw.config.ExecutionPlan[i]
		}
	}

	// Fallback to first entry
	return &qw.config.ExecutionPlan[0]
}

// getTimeBucket retrieves a time bucket by name
func (qw *QueryWorkload) getTimeBucket(name string) (*TimeBucketConfig, error) {
	for i := range qw.config.TimeBuckets {
		if qw.config.TimeBuckets[i].Name == name {
			return &qw.config.TimeBuckets[i], nil
		}
	}
	return nil, fmt.Errorf("time bucket not found: %s", name)
}

// executeWithDefaultTimeRange executes a query with default time range
func (qw *QueryWorkload) executeWithDefaultTimeRange(ctx context.Context, queryDef *QueryDefinition) (*SearchResponse, error) {
	options := QueryOptions{
		Start: "1h",
		End:   "now",
		Limit: queryDef.Limit,
	}
	if options.Limit == 0 {
		options.Limit = 20
	}

	searchStart := time.Now()
	result, httpResp, err := qw.queryClient.searchWithHTTP(ctx, queryDef.Query, options)
	searchDuration := time.Since(searchStart)

	// Record metrics
	statusCode := 0
	if httpResp != nil {
		statusCode = httpResp.StatusCode
	}
	spans := 0
	if result != nil {
		spans = len(result.Traces)
	}
	if qw.state.VU.State() != nil {
		RecordQueryDetailed(qw.state.VU.State(), qw.metrics, searchDuration, spans, err == nil, queryDef.Name, statusCode)
	}

	if httpResp != nil {
		qw.HandleHTTPResponse(httpResp)
	} else if err != nil {
		qw.backoffMutex.Lock()
		qw.backoffDuration = 0
		qw.backoffMutex.Unlock()
	} else {
		qw.backoffMutex.Lock()
		qw.backoffDuration = 0
		qw.backoffMutex.Unlock()
	}
	return result, err
}

// applyBackoff applies backoff delay if needed
func (qw *QueryWorkload) applyBackoff(ctx context.Context) {
	if !qw.config.EnableBackoff {
		return
	}

	qw.backoffMutex.Lock()
	defer qw.backoffMutex.Unlock()

	if qw.backoffDuration > 0 {
		// Add jitter if configured
		delay := qw.backoffDuration
		if qw.config.BackoffJitter {
			jitter := time.Duration(rand.Intn(int(delay.Milliseconds()/10))) * time.Millisecond
			delay += jitter
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
			// Backoff complete
		}
	}
}

// HandleHTTPResponse processes HTTP response and updates backoff based on status
func (qw *QueryWorkload) HandleHTTPResponse(resp *http.Response) {
	if !qw.config.EnableBackoff {
		return
	}

	qw.backoffMutex.Lock()
	defer qw.backoffMutex.Unlock()

	// Check for overload signals
	if resp.StatusCode == 429 || (resp.StatusCode >= 500 && resp.StatusCode < 600) {
		// Check for Retry-After header
		retryAfter := resp.Header.Get("Retry-After")
		if retryAfter != "" {
			if seconds, err := strconv.Atoi(retryAfter); err == nil {
				qw.backoffDuration = time.Duration(seconds) * time.Second
				if qw.backoffDuration > time.Duration(qw.config.MaxBackoffMs)*time.Millisecond {
					qw.backoffDuration = time.Duration(qw.config.MaxBackoffMs) * time.Millisecond
				}
				return
			}
		}

		// Exponential backoff
		if qw.backoffDuration == 0 {
			qw.backoffDuration = time.Duration(qw.config.MinBackoffMs) * time.Millisecond
		} else {
			qw.backoffDuration = time.Duration(float64(qw.backoffDuration) * 1.5)
			if qw.backoffDuration > time.Duration(qw.config.MaxBackoffMs)*time.Millisecond {
				qw.backoffDuration = time.Duration(qw.config.MaxBackoffMs) * time.Millisecond
			}
		}
	} else {
		// Success - reset backoff
		qw.backoffDuration = 0
	}
}

// GetBackoffDuration returns the current backoff duration
func (qw *QueryWorkload) GetBackoffDuration() time.Duration {
	qw.backoffMutex.Lock()
	defer qw.backoffMutex.Unlock()
	return qw.backoffDuration
}

// SetQueries updates the query definitions
func (qw *QueryWorkload) SetQueries(queries map[string]QueryDefinition) {
	qw.queries = queries
}

// JavaScript-friendly wrapper methods (exported, no context parameter required)

// ExecuteNext executes the next query from the execution plan (JavaScript-friendly)
func (qw *QueryWorkload) ExecuteNext() (*SearchResponse, error) {
	ctx := context.Background()
	return qw.executeNext(ctx)
}

// ExecuteSearchAndFetch executes a search and optionally fetches the full trace (JavaScript-friendly)
func (qw *QueryWorkload) ExecuteSearchAndFetch() error {
	ctx := context.Background()
	return qw.executeSearchAndFetch(ctx)
}

// CalculatePerWorkerQPS calculates QPS per worker given total concurrency
func CalculatePerWorkerQPS(targetQPS float64, totalConcurrency int, qpsMultiplier float64) float64 {
	if totalConcurrency <= 0 {
		return targetQPS * qpsMultiplier
	}
	return (targetQPS * qpsMultiplier) / float64(totalConcurrency)
}

// CalculateBurstSize calculates burst size for rate limiter
func CalculateBurstSize(perWorkerQPS float64, burstMultiplier float64) int {
	burst := int(math.Ceil(perWorkerQPS * burstMultiplier))
	if burst < 1 {
		return 1
	}
	return burst
}
