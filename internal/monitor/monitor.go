package monitor

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/yourusername/sam-gov-monitor/internal/config"
	"github.com/yourusername/sam-gov-monitor/internal/notify"
	"github.com/yourusername/sam-gov-monitor/internal/samgov"
)

// Monitor manages the monitoring process
type Monitor struct {
	client      *samgov.Client
	config      *config.Config
	state       *State
	builder     *QueryBuilder
	notifyMgr   *notify.NotificationManager
	verbose     bool
	dryRun      bool
	lookbackDays int
}

// Options for creating a new Monitor
type Options struct {
	APIKey       string
	Config       *config.Config
	StateFile    string
	Verbose      bool
	DryRun       bool
	LookbackDays int
}

// RunReport contains the results of a monitoring run
type RunReport struct {
	StartTime       time.Time            `json:"start_time"`
	EndTime         time.Time            `json:"end_time"`
	Duration        time.Duration        `json:"duration"`
	QueriesRun      int                  `json:"queries_run"`
	QueriesSucceded int                  `json:"queries_succeeded"`
	QueriesFailed   int                  `json:"queries_failed"`
	NewOpps         int                  `json:"new_opportunities"`
	UpdatedOpps     int                  `json:"updated_opportunities"`
	TotalOpps       int                  `json:"total_opportunities"`
	Notifications   int                  `json:"notifications_sent"`
	Errors          []string             `json:"errors"`
	QueryResults    []samgov.QueryResult `json:"query_results"`
}

// New creates a new Monitor instance
func New(opts Options) (*Monitor, error) {
	if opts.Config == nil {
		return nil, fmt.Errorf("config is required")
	}

	if opts.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	if opts.LookbackDays <= 0 {
		opts.LookbackDays = 3 // default
	}

	// Initialize state
	state, err := LoadState(opts.StateFile)
	if err != nil {
		return nil, fmt.Errorf("loading state: %w", err)
	}

	// Initialize notification manager
	notifyConfig := buildNotificationConfig()
	notifyMgr := notify.NewNotificationManager(notifyConfig, opts.Verbose)

	return &Monitor{
		client:       samgov.NewClient(opts.APIKey),
		config:       opts.Config,
		state:        state,
		builder:      NewQueryBuilder(opts.LookbackDays),
		notifyMgr:    notifyMgr,
		verbose:      opts.Verbose,
		dryRun:       opts.DryRun,
		lookbackDays: opts.LookbackDays,
	}, nil
}

// Run executes all enabled queries and processes results
func (m *Monitor) Run(ctx context.Context) error {
	report := &RunReport{
		StartTime:    time.Now(),
		QueryResults: make([]samgov.QueryResult, 0),
		Errors:       make([]string, 0),
	}

	defer func() {
		report.EndTime = time.Now()
		report.Duration = report.EndTime.Sub(report.StartTime)
		m.logReport(report)
	}()

	if m.verbose {
		log.Printf("Starting monitoring run with %d enabled queries", len(m.config.GetEnabledQueries()))
	}

	// Execute all queries concurrently
	results, err := m.runQueries(ctx)
	if err != nil {
		report.Errors = append(report.Errors, err.Error())
		return fmt.Errorf("running queries: %w", err)
	}

	report.QueryResults = results
	report.QueriesRun = len(results)

	// Process results
	for _, result := range results {
		if result.Error != nil {
			report.QueriesFailed++
			report.Errors = append(report.Errors, fmt.Sprintf("Query '%s': %s", result.QueryName, result.Error.Error()))
			continue
		}

		report.QueriesSucceded++
		report.TotalOpps += len(result.Opportunities)

		// Detect new and updated opportunities
		diff := m.diffOpportunities(result.Opportunities)
		
		newCount := len(diff.New)
		updatedCount := len(diff.Updated)
		
		report.NewOpps += newCount
		report.UpdatedOpps += updatedCount

		if m.verbose {
			log.Printf("Query '%s': %d total, %d new, %d updated", 
				result.QueryName, len(result.Opportunities), newCount, updatedCount)
		}

		// Update state with all opportunities
		for _, opp := range result.Opportunities {
			m.state.AddOpportunity(opp)
		}

		// Send notifications for new/updated opportunities
		if !m.dryRun && (newCount > 0 || updatedCount > 0) {
			query := m.findQueryByName(result.QueryName)
			if query != nil {
				err := m.sendNotifications(ctx, *query, diff)
				if err != nil {
					report.Errors = append(report.Errors, fmt.Sprintf("Notification error for '%s': %s", result.QueryName, err.Error()))
					log.Printf("Failed to send notifications for query '%s': %v", result.QueryName, err)
				} else {
					report.Notifications++
					if m.verbose {
						log.Printf("Notifications sent for query '%s': %d new + %d updated", result.QueryName, newCount, updatedCount)
					}
				}
			}
		} else if m.dryRun && (newCount > 0 || updatedCount > 0) {
			if m.verbose {
				log.Printf("[DRY RUN] Would send notifications for %d new + %d updated opportunities", newCount, updatedCount)
			}
		}
	}

	// Save state
	if !m.dryRun {
		if err := m.state.Save(); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("saving state: %s", err.Error()))
			return fmt.Errorf("saving state: %w", err)
		}
	}

	// Update last run time
	m.state.SetLastRun(time.Now())

	return nil
}

