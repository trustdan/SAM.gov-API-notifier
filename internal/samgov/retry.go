package samgov

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"
)

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxRetries      int           `json:"max_retries"`
	InitialDelay    time.Duration `json:"initial_delay"`
	MaxDelay        time.Duration `json:"max_delay"`
	BackoffFactor   float64       `json:"backoff_factor"`
	Jitter          bool          `json:"jitter"`
	RetryableErrors []int         `json:"retryable_errors"`
}

// DefaultRetryConfig returns sensible defaults for SAM.gov API
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:    3,
		InitialDelay:  1 * time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
		RetryableErrors: []int{429, 500, 502, 503, 504}, // Rate limit and server errors
	}
}

// RetryClient wraps the SAM.gov client with retry logic
type RetryClient struct {
	*Client
	config  RetryConfig
	verbose bool
}

// NewRetryClient creates a client with retry capabilities
func NewRetryClient(apiKey string, config RetryConfig, verbose bool) *RetryClient {
	return &RetryClient{
		Client:  NewClient(apiKey),
		config:  config,
		verbose: verbose,
	}
}

// NewRetryClientWithDefaults creates a retry client with default configuration
func NewRetryClientWithDefaults(apiKey string, verbose bool) *RetryClient {
	return NewRetryClient(apiKey, DefaultRetryConfig(), verbose)
}

// SearchWithRetry executes a search with automatic retry on failure
func (rc *RetryClient) SearchWithRetry(ctx context.Context, params map[string]string) (*SearchResponse, error) {
	var lastErr error
	
	for attempt := 0; attempt <= rc.config.MaxRetries; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if rc.verbose && attempt > 0 {
			log.Printf("Retry attempt %d/%d for search", attempt, rc.config.MaxRetries)
		}

		// Execute the search
		result, err := rc.Client.Search(ctx, params)
		if err == nil {
			if rc.verbose && attempt > 0 {
				log.Printf("Search succeeded on attempt %d", attempt+1)
			}
			return result, nil
		}

		lastErr = err

		// Check if this error is retryable
		if !rc.isRetryableError(err) {
			if rc.verbose {
				log.Printf("Non-retryable error, failing immediately: %v", err)
			}
			return nil, err
		}

		// Don't wait after the last attempt
		if attempt == rc.config.MaxRetries {
			break
		}

		// Calculate delay for next attempt
		delay := rc.calculateDelay(attempt)
		
		if rc.verbose {
			log.Printf("Search failed (attempt %d/%d), retrying in %v: %v", 
				attempt+1, rc.config.MaxRetries+1, delay, err)
		}

		// Wait before retry
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	return nil, fmt.Errorf("search failed after %d attempts: %w", rc.config.MaxRetries+1, lastErr)
}

// SearchWithDefaultsAndRetry combines default parameters with retry logic
func (rc *RetryClient) SearchWithDefaultsAndRetry(ctx context.Context, customParams map[string]string, lookbackDays int) (*SearchResponse, error) {
	params := make(map[string]string)

	// Set default date range
	to := time.Now()
	from := to.AddDate(0, 0, -lookbackDays)
	params["postedFrom"] = from.Format("01/02/2006")
	params["postedTo"] = to.Format("01/02/2006")

	// Set default pagination
	params["limit"] = "100"
	params["offset"] = "0"

	// Merge custom parameters
	for key, value := range customParams {
		params[key] = value
	}

	return rc.SearchWithRetry(ctx, params)
}

// isRetryableError determines if an error should trigger a retry
func (rc *RetryClient) isRetryableError(err error) bool {
	// Check if it's an API error with retryable status code
	if apiErr, ok := err.(*APIError); ok {
		for _, code := range rc.config.RetryableErrors {
			if apiErr.StatusCode == code {
				return true
			}
		}
		return false
	}

	// Check for network-related errors that are typically retryable
	errorMsg := err.Error()
	retryablePatterns := []string{
		"timeout",
		"connection refused",
		"connection reset",
		"network",
		"temporary failure",
		"i/o timeout",
		"context deadline exceeded",
	}

	for _, pattern := range retryablePatterns {
		if containsIgnoreCase(errorMsg, pattern) {
			return true
		}
	}

	return false
}

// calculateDelay computes the delay before the next retry attempt
func (rc *RetryClient) calculateDelay(attempt int) time.Duration {
	// Exponential backoff: delay = initial_delay * (backoff_factor ^ attempt)
	delay := float64(rc.config.InitialDelay) * math.Pow(rc.config.BackoffFactor, float64(attempt))
	
	// Cap at maximum delay
	if delay > float64(rc.config.MaxDelay) {
		delay = float64(rc.config.MaxDelay)
	}

	duration := time.Duration(delay)

	// Add jitter to avoid thundering herd
	if rc.config.Jitter {
		jitter := time.Duration(float64(duration) * 0.1 * (2.0*mathRand() - 1.0)) // ±10%
		duration += jitter
		
		// Ensure non-negative
		if duration < 0 {
			duration = rc.config.InitialDelay
		}
	}

	return duration
}

