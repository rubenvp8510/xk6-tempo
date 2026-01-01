package tempo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// FlexInt handles JSON numbers that may be strings or integers
type FlexInt int

func (fi *FlexInt) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		if s == "" {
			*fi = 0
			return nil
		}
		i, err := strconv.Atoi(s)
		if err != nil {
			return err
		}
		*fi = FlexInt(i)
		return nil
	}
	var i int
	if err := json.Unmarshal(b, &i); err != nil {
		return err
	}
	*fi = FlexInt(i)
	return nil
}

// FlexInt64 handles JSON numbers that may be strings or int64
type FlexInt64 int64

func (fi *FlexInt64) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		if s == "" {
			*fi = 0
			return nil
		}
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		*fi = FlexInt64(i)
		return nil
	}
	var i int64
	if err := json.Unmarshal(b, &i); err != nil {
		return err
	}
	*fi = FlexInt64(i)
	return nil
}

// QueryOptions represents options for trace search queries
type QueryOptions struct {
	Start string `js:"start"` // Relative time like "1h", "30m", or absolute timestamp
	End   string `js:"end"`   // Relative time like "now" or absolute timestamp
	Limit int    `js:"limit"` // Maximum number of results
}

// SearchResult represents a single search result
type SearchResult struct {
	TraceID         string                 `json:"traceID"`
	RootServiceName string                 `json:"rootServiceName"`
	RootTraceName   string                 `json:"rootTraceName"`
	StartTime       FlexInt64              `json:"startTimeUnixNano"`
	DurationMs      FlexInt64              `json:"durationMs"`
	Tags            map[string]string      `json:"tags"`
	ServiceStats    map[string]interface{} `json:"serviceStats"`
}

// SearchResponse represents the response from Tempo search API
type SearchResponse struct {
	Traces  []SearchResult `json:"traces"`
	Metrics struct {
		InspectedTraces FlexInt `json:"inspectedTraces"`
		InspectedBytes  FlexInt `json:"inspectedBytes"`
		InspectedBlocks FlexInt `json:"inspectedBlocks"`
		TotalBlocks     FlexInt `json:"totalBlocks"`
	} `json:"metrics"`
}

// Trace represents a full trace retrieved by ID
type Trace struct {
	Batches []TraceBatch `json:"batches"`
}

// TraceBatch represents a batch of spans in a trace
type TraceBatch struct {
	Resource   map[string]interface{} `json:"resource"`
	ScopeSpans []ScopeSpan            `json:"scopeSpans"`
}

// ScopeSpan represents spans within a scope
type ScopeSpan struct {
	Scope map[string]interface{} `json:"scope"`
	Spans []Span                 `json:"spans"`
}

// Span represents a single span
type Span struct {
	TraceID      string                 `json:"traceId"`
	SpanID       string                 `json:"spanId"`
	ParentSpanID string                 `json:"parentSpanId"`
	Name         string                 `json:"name"`
	Kind         string                 `json:"kind"`
	StartTime    FlexInt64              `json:"startTimeUnixNano"`
	EndTime      FlexInt64              `json:"endTimeUnixNano"`
	Attributes   map[string]interface{} `json:"attributes"`
	Status       map[string]interface{} `json:"status"`
	Events       []interface{}          `json:"events"`
	Links        []interface{}          `json:"links"`
}

// QueryClient handles queries to Tempo's search API
type QueryClient struct {
	client      *http.Client
	baseURL     string
	tenant      string
	bearerToken string
}

// NewQueryClient creates a new query client
func NewQueryClient(config QueryConfig) (*QueryClient, error) {
	timeout := time.Duration(config.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Resolve bearer token
	bearerToken, err := ResolveBearerToken(config.BearerToken, config.BearerTokenFile)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve bearer token: %w", err)
	}

	// Ensure baseURL doesn't end with /
	baseURL := config.Endpoint
	if len(baseURL) > 0 && baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}

	return &QueryClient{
		client: &http.Client{
			Timeout: timeout,
		},
		baseURL:     baseURL,
		tenant:      config.Tenant,
		bearerToken: bearerToken,
	}, nil
}

// SearchResponseWithHTTP wraps SearchResponse with HTTP response info
type SearchResponseWithHTTP struct {
	*SearchResponse
	HTTPResponse *http.Response
}

