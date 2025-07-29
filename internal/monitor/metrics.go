package monitor

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/yourusername/sam-gov-monitor/internal/config"
)

// Metrics tracks performance and operational statistics
type Metrics struct {
	mu sync.RWMutex

	// Run-level metrics
	TotalRuns           int                 `json:"total_runs"`
	LastRunTime         time.Time           `json:"last_run_time"`
	AverageRunDuration  time.Duration       `json:"average_run_duration"`
	RunDurations        []time.Duration     `json:"run_durations"`

	// Query-level metrics  
	QueryMetrics        map[string]*QueryMetrics `json:"query_metrics"`
	
	// API-level metrics
	TotalAPIRequests    int                 `json:"total_api_requests"`
	SuccessfulRequests  int                 `json:"successful_requests"`
	FailedRequests      int                 `json:"failed_requests"`
	TotalRetries        int                 `json:"total_retries"`
	AverageResponseTime time.Duration       `json:"average_response_time"`
	ResponseTimes       []time.Duration     `json:"response_times"`

	// Opportunity metrics
	TotalOpportunities     int            `json:"total_opportunities"`
	NewOpportunities       int            `json:"new_opportunities"`
	UpdatedOpportunities   int            `json:"updated_opportunities"`
	OpportunitiesPerQuery  map[string]int `json:"opportunities_per_query"`

	// Error metrics
	ErrorCounts         map[string]int      `json:"error_counts"`
	LastError           string              `json:"last_error"`
	LastErrorTime       time.Time           `json:"last_error_time"`

	// Notification metrics
	NotificationsSent   int                 `json:"notifications_sent"`
	NotificationErrors  int                 `json:"notification_errors"`
	NotificationsByType map[string]int      `json:"notifications_by_type"`

	// Performance thresholds
	SlowQueryThreshold    time.Duration     `json:"slow_query_threshold"`
	SlowQueries          []SlowQuery        `json:"slow_queries"`
}

// QueryMetrics tracks metrics for individual queries
type QueryMetrics struct {
	Name                     string        `json:"name"`
	ExecutionCount           int           `json:"execution_count"`
	SuccessCount             int           `json:"success_count"`
	FailureCount             int           `json:"failure_count"`
	AverageDuration          time.Duration `json:"average_duration"`
	LastExecuted             time.Time     `json:"last_executed"`
	TotalOpportunities       int           `json:"total_opportunities"`
	LastError                string        `json:"last_error"`
	Durations                []time.Duration `json:"durations"`
	AverageTime              time.Duration `json:"average_time"`
	LastOpportunityCount     int           `json:"last_opportunity_count"`
	TotalOpportunitiesFound  int           `json:"total_opportunities_found"`
	ErrorCount               int           `json:"error_count"`
}

