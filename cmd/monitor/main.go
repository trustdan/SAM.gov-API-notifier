package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/yourusername/sam-gov-monitor/internal/config"
	"github.com/yourusername/sam-gov-monitor/internal/monitor"
)

const (
	DefaultConfigPath = "config/queries.yaml"
	DefaultStateFile  = "state/monitor.json"
	DefaultLookback   = 3 // days
	Version           = "1.0.0"
	BuildDate         = "2025-01-29"
)

func main() {
	var (
		configPath  = flag.String("config", DefaultConfigPath, "Path to config file")
		stateFile   = flag.String("state", DefaultStateFile, "Path to state file")
		dryRun      = flag.Bool("dry-run", false, "Run without sending notifications")
		verbose     = flag.Bool("v", false, "Verbose output")
		validateEnv = flag.Bool("validate-env", false, "Validate environment and exit")
		showHelp    = flag.Bool("help", false, "Show help")
		lookback    = flag.Int("lookback", DefaultLookback, "Days to look back for opportunities")
		showVersion = flag.Bool("version", false, "Show version information")
		reportMode  = flag.Bool("report", false, "Generate status report from state file")
		debugEmail  = flag.Bool("debug-email", false, "Send test email every run")
	)
	flag.Parse()

	if *showHelp {
		showUsage()
		return
	}

	if *showVersion {
		fmt.Printf("SAM.gov Monitor v%s (Built: %s)\n", Version, BuildDate)
		return
	}

	if *reportMode {
		if err := generateReport(*stateFile); err != nil {
			log.Fatalf("Failed to generate report: %v", err)
		}
		return
	}

	// Setup logging
	if *verbose {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	} else {
		log.SetFlags(log.LstdFlags)
	}

	log.Printf("Starting SAM.gov Monitor")
	
	// Warn about API limits for non-federal accounts
	if os.Getenv("SAM_ACCOUNT_TYPE") == "" {
		log.Printf("WARNING: Non-federal accounts are limited to 10 API requests per day!")
		log.Printf("Set SAM_ACCOUNT_TYPE=federal if you have a federal account with higher limits")
	}
	
	if *dryRun {
		log.Printf("Running in DRY-RUN mode - no notifications will be sent")
	}

	// Validate environment
	if err := validateEnvironment(); err != nil {
		log.Fatalf("Environment validation failed: %v", err)
	}

	if *validateEnv {
		log.Printf("Environment validation passed")
		return
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Loaded %d queries (%d enabled)", len(cfg.Queries), len(cfg.GetEnabledQueries()))

	// Initialize and run monitor
	apiKey := os.Getenv("SAM_API_KEY")

	// Create monitor
	m, err := monitor.New(monitor.Options{
		APIKey:       apiKey,
		Config:       cfg,
		StateFile:    *stateFile,
		Verbose:      *verbose,
		DryRun:       *dryRun,
		LookbackDays: *lookback,
		DebugEmail:   *debugEmail,
	})
	if err != nil {
		log.Fatalf("Failed to create monitor: %v", err)
	}

	// Run monitoring
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if *verbose {
		log.Printf("Starting monitoring run...")
	}

	if err := m.Run(ctx); err != nil {
		log.Fatalf("Monitor run failed: %v", err)
	}

	log.Printf("Monitor completed successfully")
}

func validateEnvironment() error {
	required := []string{
		"SAM_API_KEY",
	}

	optional := []string{
		"SMTP_HOST",
		"SMTP_PORT", 
		"SMTP_USERNAME",
		"SMTP_PASSWORD",
		"EMAIL_FROM",
		"EMAIL_TO",
		"SLACK_WEBHOOK",
	}

	missing := []string{}
	for _, env := range required {
		if val := os.Getenv(env); val == "" {
			missing = append(missing, env)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	// Check for test values
	for _, env := range required {
		if val := os.Getenv(env); strings.Contains(strings.ToLower(val), "test") || 
			strings.Contains(strings.ToLower(val), "example") {
			return fmt.Errorf("environment variable %s appears to contain test data", env)
		}
	}

	// Log optional variables that are set
	optionalSet := []string{}
	for _, env := range optional {
		if val := os.Getenv(env); val != "" {
			optionalSet = append(optionalSet, env)
		}
	}

	if len(optionalSet) > 0 {
		log.Printf("Optional environment variables set: %s", strings.Join(optionalSet, ", "))
	}

	return nil
}


func showUsage() {
	fmt.Printf(`SAM.gov Opportunity Monitor v%s

Usage: %s [options]

Options:
  -config string
        Path to config file (default "%s")
  -state string  
        Path to state file (default "%s")
  -dry-run
        Run without sending notifications
  -v    Verbose output
  -validate-env
        Validate environment and exit
  -lookback int
        Days to look back for opportunities (default %d)
  -version
        Show version information
  -report
        Generate status report from state file
  -debug-email
        Send test email every run
  -help Show this help

Environment Variables:
  SAM_API_KEY      Required - SAM.gov API key
  SMTP_HOST        Optional - SMTP server host
  SMTP_PORT        Optional - SMTP server port  
  SMTP_USERNAME    Optional - SMTP username
  SMTP_PASSWORD    Optional - SMTP password
  EMAIL_FROM       Optional - Sender email address
  EMAIL_TO         Optional - Recipient email addresses
  SLACK_WEBHOOK    Optional - Slack webhook URL

Examples:
  %s -config myconfig.yaml -dry-run -v
  %s -validate-env
  %s -lookback 7 -v

`, Version, os.Args[0], DefaultConfigPath, DefaultStateFile, DefaultLookback, 
   os.Args[0], os.Args[0], os.Args[0])
}

// generateReport creates a status report from the state file
func generateReport(stateFile string) error {
	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("State file not found: %s\n", stateFile)
			fmt.Printf("No monitoring runs have been completed yet.\n")
			return nil
		}
		return fmt.Errorf("reading state file: %w", err)
	}

	var state struct {
		Opportunities map[string]interface{} `json:"opportunities"`
		LastRun       time.Time              `json:"last_run"`
		RunCount      int                    `json:"run_count"`
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("parsing state file: %w", err)
	}

	fmt.Printf("# SAM.gov Monitor Status Report\n\n")
	fmt.Printf("Generated: %s\n", time.Now().Format(time.RFC3339))
	fmt.Printf("State File: %s\n\n", stateFile)
	
	fmt.Printf("## Summary\n")
	fmt.Printf("- Total Opportunities Tracked: %d\n", len(state.Opportunities))
	fmt.Printf("- Total Monitor Runs: %d\n", state.RunCount)
	
	if !state.LastRun.IsZero() {
		fmt.Printf("- Last Run: %s (%s ago)\n", 
			state.LastRun.Format(time.RFC3339),
			time.Since(state.LastRun).Round(time.Minute))
	} else {
		fmt.Printf("- Last Run: Never\n")
	}

	fmt.Printf("\n## Recent Activity\n")
	if len(state.Opportunities) == 0 {
		fmt.Printf("No opportunities tracked yet.\n")
	} else {
		fmt.Printf("Tracking %d unique opportunities\n", len(state.Opportunities))
	}

	return nil
}