// ValidateAPIKeyWithRetry checks API key with retry logic
func (rc *RetryClient) ValidateAPIKeyWithRetry(ctx context.Context) error {
	params := map[string]string{
		"limit":      "1",
		"postedFrom": time.Now().AddDate(0, 0, -1).Format("01/02/2006"),
		"postedTo":   time.Now().Format("01/02/2006"),
	}

	_, err := rc.SearchWithRetry(ctx, params)
	return err
}

// RetryStats tracks retry statistics
type RetryStats struct {
	TotalRequests    int           `json:"total_requests"`
	SuccessfulRequests int         `json:"successful_requests"`
	FailedRequests   int           `json:"failed_requests"`
	TotalRetries     int           `json:"total_retries"`
	AverageRetries   float64       `json:"average_retries"`
	RetryReasons     map[string]int `json:"retry_reasons"`
}

// StatsTrackingRetryClient wraps RetryClient with statistics tracking
type StatsTrackingRetryClient struct {
	*RetryClient
	stats RetryStats
}

// NewStatsTrackingRetryClient creates a retry client that tracks statistics
func NewStatsTrackingRetryClient(apiKey string, config RetryConfig, verbose bool) *StatsTrackingRetryClient {
	return &StatsTrackingRetryClient{
		RetryClient: NewRetryClient(apiKey, config, verbose),
		stats: RetryStats{
			RetryReasons: make(map[string]int),
		},
	}
}

// SearchWithRetryAndStats executes search with retry and tracks statistics
func (src *StatsTrackingRetryClient) SearchWithRetryAndStats(ctx context.Context, params map[string]string) (*SearchResponse, error) {
	src.stats.TotalRequests++
	attemptCount := 0
	
	var lastErr error
	
	for attempt := 0; attempt <= src.config.MaxRetries; attempt++ {
		attemptCount++
		
		select {
		case <-ctx.Done():
			src.stats.FailedRequests++
			return nil, ctx.Err()
		default:
		}

		result, err := src.Client.Search(ctx, params)
		if err == nil {
			src.stats.SuccessfulRequests++
			if attempt > 0 {
				src.stats.TotalRetries += attempt
			}
			return result, nil
		}

		lastErr = err

		// Track retry reason
		if apiErr, ok := err.(*APIError); ok {
			reason := fmt.Sprintf("HTTP_%d", apiErr.StatusCode)
			src.stats.RetryReasons[reason]++
		} else {
			src.stats.RetryReasons["network_error"]++
		}

		if !src.isRetryableError(err) || attempt == src.config.MaxRetries {
			break
		}

		delay := src.calculateDelay(attempt)
		select {
		case <-ctx.Done():
			src.stats.FailedRequests++
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	src.stats.FailedRequests++
	src.stats.TotalRetries += attemptCount - 1
	return nil, fmt.Errorf("search failed after %d attempts: %w", attemptCount, lastErr)
}

// GetStats returns current retry statistics
func (src *StatsTrackingRetryClient) GetStats() RetryStats {
	stats := src.stats
	if stats.TotalRequests > 0 {
		stats.AverageRetries = float64(stats.TotalRetries) / float64(stats.TotalRequests)
	}
	return stats
}

// ResetStats clears retry statistics
func (src *StatsTrackingRetryClient) ResetStats() {
	src.stats = RetryStats{
		RetryReasons: make(map[string]int),
	}
}

// mathRand returns a random float between 0 and 1
// Simple implementation to avoid importing math/rand
func mathRand() float64 {
	// Use current time nanoseconds for pseudo-randomness
	// This is not cryptographically secure but sufficient for jitter
	ns := time.Now().UnixNano()
	return float64((ns%1000000)/1000000.0)
}

// containsIgnoreCase performs case-insensitive substring search
// Duplicated from client.go to avoid circular dependencies
func containsIgnoreCase(text, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(text) == 0 {
		return false
	}
	
	textLower := make([]rune, 0, len(text))
	for _, r := range text {
		if r >= 'A' && r <= 'Z' {
			textLower = append(textLower, r+32)
		} else {
			textLower = append(textLower, r)
		}
	}
	
	substrLower := make([]rune, 0, len(substr))
	for _, r := range substr {
		if r >= 'A' && r <= 'Z' {
			substrLower = append(substrLower, r+32)
		} else {
			substrLower = append(substrLower, r)
		}
	}
	
	textStr := string(textLower)
	substrStr := string(substrLower)
	
	for i := 0; i <= len(textStr)-len(substrStr); i++ {
		if textStr[i:i+len(substrStr)] == substrStr {
			return true
		}
	}
	
	return false
}

// CircuitBreaker implements a simple circuit breaker pattern
type CircuitBreaker struct {
	maxFailures   int
	resetTimeout  time.Duration
	failures      int
	lastFailTime  time.Time
	state         string // "closed", "open", "half-open"
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		state:        "closed",
	}
}

// Execute runs a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if cb.state == "open" {
		if time.Since(cb.lastFailTime) > cb.resetTimeout {
			cb.state = "half-open"
		} else {
			return fmt.Errorf("circuit breaker is open")
		}
	}

	err := fn()
	
	if err != nil {
		cb.failures++
		cb.lastFailTime = time.Now()
		
		if cb.failures >= cb.maxFailures {
			cb.state = "open"
		}
		return err
	}

	// Reset on success
	cb.failures = 0
	cb.state = "closed"
	return nil
}