// SlowQuery represents a query that exceeded performance thresholds
type SlowQuery struct {
	QueryName string        `json:"query_name"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
}

// MetricsCollector manages metrics collection and reporting
type MetricsCollector struct {
	metrics  *Metrics
	verbose  bool
	filePath string
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(filePath string, verbose bool) *MetricsCollector {
	mc := &MetricsCollector{
		filePath: filePath,
		verbose:  verbose,
		metrics: &Metrics{
			QueryMetrics:          make(map[string]*QueryMetrics),
			OpportunitiesPerQuery: make(map[string]int),
			ErrorCounts:          make(map[string]int),
			NotificationsByType:  make(map[string]int),
			SlowQueryThreshold:   10 * time.Second, // Default threshold
		},
	}

	// Load existing metrics if file exists
	mc.loadMetricsFromFile()
	
	return mc
}

// RecordRunStart marks the beginning of a monitoring run
func (mc *MetricsCollector) RecordRunStart() time.Time {
	mc.metrics.mu.Lock()
	defer mc.metrics.mu.Unlock()
	
	mc.metrics.TotalRuns++
	startTime := time.Now()
	
	if mc.verbose {
		log.Printf("Starting monitoring run #%d", mc.metrics.TotalRuns)
	}
	
	return startTime
}

// RecordRunEnd marks the completion of a monitoring run
func (mc *MetricsCollector) RecordRunEnd(startTime time.Time) {
	mc.metrics.mu.Lock()
	defer mc.metrics.mu.Unlock()
	
	duration := time.Since(startTime)
	mc.metrics.LastRunTime = time.Now()
	mc.metrics.RunDurations = append(mc.metrics.RunDurations, duration)
	
	// Keep only last 100 run durations
	if len(mc.metrics.RunDurations) > 100 {
		mc.metrics.RunDurations = mc.metrics.RunDurations[1:]
	}
	
	// Calculate average
	total := time.Duration(0)
	for _, d := range mc.metrics.RunDurations {
		total += d
	}
	mc.metrics.AverageRunDuration = total / time.Duration(len(mc.metrics.RunDurations))
	
	if mc.verbose {
		log.Printf("Monitoring run completed in %v (average: %v)", 
			duration, mc.metrics.AverageRunDuration)
	}
}

// RecordQueryExecution records metrics for a query execution
func (mc *MetricsCollector) RecordQueryExecution(query config.Query, duration time.Duration, success bool, error error, opportunityCount int) {
	mc.metrics.mu.Lock()
	defer mc.metrics.mu.Unlock()
	
	queryName := query.Name
	
	// Initialize query metrics if not exists
	if mc.metrics.QueryMetrics[queryName] == nil {
		mc.metrics.QueryMetrics[queryName] = &QueryMetrics{
			Name: queryName,
		}
	}
	
	qm := mc.metrics.QueryMetrics[queryName]
	qm.ExecutionCount++
	qm.LastExecuted = time.Now()
	qm.Durations = append(qm.Durations, duration)
	
	// Keep only last 50 durations per query
	if len(qm.Durations) > 50 {
		qm.Durations = qm.Durations[1:]
	}
	
	// Calculate average duration
	total := time.Duration(0)
	for _, d := range qm.Durations {
		total += d
	}
	qm.AverageDuration = total / time.Duration(len(qm.Durations))
	
	if success {
		qm.SuccessCount++
		qm.TotalOpportunities += opportunityCount
		mc.metrics.OpportunitiesPerQuery[queryName] = qm.TotalOpportunities
		
		if mc.verbose {
			log.Printf("Query '%s' completed in %v, found %d opportunities", 
				queryName, duration, opportunityCount)
		}
	} else {
		qm.FailureCount++
		if error != nil {
			qm.LastError = error.Error()
			mc.metrics.ErrorCounts[error.Error()]++
			mc.metrics.LastError = error.Error()
			mc.metrics.LastErrorTime = time.Now()
		}
		
		if mc.verbose {
			log.Printf("Query '%s' failed after %v: %v", queryName, duration, error)
		}
	}
	
	// Check for slow queries
	if duration > mc.metrics.SlowQueryThreshold {
		slowQuery := SlowQuery{
			QueryName: queryName,
			Duration:  duration,
			Timestamp: time.Now(),
		}
		mc.metrics.SlowQueries = append(mc.metrics.SlowQueries, slowQuery)
		
		// Keep only last 20 slow queries
		if len(mc.metrics.SlowQueries) > 20 {
			mc.metrics.SlowQueries = mc.metrics.SlowQueries[1:]
		}
		
		if mc.verbose {
			log.Printf("SLOW QUERY: '%s' took %v (threshold: %v)", 
				queryName, duration, mc.metrics.SlowQueryThreshold)
		}
	}
}

// RecordAPIRequest records metrics for API requests
func (mc *MetricsCollector) RecordAPIRequest(duration time.Duration, success bool, retryCount int) {
	mc.metrics.mu.Lock()
	defer mc.metrics.mu.Unlock()
	
	mc.metrics.TotalAPIRequests++
	mc.metrics.TotalRetries += retryCount
	mc.metrics.ResponseTimes = append(mc.metrics.ResponseTimes, duration)
	
	// Keep only last 200 response times
	if len(mc.metrics.ResponseTimes) > 200 {
		mc.metrics.ResponseTimes = mc.metrics.ResponseTimes[1:]
	}
	
	// Calculate average response time
	total := time.Duration(0)
	for _, rt := range mc.metrics.ResponseTimes {
		total += rt
	}
	mc.metrics.AverageResponseTime = total / time.Duration(len(mc.metrics.ResponseTimes))
	
	if success {
		mc.metrics.SuccessfulRequests++
	} else {
		mc.metrics.FailedRequests++
	}
}

// RecordOpportunities records opportunity-related metrics
func (mc *MetricsCollector) RecordOpportunities(total, new, updated int) {
	mc.metrics.mu.Lock()
	defer mc.metrics.mu.Unlock()
	
	mc.metrics.TotalOpportunities += total
	mc.metrics.NewOpportunities += new
	mc.metrics.UpdatedOpportunities += updated
	
	if mc.verbose && (new > 0 || updated > 0) {
		log.Printf("Opportunities: %d new, %d updated out of %d total", new, updated, total)
	}
}

// RecordNotification records notification metrics
func (mc *MetricsCollector) RecordNotification(notificationType string, success bool) {
	mc.metrics.mu.Lock()
	defer mc.metrics.mu.Unlock()
	
	if success {
		mc.metrics.NotificationsSent++
		mc.metrics.NotificationsByType[notificationType]++
	} else {
		mc.metrics.NotificationErrors++
	}
}

// GetMetrics returns a copy of current metrics
func (mc *MetricsCollector) GetMetrics() Metrics {
	mc.metrics.mu.RLock()
	defer mc.metrics.mu.RUnlock()
	
	// Deep copy the metrics
	metricsCopy := *mc.metrics
	
	// Copy maps
	metricsCopy.QueryMetrics = make(map[string]*QueryMetrics)
	for k, v := range mc.metrics.QueryMetrics {
		qmCopy := *v
		metricsCopy.QueryMetrics[k] = &qmCopy
	}
	
	metricsCopy.OpportunitiesPerQuery = make(map[string]int)
	for k, v := range mc.metrics.OpportunitiesPerQuery {
		metricsCopy.OpportunitiesPerQuery[k] = v
	}
	
	metricsCopy.ErrorCounts = make(map[string]int)
	for k, v := range mc.metrics.ErrorCounts {
		metricsCopy.ErrorCounts[k] = v
	}
	
	metricsCopy.NotificationsByType = make(map[string]int)
	for k, v := range mc.metrics.NotificationsByType {
		metricsCopy.NotificationsByType[k] = v
	}
	
	return metricsCopy
}

// SaveMetrics persists metrics to file
func (mc *MetricsCollector) SaveMetrics() error {
	mc.metrics.mu.RLock()
	defer mc.metrics.mu.RUnlock()
	
	data, err := json.MarshalIndent(mc.metrics, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling metrics: %w", err)
	}
	
	err = os.WriteFile(mc.filePath, data, 0644)
	if err != nil {
		return fmt.Errorf("writing metrics file: %w", err)
	}
	
	if mc.verbose {
		log.Printf("Metrics saved to %s", mc.filePath)
	}
	
	return nil
}

// loadMetricsFromFile loads existing metrics from file
func (mc *MetricsCollector) loadMetricsFromFile() {
	data, err := os.ReadFile(mc.filePath)
	if err != nil {
		if !os.IsNotExist(err) && mc.verbose {
			log.Printf("Warning: Could not load existing metrics: %v", err)
		}
		return
	}
	
	var loadedMetrics Metrics
	err = json.Unmarshal(data, &loadedMetrics)
	if err != nil {
		if mc.verbose {
			log.Printf("Warning: Could not parse existing metrics: %v", err)
		}
		return
	}
	
	// Initialize maps if nil
	if loadedMetrics.QueryMetrics == nil {
		loadedMetrics.QueryMetrics = make(map[string]*QueryMetrics)
	}
	if loadedMetrics.OpportunitiesPerQuery == nil {
		loadedMetrics.OpportunitiesPerQuery = make(map[string]int)
	}
	if loadedMetrics.ErrorCounts == nil {
		loadedMetrics.ErrorCounts = make(map[string]int)
	}
	if loadedMetrics.NotificationsByType == nil {
		loadedMetrics.NotificationsByType = make(map[string]int)
	}
	
	mc.metrics = &loadedMetrics
	
	if mc.verbose {
		log.Printf("Loaded existing metrics from %s", mc.filePath)
	}
}

// GenerateReport creates a comprehensive metrics report
func (mc *MetricsCollector) GenerateReport() string {
	metrics := mc.GetMetrics()
	
	report := fmt.Sprintf("# SAM.gov Monitor Performance Report\n\n")
	report += fmt.Sprintf("Generated: %s\n\n", time.Now().Format(time.RFC3339))
	
	// Run-level metrics
	report += fmt.Sprintf("## Run Statistics\n")
	report += fmt.Sprintf("- Total Runs: %d\n", metrics.TotalRuns)
	if !metrics.LastRunTime.IsZero() {
		report += fmt.Sprintf("- Last Run: %s\n", metrics.LastRunTime.Format(time.RFC3339))
	}
	if metrics.AverageRunDuration > 0 {
		report += fmt.Sprintf("- Average Duration: %v\n", metrics.AverageRunDuration)
	}
	
	// API metrics
	report += fmt.Sprintf("\n## API Performance\n")
	report += fmt.Sprintf("- Total Requests: %d\n", metrics.TotalAPIRequests)
	report += fmt.Sprintf("- Successful: %d (%.1f%%)\n", metrics.SuccessfulRequests,
		float64(metrics.SuccessfulRequests)/float64(metrics.TotalAPIRequests)*100)
	report += fmt.Sprintf("- Failed: %d (%.1f%%)\n", metrics.FailedRequests,
		float64(metrics.FailedRequests)/float64(metrics.TotalAPIRequests)*100)
	if metrics.TotalRetries > 0 {
		report += fmt.Sprintf("- Total Retries: %d\n", metrics.TotalRetries)
	}
	if metrics.AverageResponseTime > 0 {
		report += fmt.Sprintf("- Average Response Time: %v\n", metrics.AverageResponseTime)
	}
	
	// Query performance
	if len(metrics.QueryMetrics) > 0 {
		report += fmt.Sprintf("\n## Query Performance\n")
		for name, qm := range metrics.QueryMetrics {
			successRate := float64(qm.SuccessCount) / float64(qm.ExecutionCount) * 100
			report += fmt.Sprintf("- **%s**: %d executions, %.1f%% success, avg %v, %d opportunities\n",
				name, qm.ExecutionCount, successRate, qm.AverageDuration, qm.TotalOpportunities)
		}
	}
	
	// Opportunity metrics
	report += fmt.Sprintf("\n## Opportunity Statistics\n")
	report += fmt.Sprintf("- Total Opportunities: %d\n", metrics.TotalOpportunities)
	report += fmt.Sprintf("- New Opportunities: %d\n", metrics.NewOpportunities)
	report += fmt.Sprintf("- Updated Opportunities: %d\n", metrics.UpdatedOpportunities)
	
	// Notification metrics
	if metrics.NotificationsSent > 0 || metrics.NotificationErrors > 0 {
		report += fmt.Sprintf("\n## Notification Statistics\n")
		report += fmt.Sprintf("- Sent: %d\n", metrics.NotificationsSent)
		if metrics.NotificationErrors > 0 {
			report += fmt.Sprintf("- Errors: %d\n", metrics.NotificationErrors)
		}
		
		if len(metrics.NotificationsByType) > 0 {
			report += fmt.Sprintf("- By Type:\n")
			for notType, count := range metrics.NotificationsByType {
				report += fmt.Sprintf("  - %s: %d\n", notType, count)
			}
		}
	}
	
	// Error summary
	if len(metrics.ErrorCounts) > 0 {
		report += fmt.Sprintf("\n## Error Summary\n")
		for errorMsg, count := range metrics.ErrorCounts {
			report += fmt.Sprintf("- %s: %d occurrences\n", errorMsg, count)
		}
	}
	
	// Slow queries
	if len(metrics.SlowQueries) > 0 {
		report += fmt.Sprintf("\n## Slow Queries (>%v)\n", metrics.SlowQueryThreshold)
		for _, sq := range metrics.SlowQueries {
			report += fmt.Sprintf("- %s: %v at %s\n", 
				sq.QueryName, sq.Duration, sq.Timestamp.Format("15:04:05"))
		}
	}
	
	return report
}

// GetHealthStatus returns the overall health status
func (mc *MetricsCollector) GetHealthStatus() map[string]interface{} {
	metrics := mc.GetMetrics()
	
	status := make(map[string]interface{})
	
	// Overall health
	if metrics.TotalAPIRequests > 0 {
		successRate := float64(metrics.SuccessfulRequests) / float64(metrics.TotalAPIRequests) * 100
		status["api_success_rate"] = fmt.Sprintf("%.1f%%", successRate)
		status["healthy"] = successRate > 95.0
	} else {
		status["healthy"] = true
	}
	
	// Recent activity
	status["last_run"] = metrics.LastRunTime
	status["total_runs"] = metrics.TotalRuns
	
	// Performance indicators
	if metrics.AverageRunDuration > 0 {
		status["avg_run_duration"] = metrics.AverageRunDuration.String()
	}
	if metrics.AverageResponseTime > 0 {
		status["avg_response_time"] = metrics.AverageResponseTime.String()
	}
	
	// Error indicators
	status["recent_errors"] = len(metrics.ErrorCounts)
	status["slow_queries"] = len(metrics.SlowQueries)
	
	return status
}