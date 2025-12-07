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

// QueryOptions represents options for trace search queries
type QueryOptions struct {
	Start string `js:"start"` // Relative time like "1h", "30m", or absolute timestamp
	End   string `js:"end"`   // Relative time like "now" or absolute timestamp
	Limit int    `js:"limit"` // Maximum number of results
}

// SearchResult represents a single search result
type SearchResult struct {
	TraceID         string            `json:"traceID"`
	RootServiceName string            `json:"rootServiceName"`
	RootTraceName   string            `json:"rootTraceName"`
	StartTime       int64             `json:"startTimeUnixNano"`
	DurationMs      int64             `json:"durationMs"`
	Tags            map[string]string `json:"tags"`
	ServiceStats    map[string]int    `json:"serviceStats"`
}

// SearchResponse represents the response from Tempo search API
type SearchResponse struct {
	Traces  []SearchResult `json:"traces"`
	Metrics struct {
		InspectedTraces int `json:"inspectedTraces"`
		InspectedBytes  int `json:"inspectedBytes"`
		InspectedBlocks int `json:"inspectedBlocks"`
		TotalBlocks     int `json:"totalBlocks"`
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
	StartTime    int64                  `json:"startTimeUnixNano"`
	EndTime      int64                  `json:"endTimeUnixNano"`
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
func NewQueryClient(baseURL string, tenant string, bearerToken string, timeout time.Duration) *QueryClient {
	// Ensure baseURL doesn't end with /
	if len(baseURL) > 0 && baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}

	return &QueryClient{
		client: &http.Client{
			Timeout: timeout,
		},
		baseURL:     baseURL,
		tenant:      tenant,
		bearerToken: bearerToken,
	}
}

// SearchResponseWithHTTP wraps SearchResponse with HTTP response info
type SearchResponseWithHTTP struct {
	*SearchResponse
	HTTPResponse *http.Response
}

// Search performs a TraceQL search query
func (c *QueryClient) Search(ctx context.Context, query string, options QueryOptions) (*SearchResponse, error) {
	result, _, err := c.SearchWithHTTP(ctx, query, options)
	return result, err
}

// SearchWithHTTP performs a TraceQL search query and returns HTTP response info
func (c *QueryClient) SearchWithHTTP(ctx context.Context, query string, options QueryOptions) (*SearchResponse, *http.Response, error) {
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

// GetTrace retrieves a full trace by trace ID
func (c *QueryClient) GetTrace(ctx context.Context, traceID string) (*Trace, error) {
	result, _, err := c.GetTraceWithHTTP(ctx, traceID)
	return result, err
}

// GetTraceWithHTTP retrieves a full trace by trace ID and returns HTTP response info
func (c *QueryClient) GetTraceWithHTTP(ctx context.Context, traceID string) (*Trace, *http.Response, error) {
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