// runQueries executes all enabled queries concurrently
func (m *Monitor) runQueries(ctx context.Context) ([]samgov.QueryResult, error) {
	enabledQueries := m.config.GetEnabledQueries()
	results := make([]samgov.QueryResult, len(enabledQueries))
	
	// Use WaitGroup for concurrent execution
	var wg sync.WaitGroup
	
	for i, query := range enabledQueries {
		wg.Add(1)
		
		go func(index int, q config.Query) {
			defer wg.Done()
			
			if m.verbose {
				log.Printf("Starting query: %s", q.Name)
			}
			
			start := time.Now()
			result := m.executeQuery(ctx, q)
			result.ExecutionTime = time.Since(start)
			
			results[index] = result
			
			if m.verbose {
				if result.Error != nil {
					log.Printf("Query '%s' failed in %v: %s", q.Name, result.ExecutionTime, result.Error.Error())
				} else {
					log.Printf("Query '%s' completed in %v: %d opportunities", q.Name, result.ExecutionTime, len(result.Opportunities))
				}
			}
		}(i, query)
	}
	
	// Wait for all queries to complete
	wg.Wait()
	
	return results, nil
}

// executeQuery runs a single query
func (m *Monitor) executeQuery(ctx context.Context, query config.Query) samgov.QueryResult {
	result := samgov.QueryResult{
		QueryName:     query.Name,
		Opportunities: make([]samgov.Opportunity, 0),
	}

	// Build API parameters
	params, err := m.builder.BuildParams(query)
	if err != nil {
		result.Error = fmt.Errorf("building parameters: %w", err)
		return result
	}

	if m.verbose {
		log.Printf("Query '%s' parameters: %v", query.Name, params)
	}

	// Execute search
	response, err := m.client.Search(ctx, params)
	if err != nil {
		result.Error = fmt.Errorf("API search: %w", err)
		return result
	}

	// Apply advanced filtering if configured
	opportunities := response.OpportunitiesData
	if len(opportunities) > 0 {
		opportunities = m.applyAdvancedFilters(opportunities, query.Advanced)
	}

	result.Opportunities = opportunities
	return result
}

// applyAdvancedFilters applies client-side filtering
func (m *Monitor) applyAdvancedFilters(opportunities []samgov.Opportunity, advanced config.AdvancedQuery) []samgov.Opportunity {
	if len(advanced.Include) == 0 && len(advanced.Exclude) == 0 && advanced.MaxDaysOld == 0 {
		return opportunities // No filters configured
	}

	filtered := make([]samgov.Opportunity, 0)
	
	for _, opp := range opportunities {
		if m.matchesAdvancedCriteria(opp, advanced) {
			filtered = append(filtered, opp)
		}
	}
	
	return filtered
}