// search performs a TraceQL search query (internal, requires context)
func (c *QueryClient) search(ctx context.Context, query string, options QueryOptions) (*SearchResponse, error) {
	result, _, err := c.searchWithHTTP(ctx, query, options)
	return result, err
}

// searchWithHTTP performs a TraceQL search query and returns HTTP response info (internal, requires context)
func (c *QueryClient) searchWithHTTP(ctx context.Context, query string, options QueryOptions) (*SearchResponse, *http.Response, error) {
	// Build URL
	apiURL := c.baseURL + "/api/search"

	// Parse query options
	params := url.Values{}
	params.Set("q", query)

	// Parse start time
	if options.Start != "" {
		startTime, err := parseTime(options.Start)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid start time: %w", err)
		}
		params.Set("start", strconv.FormatInt(startTime, 10))
	}

	// Parse end time
	if options.End != "" && options.End != "now" {
		endTime, err := parseTime(options.End)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid end time: %w", err)
		}
		params.Set("end", strconv.FormatInt(endTime, 10))
	}

	if options.Limit > 0 {
		params.Set("limit", strconv.Itoa(options.Limit))
	}

	fullURL := apiURL + "?" + params.Encode()

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set tenant header if configured
	if c.tenant != "" {
		req.Header.Set("X-Scope-OrgID", c.tenant)
	}

	// Set bearer token if configured
	if c.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	}

	// Send request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, resp, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		resp.Body.Close()
		return nil, resp, fmt.Errorf("failed to decode response: %w", err)
	}
	resp.Body.Close()

	return &searchResp, resp, nil
}

// getTrace retrieves a full trace by trace ID (internal, requires context)
func (c *QueryClient) getTrace(ctx context.Context, traceID string) (*Trace, error) {
	result, _, err := c.getTraceWithHTTP(ctx, traceID)
	return result, err
}

// getTraceWithHTTP retrieves a full trace by trace ID and returns HTTP response info (internal, requires context)
func (c *QueryClient) getTraceWithHTTP(ctx context.Context, traceID string) (*Trace, *http.Response, error) {
	// Build URL - Tempo API v2
	apiURL := fmt.Sprintf("%s/api/traces/%s", c.baseURL, traceID)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set tenant header if configured
	if c.tenant != "" {
		req.Header.Set("X-Scope-OrgID", c.tenant)
	}

	// Set bearer token if configured
	if c.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	}

	// Send request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, resp, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var trace Trace
	if err := json.NewDecoder(resp.Body).Decode(&trace); err != nil {
		resp.Body.Close()
		return nil, resp, fmt.Errorf("failed to decode response: %w", err)
	}
	resp.Body.Close()

	return &trace, resp, nil
}

// JavaScript-friendly wrapper methods (exported, no context parameter required)

// Search performs a TraceQL search query (JavaScript-friendly)
func (c *QueryClient) Search(query string, options QueryOptions) (*SearchResponse, error) {
	ctx := context.Background()
	return c.search(ctx, query, options)
}

// SearchWithHTTP performs a TraceQL search query and returns HTTP response info (JavaScript-friendly)
func (c *QueryClient) SearchWithHTTP(query string, options QueryOptions) (*SearchResponse, *http.Response, error) {
	ctx := context.Background()
	return c.searchWithHTTP(ctx, query, options)
}

// GetTrace retrieves a full trace by trace ID (JavaScript-friendly)
func (c *QueryClient) GetTrace(traceID string) (*Trace, error) {
	ctx := context.Background()
	return c.getTrace(ctx, traceID)
}

// GetTraceWithHTTP retrieves a full trace by trace ID and returns HTTP response info (JavaScript-friendly)
func (c *QueryClient) GetTraceWithHTTP(traceID string) (*Trace, *http.Response, error) {
	ctx := context.Background()
	return c.getTraceWithHTTP(ctx, traceID)
}

// parseTime parses a time string (relative like "1h" or absolute timestamp)
func parseTime(timeStr string) (int64, error) {
	// Try relative time first
	if duration, err := time.ParseDuration(timeStr); err == nil {
		return time.Now().Add(-duration).UnixNano(), nil
	}

	// Try absolute timestamp (nanoseconds)
	if timestamp, err := strconv.ParseInt(timeStr, 10, 64); err == nil {
		return timestamp, nil
	}

	// Try RFC3339 format
	if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
		return t.UnixNano(), nil
	}

	return 0, fmt.Errorf("unable to parse time: %s", timeStr)
}
