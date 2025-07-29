// +build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/yourusername/sam-gov-monitor/internal/config"
	"github.com/yourusername/sam-gov-monitor/internal/monitor"
	"github.com/yourusername/sam-gov-monitor/internal/samgov"
)

func TestMonitorIntegration(t *testing.T) {
	// Skip if no API key available
	apiKey := os.Getenv("SAM_API_KEY")
	if apiKey == "" {
		t.Skip("SAM_API_KEY not set, skipping integration test")
	}

	// Create test configuration
	cfg := &config.Config{
		Queries: []config.Query{
			{
				Name:    "Integration Test Query",
				Enabled: true,
				Parameters: map[string]interface{}{
					"title": "software",
					"limit": "5", // Small limit for testing
				},
				Notification: config.NotificationConfig{
					Priority: "medium",
				},
			},
		},
	}

	// Create monitor
	m, err := monitor.New(monitor.Options{
		APIKey:       apiKey,
		Config:       cfg,
		StateFile:    "", // In-memory only
		Verbose:      testing.Verbose(),
		DryRun:       true,
		LookbackDays: 7,
	})
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}

	// Run monitor with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = m.Run(ctx)
	if err != nil {
		t.Errorf("Monitor run failed: %v", err)
	}
}

func TestAPIClientIntegration(t *testing.T) {
	apiKey := os.Getenv("SAM_API_KEY")
	if apiKey == "" {
		t.Skip("SAM_API_KEY not set, skipping integration test")
	}

	client := samgov.NewClient(apiKey)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test API key validation
	err := client.ValidateAPIKey(ctx)
	if err != nil {
		t.Errorf("API key validation failed: %v", err)
	}

	// Test basic search
	params := map[string]string{
		"limit":      "5",
		"postedFrom": time.Now().AddDate(0, 0, -7).Format("01/02/2006"),
		"postedTo":   time.Now().Format("01/02/2006"),
	}

	response, err := client.Search(ctx, params)
	if err != nil {
		t.Errorf("Basic search failed: %v", err)
		return
	}

	if response == nil {
		t.Error("Response is nil")
		return
	}

	t.Logf("Search returned %d total records, %d in this page", 
		response.TotalRecords, len(response.OpportunitiesData))

	// Validate response structure
	if response.TotalRecords < 0 {
		t.Error("TotalRecords should be non-negative")
	}

	if response.Limit != 5 {
		t.Errorf("Expected limit 5, got %d", response.Limit)
	}

	// Validate opportunities structure
	for i, opp := range response.OpportunitiesData {
		if opp.NoticeID == "" {
			t.Errorf("Opportunity %d has empty NoticeID", i)
		}
		if opp.Title == "" {
			t.Errorf("Opportunity %d has empty Title", i)
		}
		if opp.PostedDate == "" {
			t.Errorf("Opportunity %d has empty PostedDate", i)
		}
	}
}

func TestRetryClientIntegration(t *testing.T) {
	apiKey := os.Getenv("SAM_API_KEY")
	if apiKey == "" {
		t.Skip("SAM_API_KEY not set, skipping integration test")
	}

	// Test retry client with default config
	retryClient := samgov.NewRetryClientWithDefaults(apiKey, testing.Verbose())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test API validation with retry
	err := retryClient.ValidateAPIKeyWithRetry(ctx)
	if err != nil {
		t.Errorf("Retry client API validation failed: %v", err)
	}

	// Test search with retry
	params := map[string]string{
		"title": "research",
		"limit": "3",
	}

	response, err := retryClient.SearchWithDefaultsAndRetry(ctx, params, 3)
	if err != nil {
		t.Errorf("Retry search failed: %v", err)
		return
	}

	if response == nil {
		t.Error("Retry search returned nil response")
		return
	}

	t.Logf("Retry search found %d opportunities", len(response.OpportunitiesData))
}

