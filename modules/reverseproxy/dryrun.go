package reverseproxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/CrisisTextLine/modular"
)

// DryRunConfig provides configuration for dry-run functionality.
type DryRunConfig struct {
	// Enabled determines if dry-run mode is available
	Enabled bool `json:"enabled" yaml:"enabled" toml:"enabled" env:"DRY_RUN_ENABLED" default:"false"`

	// LogResponses determines if response bodies should be logged (can be verbose)
	LogResponses bool `json:"log_responses" yaml:"log_responses" toml:"log_responses" env:"DRY_RUN_LOG_RESPONSES" default:"false"`

	// MaxResponseSize is the maximum response size to compare (in bytes)
	MaxResponseSize int64 `json:"max_response_size" yaml:"max_response_size" toml:"max_response_size" env:"DRY_RUN_MAX_RESPONSE_SIZE" default:"1048576"` // 1MB

	// CompareHeaders determines which headers should be compared
	CompareHeaders []string `json:"compare_headers" yaml:"compare_headers" toml:"compare_headers" env:"DRY_RUN_COMPARE_HEADERS"`

	// IgnoreHeaders lists headers to ignore during comparison
	IgnoreHeaders []string `json:"ignore_headers" yaml:"ignore_headers" toml:"ignore_headers" env:"DRY_RUN_IGNORE_HEADERS"`

	// DefaultResponseBackend specifies which backend response to return by default ("primary" or "secondary")
	DefaultResponseBackend string `json:"default_response_backend" yaml:"default_response_backend" toml:"default_response_backend" env:"DRY_RUN_DEFAULT_RESPONSE_BACKEND" default:"primary"`
}

// DryRunResult represents the result of a dry-run comparison.
type DryRunResult struct {
	Timestamp         time.Time        `json:"timestamp"`
	RequestID         string           `json:"requestId,omitempty"`
	TenantID          string           `json:"tenantId,omitempty"`
	Endpoint          string           `json:"endpoint"`
	Method            string           `json:"method"`
	PrimaryBackend    string           `json:"primaryBackend"`
	SecondaryBackend  string           `json:"secondaryBackend"`
	PrimaryResponse   ResponseInfo     `json:"primaryResponse"`
	SecondaryResponse ResponseInfo     `json:"secondaryResponse"`
	Comparison        ComparisonResult `json:"comparison"`
	Duration          DurationInfo     `json:"duration"`
	ReturnedResponse  string           `json:"returnedResponse"` // "primary" or "secondary" - indicates which response was returned to client
}

// ResponseInfo contains information about a backend response.
type ResponseInfo struct {
	StatusCode   int               `json:"statusCode"`
	Headers      map[string]string `json:"headers,omitempty"`
	Body         string            `json:"body,omitempty"`
	BodySize     int64             `json:"bodySize"`
	ResponseTime time.Duration     `json:"responseTime"`
	Error        string            `json:"error,omitempty"`
}

// ComparisonResult contains the results of comparing two responses.
type ComparisonResult struct {
	StatusCodeMatch bool                  `json:"statusCodeMatch"`
	HeadersMatch    bool                  `json:"headersMatch"`
	BodyMatch       bool                  `json:"bodyMatch"`
	Differences     []string              `json:"differences,omitempty"`
	HeaderDiffs     map[string]HeaderDiff `json:"headerDiffs,omitempty"`
}

// HeaderDiff represents a difference in header values.
type HeaderDiff struct {
	Primary   string `json:"primary"`
	Secondary string `json:"secondary"`
}

// DurationInfo contains timing information for the dry-run.
type DurationInfo struct {
	Total     time.Duration `json:"total"`
	Primary   time.Duration `json:"primary"`
	Secondary time.Duration `json:"secondary"`
}

// DryRunHandler handles dry-run request processing.
type DryRunHandler struct {
	config         DryRunConfig
	tenantIDHeader string
	httpClient     *http.Client
	logger         modular.Logger
}

