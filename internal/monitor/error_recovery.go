package monitor

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/yourusername/sam-gov-monitor/internal/config"
	"github.com/yourusername/sam-gov-monitor/internal/samgov"
)

// ErrorType categorizes different types of errors
type ErrorType string

const (
	ErrorTypeNetwork       ErrorType = "network"
	ErrorTypeAPI          ErrorType = "api"
	ErrorTypeRateLimit    ErrorType = "rate_limit"
	ErrorTypeAuthentication ErrorType = "auth"
	ErrorTypeValidation   ErrorType = "validation"
	ErrorTypeTimeout      ErrorType = "timeout"
	ErrorTypeUnknown      ErrorType = "unknown"
)

// QueryResult represents the result of executing a single query
type QueryResult struct {
	Query         config.Query
	Opportunities []samgov.Opportunity
	Success       bool
	Error         error
	ErrorType     ErrorType
	Duration      time.Duration
	RetryCount    int
}

// PartialFailureHandler manages recovery from partial query failures
type PartialFailureHandler struct {
	verbose         bool
	maxRetries      int
	retryDelay      time.Duration
	failureThreshold float64 // Percentage of queries that can fail before stopping
}

// NewPartialFailureHandler creates a new error recovery handler
func NewPartialFailureHandler(verbose bool) *PartialFailureHandler {
	return &PartialFailureHandler{
		verbose:         verbose,
		maxRetries:      2,
		retryDelay:      30 * time.Second,
		failureThreshold: 0.5, // 50% of queries can fail
	}
}

// ExecuteQueriesWithRecovery executes queries with partial failure recovery
func (pfh *PartialFailureHandler) ExecuteQueriesWithRecovery(
	ctx context.Context,
	queries []config.Query,
	client *samgov.RetryClient,
	queryBuilder *QueryBuilder,
) ([]QueryResult, error) {
	results := make([]QueryResult, len(queries))
	
	// First pass: execute all queries
	pfh.executeQueriesParallel(ctx, queries, client, queryBuilder, results)
	
	// Analyze results and determine recovery strategy
	failedQueries := pfh.analyzeFailures(results)
	if len(failedQueries) == 0 {
		if pfh.verbose {
			log.Printf("All %d queries executed successfully", len(queries))
		}
		return results, nil
	}

	if pfh.verbose {
		log.Printf("Initial execution: %d succeeded, %d failed", 
			len(queries)-len(failedQueries), len(failedQueries))
	}

	// Check if failure rate is acceptable
	failureRate := float64(len(failedQueries)) / float64(len(queries))
	if failureRate > pfh.failureThreshold {
		return results, fmt.Errorf("failure rate %.1f%% exceeds threshold %.1f%%", 
			failureRate*100, pfh.failureThreshold*100)
	}

	// Retry failed queries with recovery strategies
	pfh.retryFailedQueries(ctx, failedQueries, client, queryBuilder, results)

	// Final analysis
	finalFailures := pfh.countFailures(results)
	if pfh.verbose {
		log.Printf("Final results: %d succeeded, %d failed", 
			len(queries)-finalFailures, finalFailures)
	}

	return results, nil
}

// executeQueriesParallel executes queries in parallel with controlled concurrency
func (pfh *PartialFailureHandler) executeQueriesParallel(
	ctx context.Context,
	queries []config.Query,
	client *samgov.RetryClient,
	queryBuilder *QueryBuilder,
	results []QueryResult,
) {
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 3) // Limit concurrent queries to avoid rate limits

	for i, query := range queries {
		if !query.Enabled {
			results[i] = QueryResult{
				Query:   query,
				Success: false,
				Error:   fmt.Errorf("query disabled"),
				ErrorType: ErrorTypeValidation,
			}
			continue
		}

		wg.Add(1)
		go func(index int, q config.Query) {
			defer wg.Done()
			
			// Acquire semaphore
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				results[index] = QueryResult{
					Query: q,
					Success: false,
					Error: ctx.Err(),
					ErrorType: ErrorTypeTimeout,
				}
				return
			}

			results[index] = pfh.executeQuery(ctx, q, client, queryBuilder)
		}(i, query)
	}

	wg.Wait()
}

// executeQuery executes a single query and returns the result
func (pfh *PartialFailureHandler) executeQuery(
	ctx context.Context,
	query config.Query,
	client *samgov.RetryClient,
	queryBuilder *QueryBuilder,
) QueryResult {
	start := time.Now()
	
	result := QueryResult{
		Query: query,
	}

	// Build query parameters
	params, err := queryBuilder.BuildParams(query)
	if err != nil {
		result.Error = fmt.Errorf("building parameters: %w", err)
		result.ErrorType = ErrorTypeValidation
		result.Duration = time.Since(start)
		return result
	}

	// Execute search with retry
	response, err := client.SearchWithRetry(ctx, params)
	result.Duration = time.Since(start)

	if err != nil {
		result.Error = err
		result.ErrorType = pfh.categorizeError(err)
		if pfh.verbose {
			log.Printf("Query '%s' failed: %v", query.Name, err)
		}
		return result
	}

	result.Success = true
	result.Opportunities = response.OpportunitiesData

	if pfh.verbose {
		log.Printf("Query '%s' succeeded: %d opportunities found in %v",
			query.Name, len(result.Opportunities), result.Duration)
	}

	return result
}