// matchesAdvancedCriteria checks if opportunity matches advanced filters
func (m *Monitor) matchesAdvancedCriteria(opp samgov.Opportunity, advanced config.AdvancedQuery) bool {
	// Check include keywords
	if len(advanced.Include) > 0 {
		found := false
		for _, keyword := range advanced.Include {
			if containsIgnoreCase(opp.Title, keyword) || containsIgnoreCase(opp.Description, keyword) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check exclude keywords
	for _, keyword := range advanced.Exclude {
		if containsIgnoreCase(opp.Title, keyword) || containsIgnoreCase(opp.Description, keyword) {
			return false
		}
	}

	// Check age limit
	if advanced.MaxDaysOld > 0 {
		if postedDate, err := time.Parse("2006-01-02", opp.PostedDate); err == nil {
			daysSince := int(time.Since(postedDate).Hours() / 24)
			if daysSince > advanced.MaxDaysOld {
				return false
			}
		}
	}

	// Check NAICS codes
	if len(advanced.NAICSCodes) > 0 {
		found := false
		for _, code := range advanced.NAICSCodes {
			if opp.NAICSCode == code {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check set-aside types
	if len(advanced.SetAsideTypes) > 0 {
		found := false
		for _, setAside := range advanced.SetAsideTypes {
			if opp.TypeOfSetAside == setAside {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// diffOpportunities compares current opportunities with state
func (m *Monitor) diffOpportunities(current []samgov.Opportunity) samgov.DiffResult {
	diff := samgov.DiffResult{
		New:      make([]samgov.Opportunity, 0),
		Updated:  make([]samgov.Opportunity, 0),
		Existing: make([]samgov.Opportunity, 0),
	}

	for _, opp := range current {
		if previous, exists := m.state.GetOpportunity(opp.NoticeID); exists {
			if m.hasOpportunityChanged(previous, opp) {
				diff.Updated = append(diff.Updated, opp)
			} else {
				diff.Existing = append(diff.Existing, opp)
			}
		} else {
			diff.New = append(diff.New, opp)
		}
	}

	return diff
}

// hasOpportunityChanged determines if an opportunity has been modified
func (m *Monitor) hasOpportunityChanged(previous samgov.OpportunityState, current samgov.Opportunity) bool {
	// Simple comparison - in production you might want to hash the content
	return previous.Title != current.Title ||
		   (previous.Deadline == nil && current.ResponseDeadline != nil) ||
		   (previous.Deadline != nil && current.ResponseDeadline == nil) ||
		   (previous.Deadline != nil && current.ResponseDeadline != nil && *previous.Deadline != *current.ResponseDeadline)
}

// logReport prints the monitoring run report
func (m *Monitor) logReport(report *RunReport) {
	log.Printf("=== Monitoring Run Complete ===")
	log.Printf("Duration: %v", report.Duration)
	log.Printf("Queries: %d run, %d succeeded, %d failed", report.QueriesRun, report.QueriesSucceded, report.QueriesFailed)
	log.Printf("Opportunities: %d total, %d new, %d updated", report.TotalOpps, report.NewOpps, report.UpdatedOpps)
	
	if report.Notifications > 0 {
		log.Printf("Notifications: %d sent", report.Notifications)
	}
	
	if len(report.Errors) > 0 {
		log.Printf("Errors: %d", len(report.Errors))
		for _, err := range report.Errors {
			log.Printf("  - %s", err)
		}
	}
	
	if m.verbose {
		log.Printf("Query execution times:")
		for _, result := range report.QueryResults {
			status := "✓"
			if result.Error != nil {
				status = "✗"
			}
			log.Printf("  %s %s: %v (%d opportunities)", status, result.QueryName, result.ExecutionTime, len(result.Opportunities))
		}
	}
}

// containsIgnoreCase performs case-insensitive substring search
func containsIgnoreCase(text, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(text) == 0 {
		return false
	}
	
	// Simple case-insensitive search
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

// buildNotificationConfig creates notification configuration from environment variables
func buildNotificationConfig() notify.NotificationConfig {
	config := notify.NotificationConfig{}

	// Email configuration
	config.Email = notify.EmailConfig{
		Enabled:     os.Getenv("SMTP_HOST") != "",
		SMTPHost:    os.Getenv("SMTP_HOST"),
		SMTPPort:    getEnvInt("SMTP_PORT", 587),
		Username:    os.Getenv("SMTP_USERNAME"),
		Password:    os.Getenv("SMTP_PASSWORD"),
		FromAddress: os.Getenv("EMAIL_FROM"),
		ToAddresses: getEnvStringSlice("EMAIL_TO"),
		UseTLS:      getEnvBool("SMTP_USE_TLS", true),
	}

	// Slack configuration
	config.Slack = notify.SlackConfig{
		Enabled:    os.Getenv("SLACK_WEBHOOK") != "",
		WebhookURL: os.Getenv("SLACK_WEBHOOK"),
		Channel:    os.Getenv("SLACK_CHANNEL"),
		Username:   os.Getenv("SLACK_USERNAME"),
		IconEmoji:  os.Getenv("SLACK_ICON_EMOJI"),
	}

	// GitHub configuration
	config.GitHub = notify.GitHubConfig{
		Enabled:     os.Getenv("GITHUB_TOKEN") != "",
		Token:       os.Getenv("GITHUB_TOKEN"),
		Owner:       getEnvWithDefault("GITHUB_OWNER", "yourusername"),
		Repository:  getEnvWithDefault("GITHUB_REPOSITORY", "sam-gov-monitor"),
		Labels:      getEnvStringSlice("GITHUB_LABELS"),
		AssignUsers: getEnvStringSlice("GITHUB_ASSIGN_USERS"),
	}

	return config
}

// sendNotifications sends notifications for opportunities
func (m *Monitor) sendNotifications(ctx context.Context, query config.Query, diff samgov.DiffResult) error {
	// Send notifications for new opportunities
	if len(diff.New) > 0 {
		if err := m.sendNewOpportunityNotifications(ctx, query, diff.New); err != nil {
			return fmt.Errorf("sending new opportunity notifications: %w", err)
		}
	}

	// Send notifications for updated opportunities
	if len(diff.Updated) > 0 {
		if err := m.sendUpdatedOpportunityNotifications(ctx, query, diff.Updated); err != nil {
			return fmt.Errorf("sending updated opportunity notifications: %w", err)
		}
	}

	return nil
}

// sendNewOpportunityNotifications sends notifications for new opportunities
func (m *Monitor) sendNewOpportunityNotifications(ctx context.Context, query config.Query, opportunities []samgov.Opportunity) error {
	priority := notify.Priority(query.Notification.Priority)
	if priority == "" {
		priority = notify.PriorityMedium
	}

	subject := fmt.Sprintf("🚨 %d New SAM.gov Opportunities - %s", len(opportunities), query.Name)

	// Build notification
	notification := notify.NewNotificationBuilder().
		WithQuery(query.Name, priority).
		WithRecipients(query.Notification.Recipients).
		WithOpportunities(opportunities).
		WithSubject(subject).
		WithMetadata("query_type", "new").
		Build()

	// Add calendar attachment if there are deadlines
	if m.hasDeadlines(opportunities) {
		calGen := notify.NewCalendarGenerator(m.verbose)
		attachment := calGen.CreateCalendarAttachment(opportunities, query.Name)
		notification.Attachments = []notify.Attachment{attachment}
	}

	return m.notifyMgr.SendNotification(ctx, notification)
}

// sendUpdatedOpportunityNotifications sends notifications for updated opportunities
func (m *Monitor) sendUpdatedOpportunityNotifications(ctx context.Context, query config.Query, opportunities []samgov.Opportunity) error {
	priority := notify.Priority(query.Notification.Priority)
	if priority == "" {
		priority = notify.PriorityMedium
	}

	subject := fmt.Sprintf("🔄 %d Updated SAM.gov Opportunities - %s", len(opportunities), query.Name)

	// Build notification
	notification := notify.NewNotificationBuilder().
		WithQuery(query.Name, priority).
		WithRecipients(query.Notification.Recipients).
		WithUpdatedOpportunities(opportunities).
		WithSubject(subject).
		WithMetadata("query_type", "updated").
		Build()

	return m.notifyMgr.SendNotification(ctx, notification)
}

// findQueryByName finds a query configuration by name
func (m *Monitor) findQueryByName(name string) *config.Query {
	for _, query := range m.config.Queries {
		if query.Name == name {
			return &query
		}
	}
	return nil
}

// hasDeadlines checks if any opportunities have response deadlines
func (m *Monitor) hasDeadlines(opportunities []samgov.Opportunity) bool {
	for _, opp := range opportunities {
		if opp.ResponseDeadline != nil && *opp.ResponseDeadline != "" {
			return true
		}
	}
	return false
}

// Helper functions for environment variable parsing

func getEnvInt(key string, defaultValue int) int {
	if val := os.Getenv(key); val != "" {
		if parsed, err := parseInt(val); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if val := os.Getenv(key); val != "" {
		return val == "true" || val == "1" || val == "yes"
	}
	return defaultValue
}

func getEnvWithDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

func getEnvStringSlice(key string) []string {
	if val := os.Getenv(key); val != "" {
		// Split by comma and trim spaces
		parts := strings.Split(val, ",")
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	}
	return nil
}

// Simple integer parsing to avoid importing strconv
func parseInt(s string) (int, error) {
	result := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid integer: %s", s)
		}
		result = result*10 + int(r-'0')
	}
	return result, nil
}