// NewDryRunHandler creates a new dry-run handler.
func NewDryRunHandler(config DryRunConfig, tenantIDHeader string, logger modular.Logger) *DryRunHandler {
	if tenantIDHeader == "" {
		tenantIDHeader = "X-Tenant-ID" // Default fallback
	}
	return &DryRunHandler{
		config:         config,
		tenantIDHeader: tenantIDHeader,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// ProcessDryRun processes a request in dry-run mode, sending it to both backends and comparing responses.
func (d *DryRunHandler) ProcessDryRun(ctx context.Context, req *http.Request, primaryBackend, secondaryBackend string) (*DryRunResult, error) {
	if !d.config.Enabled {
		return nil, ErrDryRunModeNotEnabled
	}

	startTime := time.Now()

	// Create dry-run result
	result := &DryRunResult{
		Timestamp:        startTime,
		RequestID:        req.Header.Get("X-Request-ID"),
		TenantID:         req.Header.Get(d.tenantIDHeader),
		Endpoint:         req.URL.Path,
		Method:           req.Method,
		PrimaryBackend:   primaryBackend,
		SecondaryBackend: secondaryBackend,
	}

	// Read and store request body for replication
	var requestBody []byte
	if req.Body != nil {
		var err error
		requestBody, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
		req.Body.Close()
	}

	// Send requests to both backends concurrently
	primaryChan := make(chan ResponseInfo, 1)
	secondaryChan := make(chan ResponseInfo, 1)

	// Send request to primary backend
	go func() {
		primaryStart := time.Now()
		response := d.sendRequest(ctx, req, primaryBackend, requestBody)
		response.ResponseTime = time.Since(primaryStart)
		primaryChan <- response
	}()

	// Send request to secondary backend
	go func() {
		secondaryStart := time.Now()
		response := d.sendRequest(ctx, req, secondaryBackend, requestBody)
		response.ResponseTime = time.Since(secondaryStart)
		secondaryChan <- response
	}()

	// Collect responses
	result.PrimaryResponse = <-primaryChan
	result.SecondaryResponse = <-secondaryChan

	// Calculate timing
	result.Duration = DurationInfo{
		Total:     time.Since(startTime),
		Primary:   result.PrimaryResponse.ResponseTime,
		Secondary: result.SecondaryResponse.ResponseTime,
	}

	// Determine which response to return based on configuration
	if d.config.DefaultResponseBackend == "secondary" {
		result.ReturnedResponse = "secondary"
	} else {
		result.ReturnedResponse = "primary" // Default to primary
	}

	// Compare responses
	result.Comparison = d.compareResponses(result.PrimaryResponse, result.SecondaryResponse)

	// Log the dry-run result
	d.logDryRunResult(result)

	return result, nil
}

// GetReturnedResponse returns the response information that should be sent to the client.
func (d *DryRunResult) GetReturnedResponse() ResponseInfo {
	if d.ReturnedResponse == "secondary" {
		return d.SecondaryResponse
	}
	return d.PrimaryResponse
}

// sendRequest sends a request to a specific backend and returns response information.
func (d *DryRunHandler) sendRequest(ctx context.Context, originalReq *http.Request, backend string, requestBody []byte) ResponseInfo {
	response := ResponseInfo{}

	// Create new request with proper URL joining
	url := singleJoiningSlash(backend, originalReq.URL.Path)
	if originalReq.URL.RawQuery != "" {
		url += "?" + originalReq.URL.RawQuery
	}

	var bodyReader io.Reader
	if len(requestBody) > 0 {
		bodyReader = bytes.NewReader(requestBody)
	}

	req, err := http.NewRequestWithContext(ctx, originalReq.Method, url, bodyReader)
	if err != nil {
		response.Error = fmt.Sprintf("failed to create request: %v", err)
		return response
	}

	// Copy headers
	for key, values := range originalReq.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Send request
	resp, err := d.httpClient.Do(req)
	if err != nil {
		response.Error = fmt.Sprintf("request failed: %v", err)
		return response
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("failed to close response body: %v\n", err)
		}
	}()

	response.StatusCode = resp.StatusCode

	// Read response body
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, d.config.MaxResponseSize))
	if err != nil {
		response.Error = fmt.Sprintf("failed to read response body: %v", err)
		return response
	}

	response.BodySize = int64(len(bodyBytes))
	if d.config.LogResponses {
		response.Body = string(bodyBytes)
	}

	// Copy response headers
	response.Headers = make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			response.Headers[key] = values[0] // Take first value
		}
	}

	return response
}

// compareResponses compares two responses and returns the comparison result.
func (d *DryRunHandler) compareResponses(primary, secondary ResponseInfo) ComparisonResult {
	result := ComparisonResult{
		Differences: []string{},
		HeaderDiffs: make(map[string]HeaderDiff),
	}

	// Compare status codes
	result.StatusCodeMatch = primary.StatusCode == secondary.StatusCode
	if !result.StatusCodeMatch {
		result.Differences = append(result.Differences,
			fmt.Sprintf("Status code: primary=%d, secondary=%d", primary.StatusCode, secondary.StatusCode))
	}

	// Compare headers
	result.HeadersMatch = d.compareHeaders(primary.Headers, secondary.Headers, result)

	// Compare response bodies
	result.BodyMatch = primary.Body == secondary.Body
	if !result.BodyMatch && primary.Body != "" && secondary.Body != "" {
		result.Differences = append(result.Differences, "Response body content differs")
	}

	// Check for errors
	if primary.Error != "" || secondary.Error != "" {
		if primary.Error != secondary.Error {
			result.Differences = append(result.Differences,
				fmt.Sprintf("Error: primary='%s', secondary='%s'", primary.Error, secondary.Error))
		}
	}

	return result
}