// analyzeFailures identifies failed queries and categorizes them
func (pfh *PartialFailureHandler) analyzeFailures(results []QueryResult) []int {
	var failedIndices []int
	errorCounts := make(map[ErrorType]int)

	for i, result := range results {
		if !result.Success {
			failedIndices = append(failedIndices, i)
			errorCounts[result.ErrorType]++
		}
	}

	if pfh.verbose && len(failedIndices) > 0 {
		log.Printf("Failure analysis:")
		for errorType, count := range errorCounts {
			log.Printf("  %s: %d", errorType, count)
		}
	}

	return failedIndices
}

// retryFailedQueries implements recovery strategies for failed queries
func (pfh *PartialFailureHandler) retryFailedQueries(
	ctx context.Context,
	failedIndices []int,
	client *samgov.RetryClient,
	queryBuilder *QueryBuilder,
	results []QueryResult,
) {
	if len(failedIndices) == 0 {
		return
	}

	if pfh.verbose {
		log.Printf("Attempting recovery for %d failed queries", len(failedIndices))
	}

	// Group failures by error type to apply appropriate recovery strategies
	errorGroups := make(map[ErrorType][]int)
	for _, index := range failedIndices {
		errorType := results[index].ErrorType
		errorGroups[errorType] = append(errorGroups[errorType], index)
	}

	// Apply recovery strategies based on error type
	for errorType, indices := range errorGroups {
		pfh.applyRecoveryStrategy(ctx, errorType, indices, client, queryBuilder, results)
	}
}

// applyRecoveryStrategy applies specific recovery based on error type
func (pfh *PartialFailureHandler) applyRecoveryStrategy(
	ctx context.Context,
	errorType ErrorType,
	indices []int,
	client *samgov.RetryClient,
	queryBuilder *QueryBuilder,
	results []QueryResult,
) {
	switch errorType {
	case ErrorTypeRateLimit:
		// For rate limit errors, wait longer and retry with spacing
		pfh.retryWithDelay(ctx, indices, client, queryBuilder, results, 
			2*time.Minute, 30*time.Second)

	case ErrorTypeNetwork, ErrorTypeTimeout:
		// For network issues, retry with shorter delay
		pfh.retryWithDelay(ctx, indices, client, queryBuilder, results,
			30*time.Second, 10*time.Second)

	case ErrorTypeAPI:
		// For API errors, try simplified queries
		pfh.retryWithSimplifiedQueries(ctx, indices, client, queryBuilder, results)

	case ErrorTypeAuthentication:
		// For auth errors, don't retry (will likely fail again)
		if pfh.verbose {
			log.Printf("Skipping retry for %d authentication errors", len(indices))
		}

	case ErrorTypeValidation:
		// For validation errors, try with fallback parameters
		pfh.retryWithFallbackParams(ctx, indices, client, queryBuilder, results)

	default:
		// For unknown errors, try simple retry
		pfh.retryWithDelay(ctx, indices, client, queryBuilder, results,
			pfh.retryDelay, 0)
	}
}

// retryWithDelay retries queries after waiting
func (pfh *PartialFailureHandler) retryWithDelay(
	ctx context.Context,
	indices []int,
	client *samgov.RetryClient,
	queryBuilder *QueryBuilder,
	results []QueryResult,
	initialDelay, spacingDelay time.Duration,
) {
	if initialDelay > 0 {
		select {
		case <-ctx.Done():
			return
		case <-time.After(initialDelay):
		}
	}

	for i, index := range indices {
		if spacingDelay > 0 && i > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(spacingDelay):
			}
		}

		query := results[index].Query
		if pfh.verbose {
			log.Printf("Retrying query '%s' (was %s)", query.Name, results[index].ErrorType)
		}

		retryResult := pfh.executeQuery(ctx, query, client, queryBuilder)
		retryResult.RetryCount = results[index].RetryCount + 1
		results[index] = retryResult
	}
}

// retryWithSimplifiedQueries tries simpler versions of failed queries
func (pfh *PartialFailureHandler) retryWithSimplifiedQueries(
	ctx context.Context,
	indices []int,
	client *samgov.RetryClient,
	queryBuilder *QueryBuilder,
	results []QueryResult,
) {
	for _, index := range indices {
		query := results[index].Query
		simplifiedQuery := pfh.simplifyQuery(query)
		
		if pfh.verbose {
			log.Printf("Retrying query '%s' with simplified parameters", query.Name)
		}

		retryResult := pfh.executeQuery(ctx, simplifiedQuery, client, queryBuilder)
		retryResult.RetryCount = results[index].RetryCount + 1
		results[index] = retryResult
	}
}