func TestQueryBuilderIntegration(t *testing.T) {
	builder := monitor.NewQueryBuilder(7)

	testCases := []struct {
		name  string
		query config.Query
	}{
		{
			name: "DARPA AI Query",
			query: config.Query{
				Name:    "DARPA AI Test",
				Enabled: true,
				Parameters: map[string]interface{}{
					"title":            "artificial intelligence",
					"organizationName": "DARPA",
					"ptype":            []string{"s", "p"},
				},
			},
		},
		{
			name: "Small Business Query",
			query: config.Query{
				Name:    "Small Business Test",
				Enabled: true,
				Parameters: map[string]interface{}{
					"typeOfSetAside": []string{"SBA", "8A"},
					"naicsCode":      "541512",
					"state":          "CA",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Validate query
			err := builder.ValidateParameters(tc.query)
			if err != nil {
				t.Errorf("Query validation failed: %v", err)
			}

			// Build parameters
			params, err := builder.BuildParams(tc.query)
			if err != nil {
				t.Errorf("Parameter building failed: %v", err)
				return
			}

			// Check required parameters are present
			if params["postedFrom"] == "" {
				t.Error("postedFrom parameter missing")
			}
			if params["postedTo"] == "" {
				t.Error("postedTo parameter missing")
			}
			if params["limit"] == "" {
				t.Error("limit parameter missing")
			}

			t.Logf("Built parameters: %v", params)
		})
	}
}

func TestStateManagementIntegration(t *testing.T) {
	// Create temporary state file
	stateFile := "/tmp/test_monitor_state.json"
	defer os.Remove(stateFile)

	// Load initial state
	state, err := monitor.LoadState(stateFile)
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// Add test opportunities
	testOpps := []samgov.Opportunity{
		{
			NoticeID:    "TEST-001",
			Title:       "Test Opportunity 1",
			PostedDate:  time.Now().Format("2006-01-02"),
			Type:        "s",
		},
		{
			NoticeID:    "TEST-002",
			Title:       "Test Opportunity 2",
			PostedDate:  time.Now().Format("2006-01-02"),
			Type:        "p",
		},
	}

	// Test adding opportunities
	for _, opp := range testOpps {
		isNew := state.AddOpportunity(opp)
		if !isNew {
			t.Errorf("Expected opportunity %s to be new", opp.NoticeID)
		}
	}

	// Test retrieving opportunities
	for _, opp := range testOpps {
		stored, exists := state.GetOpportunity(opp.NoticeID)
		if !exists {
			t.Errorf("Opportunity %s not found in state", opp.NoticeID)
			continue
		}
		if stored.Title != opp.Title {
			t.Errorf("Title mismatch for %s: expected %s, got %s", 
				opp.NoticeID, opp.Title, stored.Title)
		}
	}

	// Test saving state
	err = state.Save()
	if err != nil {
		t.Errorf("Failed to save state: %v", err)
	}

	// Test loading saved state
	loadedState, err := monitor.LoadState(stateFile)
	if err != nil {
		t.Errorf("Failed to load saved state: %v", err)
		return
	}

	// Verify loaded state contains our opportunities
	for _, opp := range testOpps {
		_, exists := loadedState.GetOpportunity(opp.NoticeID)
		if !exists {
			t.Errorf("Opportunity %s not found in loaded state", opp.NoticeID)
		}
	}

	// Test state statistics
	stats := loadedState.GetStats()
	if stats.TotalOpportunities != len(testOpps) {
		t.Errorf("Expected %d opportunities in stats, got %d", 
			len(testOpps), stats.TotalOpportunities)
	}
}

func TestOpportunityDifferIntegration(t *testing.T) {
	differ := monitor.NewOpportunityDiffer(testing.Verbose())
	
	// Create state and add initial opportunities
	state, _ := monitor.LoadState("")
	
	initialOpps := []samgov.Opportunity{
		{
			NoticeID:    "DIFF-001",
			Title:       "Original Title",
			PostedDate:  "2024-01-01",
			Type:        "s",
		},
	}

	for _, opp := range initialOpps {
		state.AddOpportunity(opp)
	}

	// Test with same opportunities (should be existing)
	diff := differ.DiffOpportunities(initialOpps, state)
	if len(diff.New) != 0 {
		t.Errorf("Expected 0 new opportunities, got %d", len(diff.New))
	}
	if len(diff.Existing) != 1 {
		t.Errorf("Expected 1 existing opportunity, got %d", len(diff.Existing))
	}

	// Test with new opportunity
	newOpps := append(initialOpps, samgov.Opportunity{
		NoticeID:   "DIFF-002",
		Title:      "New Opportunity",
		PostedDate: "2024-01-02",
		Type:       "p",
	})

	diff = differ.DiffOpportunities(newOpps, state)
	if len(diff.New) != 1 {
		t.Errorf("Expected 1 new opportunity, got %d", len(diff.New))
	}
	if len(diff.Existing) != 1 {
		t.Errorf("Expected 1 existing opportunity, got %d", len(diff.Existing))
	}

	// Test with updated opportunity
	updatedOpps := []samgov.Opportunity{
		{
			NoticeID:    "DIFF-001",
			Title:       "Updated Title", // Changed title
			PostedDate:  "2024-01-01",
			Type:        "s",
		},
	}

	diff = differ.DiffOpportunities(updatedOpps, state)
	if len(diff.Updated) != 1 {
		t.Errorf("Expected 1 updated opportunity, got %d", len(diff.Updated))
	}

	// Test generating diff report
	report := differ.GenerateDiffReport(diff, "Test Query")
	if report == "" {
		t.Error("Diff report should not be empty")
	}
	t.Logf("Diff report:\n%s", report)
}

func TestConfigValidationIntegration(t *testing.T) {
	testCases := []struct {
		name        string
		configPath  string
		expectError bool
	}{
		{
			name:        "Valid Config",
			configPath:  "../../config/queries.yaml",
			expectError: false,
		},
		{
			name:        "Nonexistent Config",
			configPath:  "nonexistent.yaml",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := config.Load(tc.configPath)
			
			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Validate configuration
			if err := cfg.Validate(); err != nil {
				t.Errorf("Config validation failed: %v", err)
			}

			// Check that we have enabled queries
			enabledQueries := cfg.GetEnabledQueries()
			if len(enabledQueries) == 0 {
				t.Error("No enabled queries found")
			}

			t.Logf("Config loaded successfully with %d enabled queries", len(enabledQueries))
		})
	}
}

func TestEndToEndIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping end-to-end test in short mode")
	}

	apiKey := os.Getenv("SAM_API_KEY")
	if apiKey == "" {
		t.Skip("SAM_API_KEY not set, skipping end-to-end test")
	}

	// Load real configuration
	cfg, err := config.Load("../../config/queries.yaml")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Use only the first enabled query for testing
	enabledQueries := cfg.GetEnabledQueries()
	if len(enabledQueries) == 0 {
		t.Fatal("No enabled queries in configuration")
	}

	// Create test config with single query
	testCfg := &config.Config{
		Queries: []config.Query{enabledQueries[0]},
	}

	// Create temporary state file
	stateFile := "/tmp/e2e_test_state.json"
	defer os.Remove(stateFile)

	// Create monitor
	m, err := monitor.New(monitor.Options{
		APIKey:       apiKey,
		Config:       testCfg,
		StateFile:    stateFile,
		Verbose:      testing.Verbose(),
		DryRun:       true, // Always dry run for tests
		LookbackDays: 3,    // Short lookback for faster tests
	})
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}

	// Run first monitoring cycle
	ctx1, cancel1 := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel1()

	t.Log("Running first monitoring cycle...")
	err = m.Run(ctx1)
	if err != nil {
		t.Errorf("First monitor run failed: %v", err)
	}

	// Verify state file was created
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Error("State file was not created")
	}

	// Run second monitoring cycle (should use existing state)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel2()

	t.Log("Running second monitoring cycle...")
	err = m.Run(ctx2)
	if err != nil {
		t.Errorf("Second monitor run failed: %v", err)
	}

	t.Log("End-to-end test completed successfully")
}