// compareHeaders compares headers between two responses.
func (d *DryRunHandler) compareHeaders(primaryHeaders, secondaryHeaders map[string]string, result ComparisonResult) bool {
	headersMatch := true
	ignoreMap := make(map[string]bool)

	// Build ignore map
	for _, header := range d.config.IgnoreHeaders {
		ignoreMap[header] = true
	}

	// Default headers to ignore
	ignoreMap["Date"] = true
	ignoreMap["X-Request-ID"] = true
	ignoreMap["X-Trace-ID"] = true

	// Compare headers that should be compared
	compareMap := make(map[string]bool)
	if len(d.config.CompareHeaders) > 0 {
		for _, header := range d.config.CompareHeaders {
			compareMap[header] = true
		}
	}

	// Check all headers in primary response
	for key, primaryValue := range primaryHeaders {
		if ignoreMap[key] {
			continue
		}

		// If compare headers are specified, only compare those
		if len(compareMap) > 0 && !compareMap[key] {
			continue
		}

		secondaryValue, exists := secondaryHeaders[key]
		if !exists {
			headersMatch = false
			result.HeaderDiffs[key] = HeaderDiff{
				Primary:   primaryValue,
				Secondary: "<missing>",
			}
		} else if primaryValue != secondaryValue {
			headersMatch = false
			result.HeaderDiffs[key] = HeaderDiff{
				Primary:   primaryValue,
				Secondary: secondaryValue,
			}
		}
	}

	// Check headers that exist in secondary but not in primary
	for key, secondaryValue := range secondaryHeaders {
		if ignoreMap[key] {
			continue
		}

		if len(compareMap) > 0 && !compareMap[key] {
			continue
		}

		if _, exists := primaryHeaders[key]; !exists {
			headersMatch = false
			result.HeaderDiffs[key] = HeaderDiff{
				Primary:   "<missing>",
				Secondary: secondaryValue,
			}
		}
	}

	return headersMatch
}

// logDryRunResult logs the dry-run result.
func (d *DryRunHandler) logDryRunResult(result *DryRunResult) {
	logLevel := "info"
	if len(result.Comparison.Differences) > 0 {
		logLevel = "warn"
	}

	logAttrs := []interface{}{
		"operation", "dry-run",
		"endpoint", result.Endpoint,
		"method", result.Method,
		"primaryBackend", result.PrimaryBackend,
		"secondaryBackend", result.SecondaryBackend,
		"statusCodeMatch", result.Comparison.StatusCodeMatch,
		"headersMatch", result.Comparison.HeadersMatch,
		"bodyMatch", result.Comparison.BodyMatch,
		"primaryStatus", result.PrimaryResponse.StatusCode,
		"secondaryStatus", result.SecondaryResponse.StatusCode,
		"primaryResponseTime", result.Duration.Primary,
		"secondaryResponseTime", result.Duration.Secondary,
		"totalDuration", result.Duration.Total,
	}

	if result.TenantID != "" {
		logAttrs = append(logAttrs, "tenant", result.TenantID)
	}

	if result.RequestID != "" {
		logAttrs = append(logAttrs, "requestId", result.RequestID)
	}

	if len(result.Comparison.Differences) > 0 {
		logAttrs = append(logAttrs, "differences", result.Comparison.Differences)
	}

	if len(result.Comparison.HeaderDiffs) > 0 {
		logAttrs = append(logAttrs, "headerDifferences", result.Comparison.HeaderDiffs)
	}

	if result.PrimaryResponse.Error != "" {
		logAttrs = append(logAttrs, "primaryError", result.PrimaryResponse.Error)
	}

	if result.SecondaryResponse.Error != "" {
		logAttrs = append(logAttrs, "secondaryError", result.SecondaryResponse.Error)
	}

	message := "Dry-run completed"
	if len(result.Comparison.Differences) > 0 {
		message = "Dry-run completed with differences"
	}

	switch logLevel {
	case "warn":
		d.logger.Warn(message, logAttrs...)
	default:
		d.logger.Info(message, logAttrs...)
	}
}
