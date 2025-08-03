// Package reverseproxy provides retry functionality for the reverse proxy module.
package reverseproxy

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"time"
)

// RetryPolicy defines the retry behavior for failing requests.
type RetryPolicy struct {
	// MaxRetries is the maximum number of retries to attempt.
	MaxRetries int
	// BaseDelay is the base delay between retries.
	BaseDelay time.Duration
	// MaxDelay is the maximum delay between retries.
	MaxDelay time.Duration
	// Jitter is the amount of randomness to add to the delay.
	Jitter float64
	// RetryableStatusCodes is a list of status codes that should trigger a retry.
	RetryableStatusCodes map[int]bool
	// Timeout is the timeout for each attempt.
	Timeout time.Duration
}

// DefaultRetryPolicy returns a default retry policy.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries: 3,
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   10 * time.Second,
		Jitter:     0.1,
		Timeout:    5 * time.Second,
		RetryableStatusCodes: map[int]bool{
			408: true, // Request Timeout
			429: true, // Too Many Requests
			500: true, // Internal Server Error
			502: true, // Bad Gateway
			503: true, // Service Unavailable
			504: true, // Gateway Timeout
		},
	}
}

// WithMaxRetries sets the maximum number of retries.
func (p RetryPolicy) WithMaxRetries(maxRetries int) RetryPolicy {
	p.MaxRetries = maxRetries
	return p
}

// WithBaseDelay sets the base delay.
func (p RetryPolicy) WithBaseDelay(baseDelay time.Duration) RetryPolicy {
	p.BaseDelay = baseDelay
	return p
}

// WithMaxDelay sets the maximum delay.
func (p RetryPolicy) WithMaxDelay(maxDelay time.Duration) RetryPolicy {
	p.MaxDelay = maxDelay
	return p
}

// WithJitter sets the jitter.
func (p RetryPolicy) WithJitter(jitter float64) RetryPolicy {
	p.Jitter = jitter
	return p
}

// WithTimeout sets the timeout for each attempt.
func (p RetryPolicy) WithTimeout(timeout time.Duration) RetryPolicy {
	p.Timeout = timeout
	return p
}

// WithRetryableStatusCodes sets the status codes that should trigger a retry.
func (p RetryPolicy) WithRetryableStatusCodes(codes ...int) RetryPolicy {
	p.RetryableStatusCodes = make(map[int]bool, len(codes))
	for _, code := range codes {
		p.RetryableStatusCodes[code] = true
	}
	return p
}

// ShouldRetry returns true if the status code should trigger a retry.
func (p RetryPolicy) ShouldRetry(statusCode int) bool {
	return p.RetryableStatusCodes[statusCode]
}

// CalculateBackoff calculates the backoff duration for a retry attempt.
func (p RetryPolicy) CalculateBackoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	// Calculate exponential backoff with jitter
	backoff := float64(p.BaseDelay) * math.Pow(2, float64(attempt))

	// Apply maximum delay limit
	if backoff > float64(p.MaxDelay) {
		backoff = float64(p.MaxDelay)
	}

	// Add jitter to prevent synchronized retries
	if p.Jitter > 0 {
		// Use crypto/rand for secure random number generation
		randomBig, err := rand.Int(rand.Reader, big.NewInt(1000000))
		if err != nil {
			// Fall back to no jitter if crypto/rand fails
			return time.Duration(backoff)
		}
		random := float64(randomBig.Int64()) / 1000000.0
		jitter := (random*2 - 1) * p.Jitter * backoff
		backoff += jitter
	}

	return time.Duration(backoff)
}

// RetryFunc represents a function that can be retried.
type RetryFunc func(ctx context.Context) (interface{}, int, error)

// RetryWithPolicy executes the given function with retries according to the policy.
func RetryWithPolicy(ctx context.Context, policy RetryPolicy, fn RetryFunc, metrics *MetricsCollector, backendID string) (interface{}, int, error) {
	var (
		result     interface{}
		statusCode int
		err        error
		attempt    int
	)

	startTime := time.Now()

	for attempt = 0; attempt <= policy.MaxRetries; attempt++ {
		// Create a context with timeout for this attempt
		attemptCtx, cancel := context.WithTimeout(ctx, policy.Timeout)

		// Record attempt start time for metrics
		attemptStart := time.Now()

		// Execute the function
		result, statusCode, err = fn(attemptCtx)

		// Record attempt in metrics if available
		if metrics != nil && attempt > 0 {
			metrics.RecordRequest(backendID+"_retry_"+strconv.Itoa(attempt), attemptStart, statusCode, err)
		}

		cancel()

		// If successful or context canceled, return immediately
		if err == nil || ctx.Err() != nil {
			break
		}

		// If this is a non-retryable status code, return immediately
		if statusCode > 0 && !policy.ShouldRetry(statusCode) {
			break
		}

		// If we've reached the maximum retries, break
		if attempt >= policy.MaxRetries {
			break
		}

		// Calculate backoff duration
		backoff := policy.CalculateBackoff(attempt)

		// Create a timer for backoff
		timer := time.NewTimer(backoff)

		// Wait for either the backoff timer or context cancellation
		select {
		case <-timer.C:
			// Continue with next attempt
		case <-ctx.Done():
			timer.Stop()
			return nil, statusCode, fmt.Errorf("request cancelled: %w", ctx.Err())
		}
	}

	// If we've exhausted retries and still have an error
	if err != nil && attempt > policy.MaxRetries {
		return nil, statusCode, ErrMaxRetriesReached
	}

	// Record total request time including all retries
	if metrics != nil && attempt > 0 {
		metrics.RecordRequest(backendID+"_with_retries", startTime, statusCode, err)
	}

	return result, statusCode, err
}
