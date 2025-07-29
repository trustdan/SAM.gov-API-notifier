// +build benchmark

package test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/yourusername/sam-gov-monitor/internal/cache"
	"github.com/yourusername/sam-gov-monitor/internal/config"
	"github.com/yourusername/sam-gov-monitor/internal/monitor"
	"github.com/yourusername/sam-gov-monitor/internal/samgov"
)

// BenchmarkQueryExecution benchmarks the execution of individual queries
func BenchmarkQueryExecution(b *testing.B) {
	apiKey := os.Getenv("SAM_API_KEY")
	if apiKey == "" {
		b.Skip("SAM_API_KEY not set, skipping benchmark")
	}

	client := samgov.NewRetryClientWithDefaults(apiKey, false)
	builder := monitor.NewQueryBuilder(7)

	testQuery := config.Query{
		Name:    "Benchmark Query",
		Enabled: true,
		Parameters: map[string]interface{}{
			"title": "software",
			"limit": "50",
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		params, err := builder.BuildParams(testQuery)
		if err != nil {
			b.Fatalf("Failed to build params: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, err = client.SearchWithRetry(ctx, params)
		cancel()

		if err != nil {
			b.Fatalf("Search failed: %v", err)
		}
	}
}

// BenchmarkMultipleQueries benchmarks concurrent execution of multiple queries
func BenchmarkMultipleQueries(b *testing.B) {
	apiKey := os.Getenv("SAM_API_KEY")
	if apiKey == "" {
		b.Skip("SAM_API_KEY not set, skipping benchmark")
	}

	cfg := &config.Config{
		Queries: []config.Query{
			{
				Name:    "Software Query",
				Enabled: true,
				Parameters: map[string]interface{}{
					"title": "software",
					"limit": "25",
				},
			},
			{
				Name:    "Research Query", 
				Enabled: true,
				Parameters: map[string]interface{}{
					"title": "research",
					"limit": "25",
				},
			},
			{
				Name:    "DARPA Query",
				Enabled: true,
				Parameters: map[string]interface{}{
					"organizationName": "DARPA",
					"limit": "25",
				},
			},
		},
	}

	m, err := monitor.New(monitor.Options{
		APIKey:       apiKey,
		Config:       cfg,
		StateFile:    "/tmp/benchmark_state.json",
		Verbose:      false,
		DryRun:       true,
		LookbackDays: 3,
	})
	if err != nil {
		b.Fatalf("Failed to create monitor: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		err := m.Run(ctx)
		cancel()

		if err != nil {
			b.Fatalf("Monitor run failed: %v", err)
		}
	}
}

// BenchmarkQueryBuilder benchmarks query parameter building
func BenchmarkQueryBuilder(b *testing.B) {
	builder := monitor.NewQueryBuilder(7)
	
	testQuery := config.Query{
		Name:    "Complex Query",
		Enabled: true,
		Parameters: map[string]interface{}{
			"title":            "artificial intelligence machine learning",
			"organizationName": "DEFENSE ADVANCED RESEARCH PROJECTS AGENCY",
			"ptype":            []string{"s", "p", "o"},
			"naicsCode":        "541715",
			"state":            "CA",
			"lookbackDays":     14,
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := builder.BuildParams(testQuery)
		if err != nil {
			b.Fatalf("BuildParams failed: %v", err)
		}
	}
}

// BenchmarkAdvancedFiltering benchmarks advanced query filtering
func BenchmarkAdvancedFiltering(b *testing.B) {
	filter := monitor.NewAdvancedFilter(false)
	
	// Create test opportunities
	opportunities := make([]samgov.Opportunity, 100)
	for i := 0; i < 100; i++ {
		opportunities[i] = samgov.Opportunity{
			NoticeID:    "TEST-" + fmt.Sprintf("%03d", i),
			Title:       "Test Opportunity for Software Development " + fmt.Sprintf("%d", i),
			Description: "This is a test opportunity for software development and artificial intelligence research",
			Department:  "DEPARTMENT OF DEFENSE",
			PostedDate:  time.Now().AddDate(0, 0, -i%30).Format("2006-01-02"),
			Type:        "s",
		}
	}

	testQuery := config.Query{
		Name:    "Filter Test",
		Enabled: true,
		Parameters: map[string]interface{}{
			"title": "software",
			"advanced": map[string]interface{}{
				"include":    []string{"artificial intelligence", "machine learning"},
				"exclude":    []string{"training", "educational"},
				"maxDaysOld": 14,
			},
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := filter.FilterOpportunities(opportunities, testQuery)
		if err != nil {
			b.Fatalf("Filtering failed: %v", err)
		}
	}
}

// BenchmarkStateManagement benchmarks state loading and saving
func BenchmarkStateManagement(b *testing.B) {
	stateFile := "/tmp/benchmark_state_mgmt.json"
	defer os.Remove(stateFile)

	// Create test opportunities
	opportunities := make([]samgov.Opportunity, 50)
	for i := 0; i < 50; i++ {
		opportunities[i] = samgov.Opportunity{
			NoticeID:   "BENCH-" + fmt.Sprintf("%03d", i),
			Title:      "Benchmark Opportunity " + fmt.Sprintf("%d", i),
			PostedDate: time.Now().Format("2006-01-02"),
			Type:       "s",
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Load state
		state, err := monitor.LoadState(stateFile)
		if err != nil {
			b.Fatalf("LoadState failed: %v", err)
		}

		// Add opportunities
		for _, opp := range opportunities {
			state.AddOpportunity(opp)
		}

		// Save state
		err = state.Save()
		if err != nil {
			b.Fatalf("Save failed: %v", err)
		}
	}
}

// BenchmarkCaching benchmarks cache performance
func BenchmarkCaching(b *testing.B) {
	cacheDir := "/tmp/benchmark_cache"
	defer os.RemoveAll(cacheDir)

	cache, err := cache.NewCache(cacheDir, 1*time.Hour, false)
	if err != nil {
		b.Fatalf("Failed to create cache: %v", err)
	}

	// Create test response
	testResponse := &samgov.SearchResponse{
		TotalRecords: 100,
		Limit:        50,
		Offset:       0,
		OpportunitiesData: make([]samgov.Opportunity, 50),
	}

	for i := 0; i < 50; i++ {
		testResponse.OpportunitiesData[i] = samgov.Opportunity{
			NoticeID:   "CACHE-" + fmt.Sprintf("%03d", i),
			Title:      "Cache Test Opportunity " + fmt.Sprintf("%d", i),
			PostedDate: time.Now().Format("2006-01-02"),
		}
	}

	params := map[string]string{
		"title": "software",
		"limit": "50",
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.Run("Set", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			testParams := make(map[string]string)
			for k, v := range params {
				testParams[k] = v
			}
			testParams["iteration"] = fmt.Sprintf("%d", i)
			
			err := cache.Set(testParams, testResponse)
			if err != nil {
				b.Fatalf("Cache set failed: %v", err)
			}
		}
	})

	b.Run("Get", func(b *testing.B) {
		// Pre-populate cache
		for i := 0; i < 100; i++ {
			testParams := make(map[string]string)
			for k, v := range params {
				testParams[k] = v
			}
			testParams["iteration"] = fmt.Sprintf("%d", i)
			cache.Set(testParams, testResponse)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			testParams := make(map[string]string)
			for k, v := range params {
				testParams[k] = v
			}
			testParams["iteration"] = fmt.Sprintf("%d", i%100)
			
			_, found := cache.Get(testParams)
			if !found && i%100 < 100 {
				b.Fatalf("Cache miss for existing key")
			}
		}
	})
}

// BenchmarkMetricsCollection benchmarks metrics collection performance
func BenchmarkMetricsCollection(b *testing.B) {
	metricsFile := "/tmp/benchmark_metrics.json"
	defer os.Remove(metricsFile)

	collector := monitor.NewMetricsCollector(metricsFile, false)
	
	testQuery := config.Query{
		Name:    "Metrics Test Query",
		Enabled: true,
		Parameters: map[string]interface{}{
			"title": "test",
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		startTime := collector.RecordRunStart()
		
		// Simulate query execution
		collector.RecordQueryExecution(
			testQuery,
			100*time.Millisecond,
			true,
			nil,
			25,
		)
		
		// Simulate API request
		collector.RecordAPIRequest(50*time.Millisecond, true, 0)
		
		// Simulate opportunities processing
		collector.RecordOpportunities(25, 5, 2)
		
		// Simulate notification
		collector.RecordNotification("email", true)
		
		collector.RecordRunEnd(startTime)
		
		// Save metrics every 10 iterations
		if i%10 == 0 {
			collector.SaveMetrics()
		}
	}
}

// BenchmarkRetryLogic benchmarks retry mechanism performance
func BenchmarkRetryLogic(b *testing.B) {
	// This benchmark tests the retry logic without making actual API calls
	config := samgov.RetryConfig{
		MaxRetries:      3,
		InitialDelay:    10 * time.Millisecond, // Short delay for benchmark
		MaxDelay:        100 * time.Millisecond,
		BackoffFactor:   2.0,
		Jitter:          true,
		RetryableErrors: []int{429, 500, 502, 503, 504},
	}

	client := samgov.NewRetryClient("test-key", config, false)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// This will fail fast since we're using a test key,
		// but we can measure the retry logic overhead
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		params := map[string]string{
			"title": "test",
			"limit": "1",
		}
		
		client.SearchWithRetry(ctx, params)
		cancel()
	}
}

// BenchmarkConfigValidation benchmarks configuration validation
func BenchmarkConfigValidation(b *testing.B) {
	validator := config.NewConfigValidator(false)
	
	testConfig := &config.Config{
		Queries: []config.Query{
			{
				Name:    "Test Query 1",
				Enabled: true,
				Parameters: map[string]interface{}{
					"title":            "software development",
					"organizationName": "DEPARTMENT OF DEFENSE",
					"ptype":            []string{"s", "p"},
					"naicsCode":        "541511",
					"state":            "CA",
				},
				Notification: config.NotificationConfig{
					Priority:   "high",
					Recipients: []string{"test@example.com", "admin@example.com"},
					Channels:   []string{"email", "slack"},
				},
			},
			{
				Name:    "Test Query 2",
				Enabled: true,
				Parameters: map[string]interface{}{
					"title": "artificial intelligence",
					"advanced": map[string]interface{}{
						"include":    []string{"machine learning", "neural network"},
						"exclude":    []string{"training", "educational"},
						"minValue":   100000.0,
						"maxValue":   5000000.0,
						"maxDaysOld": 30,
					},
				},
				Notification: config.NotificationConfig{
					Priority:   "medium",
					Recipients: []string{"ai-team@example.com"},
					Channels:   []string{"email"},
				},
			},
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result := validator.Validate(testConfig)
		if !result.Valid {
			b.Fatalf("Validation failed: %+v", result.Errors)
		}
	}
}

// BenchmarkErrorRecovery benchmarks error recovery system
func BenchmarkErrorRecovery(b *testing.B) {
	handler := monitor.NewPartialFailureHandler(false)
	
	// Create test queries
	queries := make([]config.Query, 5)
	for i := 0; i < 5; i++ {
		queries[i] = config.Query{
			Name:    fmt.Sprintf("Test Query %d", i+1),
			Enabled: true,
			Parameters: map[string]interface{}{
				"title": fmt.Sprintf("test%d", i+1),
				"limit": "10",
			},
		}
	}

	// Create mock results with some failures
	results := make([]monitor.QueryResult, 5)
	for i := 0; i < 5; i++ {
		results[i] = monitor.QueryResult{
			Query:   queries[i],
			Success: i%2 == 0, // Alternate success/failure
			Error:   nil,
			Duration: 100 * time.Millisecond,
		}
		if !results[i].Success {
			results[i].Error = fmt.Errorf("simulated error %d", i)
			results[i].ErrorType = monitor.ErrorTypeNetwork
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		report := handler.GenerateErrorReport(results)
		if len(report) == 0 {
			b.Fatalf("Empty error report generated")
		}
	}
}

// Add missing import and fix compilation
import "fmt"

// RunBenchmarks runs all benchmarks and generates a performance report
func RunBenchmarks() {
	fmt.Println("To run benchmarks, use:")
	fmt.Println("go test -tags=benchmark -bench=. -benchmem ./test/")
	fmt.Println("")
	fmt.Println("Specific benchmarks:")
	fmt.Println("go test -tags=benchmark -bench=BenchmarkQueryExecution -benchmem ./test/")
	fmt.Println("go test -tags=benchmark -bench=BenchmarkMultipleQueries -benchmem ./test/")
	fmt.Println("go test -tags=benchmark -bench=BenchmarkCaching -benchmem ./test/")
}