// retryWithFallbackParams tries queries with fallback parameters
func (pfh *PartialFailureHandler) retryWithFallbackParams(
	ctx context.Context,
	indices []int,
	client *samgov.RetryClient,
	queryBuilder *QueryBuilder,
	results []QueryResult,
) {
	for _, index := range indices {
		query := results[index].Query
		fallbackQuery := pfh.createFallbackQuery(query)
		
		if pfh.verbose {
			log.Printf("Retrying query '%s' with fallback parameters", query.Name)
		}

		retryResult := pfh.executeQuery(ctx, fallbackQuery, client, queryBuilder)
		retryResult.RetryCount = results[index].RetryCount + 1
		results[index] = retryResult
	}
}

// simplifyQuery creates a simplified version of a query
func (pfh *PartialFailureHandler) simplifyQuery(query config.Query) config.Query {
	simplified := query
	params := make(map[string]interface{})

	// Keep only essential parameters
	for key, value := range query.Parameters {
		switch key {
		case "title", "organizationName":
			params[key] = value
		case "ptype":
			// Simplify to just solicitations
			params[key] = "s"
		}
	}

	simplified.Parameters = params
	return simplified
}

// createFallbackQuery creates a fallback version of a query
func (pfh *PartialFailureHandler) createFallbackQuery(query config.Query) config.Query {
	fallback := query
	params := make(map[string]interface{})

	// Use very basic search parameters
	if title, exists := query.Parameters["title"]; exists {
		params["title"] = title
	} else if org, exists := query.Parameters["organizationName"]; exists {
		params["organizationName"] = org
	} else {
		// Last resort: search for any solicitations
		params["ptype"] = "s"
	}

	fallback.Parameters = params
	return fallback
}

// categorizeError determines the type of error
func (pfh *PartialFailureHandler) categorizeError(err error) ErrorType {
	if err == nil {
		return ErrorTypeUnknown
	}

	errorMsg := err.Error()

	// Check for API errors first
	if apiErr, ok := err.(*samgov.APIError); ok {
		switch apiErr.StatusCode {
		case 401, 403:
			return ErrorTypeAuthentication
		case 429:
			return ErrorTypeRateLimit
		case 400, 422:
			return ErrorTypeValidation
		case 500, 502, 503, 504:
			return ErrorTypeAPI
		default:
			return ErrorTypeAPI
		}
	}

	// Check error message patterns
	switch {
	case containsAny(errorMsg, []string{"timeout", "context deadline exceeded"}):
		return ErrorTypeTimeout
	case containsAny(errorMsg, []string{"network", "connection", "dns", "resolve"}):
		return ErrorTypeNetwork
	case containsAny(errorMsg, []string{"rate limit", "too many requests"}):
		return ErrorTypeRateLimit
	case containsAny(errorMsg, []string{"unauthorized", "forbidden", "authentication"}):
		return ErrorTypeAuthentication
	case containsAny(errorMsg, []string{"validation", "invalid", "bad request"}):
		return ErrorTypeValidation
	default:
		return ErrorTypeUnknown
	}
}

// countFailures counts the number of failed queries
func (pfh *PartialFailureHandler) countFailures(results []QueryResult) int {
	count := 0
	for _, result := range results {
		if !result.Success {
			count++
		}
	}
	return count
}

// GenerateErrorReport creates a detailed error report
func (pfh *PartialFailureHandler) GenerateErrorReport(results []QueryResult) string {
	totalQueries := len(results)
	successful := 0
	failed := 0
	errorBreakdown := make(map[ErrorType]int)
	totalRetries := 0

	for _, result := range results {
		if result.Success {
			successful++
		} else {
			failed++
			errorBreakdown[result.ErrorType]++
		}
		totalRetries += result.RetryCount
	}

	report := fmt.Sprintf("# Query Execution Error Report\n\n")
	report += fmt.Sprintf("## Summary\n")
	report += fmt.Sprintf("- Total Queries: %d\n", totalQueries)
	report += fmt.Sprintf("- Successful: %d (%.1f%%)\n", successful, 
		float64(successful)/float64(totalQueries)*100)
	report += fmt.Sprintf("- Failed: %d (%.1f%%)\n", failed,
		float64(failed)/float64(totalQueries)*100)
	report += fmt.Sprintf("- Total Retries: %d\n", totalRetries)

	if len(errorBreakdown) > 0 {
		report += fmt.Sprintf("\n## Error Breakdown\n")
		for errorType, count := range errorBreakdown {
			report += fmt.Sprintf("- %s: %d\n", errorType, count)
		}
	}

	report += fmt.Sprintf("\n## Failed Queries\n")
	for _, result := range results {
		if !result.Success {
			report += fmt.Sprintf("- **%s**: %s (%s, %d retries)\n", 
				result.Query.Name, result.Error.Error(), result.ErrorType, result.RetryCount)
		}
	}

	return report
}

// containsAny checks if text contains any of the provided substrings
func containsAny(text string, substrings []string) bool {
	textLower := strings.ToLower(text)
	for _, substr := range substrings {
		if strings.Contains(textLower, strings.ToLower(substr)) {
			return true
		}
	}
	return false
}