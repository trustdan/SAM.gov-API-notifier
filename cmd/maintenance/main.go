package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yourusername/sam-gov-monitor/internal/config"
	"github.com/yourusername/sam-gov-monitor/internal/samgov"
)

func main() {
	var (
		task       = flag.String("task", "", "Maintenance task to run")
		ageDays    = flag.Int("age-days", 30, "Age in days for cleanup operations")
		force      = flag.Bool("force", false, "Force operations even with warnings")
		output     = flag.String("output", "", "Output file path for reports")
		verbose    = flag.Bool("v", false, "Verbose output")
		configPath = flag.String("config", "config/queries.yaml", "Configuration file path")
	)
	flag.Parse()

	if *task == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s -task <task-name> [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nAvailable tasks:\n")
		fmt.Fprintf(os.Stderr, "  cleanup-state      Clean old state files\n")
		fmt.Fprintf(os.Stderr, "  security-audit     Run security audit\n")
		fmt.Fprintf(os.Stderr, "  generate-report    Generate maintenance report\n")
		fmt.Fprintf(os.Stderr, "  optimize-cache     Optimize cache and state\n")
		fmt.Fprintf(os.Stderr, "  health-check       Run system health checks\n")
		os.Exit(1)
	}

	ctx := context.Background()
	logger := log.New(os.Stdout, "[maintenance] ", log.LstdFlags)

	if *verbose {
		logger.Printf("Starting maintenance task: %s", *task)
	}

	switch *task {
	case "cleanup-state":
		if err := cleanupState(*ageDays, *force, *verbose, logger); err != nil {
			logger.Fatalf("State cleanup failed: %v", err)
		}
	case "security-audit":
		if err := securityAudit(*verbose, logger); err != nil {
			logger.Fatalf("Security audit failed: %v", err)
		}
	case "generate-report":
		if err := generateReport(*output, *configPath, *verbose, logger); err != nil {
			logger.Fatalf("Report generation failed: %v", err)
		}
	case "optimize-cache":
		if err := optimizeCache(*verbose, logger); err != nil {
			logger.Fatalf("Cache optimization failed: %v", err)
		}
	case "health-check":
		if err := healthCheck(ctx, *configPath, *verbose, logger); err != nil {
			logger.Fatalf("Health check failed: %v", err)
		}
	default:
		logger.Fatalf("Unknown task: %s", *task)
	}

	if *verbose {
		logger.Printf("Maintenance task completed successfully")
	}
}

func cleanupState(ageDays int, force bool, verbose bool, logger *log.Logger) error {
	stateDir := "state"
	cutoff := time.Now().AddDate(0, 0, -ageDays)

	if verbose {
		logger.Printf("Cleaning state files older than %d days (before %s)", ageDays, cutoff.Format("2006-01-02"))
	}

	if _, err := os.Stat(stateDir); os.IsNotExist(err) {
		if verbose {
			logger.Printf("State directory does not exist, creating: %s", stateDir)
		}
		return os.MkdirAll(stateDir, 0755)
	}

	var removed int
	err := filepath.Walk(stateDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Keep current state files
		if strings.HasSuffix(path, "_current.json") && !force {
			return nil
		}

		if info.ModTime().Before(cutoff) {
			if verbose {
				logger.Printf("Removing old state file: %s (modified: %s)", path, info.ModTime().Format("2006-01-02"))
			}
			if err := os.Remove(path); err != nil {
				logger.Printf("Failed to remove %s: %v", path, err)
			} else {
				removed++
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk state directory: %w", err)
	}

	logger.Printf("Cleanup completed: removed %d old state files", removed)
	return nil
}

func securityAudit(verbose bool, logger *log.Logger) error {
	if verbose {
		logger.Printf("Running security audit...")
	}

	// Check environment variables for sensitive data
	sensitiveVars := []string{"SAM_API_KEY", "SMTP_PASSWORD", "SLACK_WEBHOOK"}
	issues := 0

	for _, varName := range sensitiveVars {
		value := os.Getenv(varName)
		if value == "" {
			logger.Printf("WARNING: Required environment variable %s is not set", varName)
			issues++
			continue
		}

		// Check for common security issues
		if len(value) < 10 {
			logger.Printf("WARNING: %s appears to be too short (possible test value)", varName)
			issues++
		}

		if strings.Contains(strings.ToLower(value), "test") || 
		   strings.Contains(strings.ToLower(value), "example") ||
		   strings.Contains(strings.ToLower(value), "dummy") {
			logger.Printf("WARNING: %s contains test/example patterns", varName)
			issues++
		}
	}

	// Check file permissions
	configFiles := []string{"config/queries.yaml", "state/", "logs/"}
	for _, file := range configFiles {
		if info, err := os.Stat(file); err == nil {
			mode := info.Mode()
			if mode&0077 != 0 { // World or group writable
				logger.Printf("WARNING: %s has overly permissive permissions: %o", file, mode)
				issues++
			}
		}
	}

	if issues > 0 {
		logger.Printf("Security audit completed with %d issues found", issues)
		return fmt.Errorf("security audit found %d issues", issues)
	}

	logger.Printf("Security audit passed: no issues found")
	return nil
}

func generateReport(outputPath, configPath string, verbose bool, logger *log.Logger) error {
	if verbose {
		logger.Printf("Generating maintenance report...")
	}

	report := struct {
		Timestamp   time.Time `json:"timestamp"`
		SystemInfo  SystemInfo `json:"system_info"`
		StateInfo   StateInfo `json:"state_info"`
		ConfigInfo  ConfigInfo `json:"config_info"`
		Recommendations []string `json:"recommendations"`
	}{
		Timestamp: time.Now(),
		Recommendations: []string{},
	}

	// Gather system information
	report.SystemInfo = SystemInfo{
		GoVersion:   os.Getenv("GO_VERSION"),
		Environment: getEnvironmentInfo(),
		Uptime:      time.Since(time.Now().Truncate(24 * time.Hour)).String(),
	}

	// Gather state information
	stateInfo, err := getStateInfo()
	if err != nil {
		logger.Printf("Warning: Could not gather state info: %v", err)
	} else {
		report.StateInfo = stateInfo
	}

	// Gather configuration information
	configInfo, err := getConfigInfo(configPath)
	if err != nil {
		logger.Printf("Warning: Could not gather config info: %v", err)
	} else {
		report.ConfigInfo = configInfo
	}

	// Generate recommendations
	if report.StateInfo.TotalFiles > 100 {
		report.Recommendations = append(report.Recommendations, 
			"Consider increasing cleanup frequency - high number of state files")
	}
	if len(report.ConfigInfo.Queries) > 20 {
		report.Recommendations = append(report.Recommendations, 
			"Consider optimizing queries - high query count may impact performance")
	}

	// Output report
	if outputPath == "" {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Write as markdown
	fmt.Fprintf(file, "# Maintenance Report\n\n")
	fmt.Fprintf(file, "**Generated**: %s\n\n", report.Timestamp.Format("2006-01-02 15:04:05 UTC"))
	
	fmt.Fprintf(file, "## System Information\n")
	fmt.Fprintf(file, "- Go Version: %s\n", report.SystemInfo.GoVersion)
	fmt.Fprintf(file, "- Environment Variables: %d configured\n", len(report.SystemInfo.Environment))
	
	fmt.Fprintf(file, "\n## State Information\n")
	fmt.Fprintf(file, "- Total State Files: %d\n", report.StateInfo.TotalFiles)
	fmt.Fprintf(file, "- Total Size: %d bytes\n", report.StateInfo.TotalSize)
	fmt.Fprintf(file, "- Last Modified: %s\n", report.StateInfo.LastModified.Format("2006-01-02 15:04:05"))
	
	fmt.Fprintf(file, "\n## Configuration\n")
	fmt.Fprintf(file, "- Query Count: %d\n", len(report.ConfigInfo.Queries))
	fmt.Fprintf(file, "- Email Recipients: %d\n", report.ConfigInfo.EmailRecipients)
	
	if len(report.Recommendations) > 0 {
		fmt.Fprintf(file, "\n## Recommendations\n")
		for _, rec := range report.Recommendations {
			fmt.Fprintf(file, "- %s\n", rec)
		}
	}

	logger.Printf("Maintenance report saved to: %s", outputPath)
	return nil
}

func optimizeCache(verbose bool, logger *log.Logger) error {
	if verbose {
		logger.Printf("Optimizing cache and state files...")
	}

	// Consolidate state files
	stateDir := "state"
	if _, err := os.Stat(stateDir); os.IsNotExist(err) {
		return os.MkdirAll(stateDir, 0755)
	}

	// Remove duplicate state files
	files, err := filepath.Glob(filepath.Join(stateDir, "*.json"))
	if err != nil {
		return fmt.Errorf("failed to list state files: %w", err)
	}

	duplicates := 0
	seen := make(map[string]string)

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		hash := fmt.Sprintf("%x", content)
		if existing, exists := seen[hash]; exists {
			if verbose {
				logger.Printf("Removing duplicate state file: %s (duplicate of %s)", file, existing)
			}
			os.Remove(file)
			duplicates++
		} else {
			seen[hash] = file
		}
	}

	logger.Printf("Cache optimization completed: removed %d duplicate files", duplicates)
	return nil
}

func healthCheck(ctx context.Context, configPath string, verbose bool, logger *log.Logger) error {
	if verbose {
		logger.Printf("Running system health check...")
	}

	results := []string{}

	// Test SAM.gov API connectivity
	client := samgov.NewClient(os.Getenv("SAM_API_KEY"))
	params := map[string]string{"limit": "1"}
	if _, err := client.Search(ctx, params); err != nil {
		results = append(results, fmt.Sprintf("❌ SAM.gov API: %v", err))
	} else {
		results = append(results, "✅ SAM.gov API: Connected")
	}

	// Test configuration loading
	if _, err := config.Load(configPath); err != nil {
		results = append(results, fmt.Sprintf("❌ Configuration: %v", err))
	} else {
		results = append(results, "✅ Configuration: Valid")
	}

	// Check required environment variables
	requiredVars := []string{"SAM_API_KEY", "EMAIL_FROM", "EMAIL_TO"}
	missingVars := []string{}
	for _, varName := range requiredVars {
		if os.Getenv(varName) == "" {
			missingVars = append(missingVars, varName)
		}
	}

	if len(missingVars) > 0 {
		results = append(results, fmt.Sprintf("❌ Environment: Missing %v", missingVars))
	} else {
		results = append(results, "✅ Environment: All required variables set")
	}

	// Check disk space
	if stateInfo, err := getStateInfo(); err == nil {
		if stateInfo.TotalSize > 100*1024*1024 { // 100MB
			results = append(results, "⚠️ Disk Usage: State files are large, consider cleanup")
		} else {
			results = append(results, "✅ Disk Usage: Normal")
		}
	}

	// Output results
	for _, result := range results {
		fmt.Println(result)
	}

	// Return error if any critical checks failed
	for _, result := range results {
		if strings.HasPrefix(result, "❌") {
			return fmt.Errorf("health check failed")
		}
	}

	fmt.Println("✅ Overall Health: All systems operational")
	return nil
}

// Helper types and functions
type SystemInfo struct {
	GoVersion   string            `json:"go_version"`
	Environment map[string]string `json:"environment"`
	Uptime      string            `json:"uptime"`
}

type StateInfo struct {
	TotalFiles   int       `json:"total_files"`
	TotalSize    int64     `json:"total_size"`
	LastModified time.Time `json:"last_modified"`
}

type ConfigInfo struct {
	Queries         []string `json:"queries"`
	EmailRecipients int      `json:"email_recipients"`
}

func getEnvironmentInfo() map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		if len(pair) == 2 {
			// Don't include sensitive values in reports
			if strings.Contains(strings.ToUpper(pair[0]), "PASSWORD") ||
			   strings.Contains(strings.ToUpper(pair[0]), "KEY") ||
			   strings.Contains(strings.ToUpper(pair[0]), "SECRET") {
				env[pair[0]] = "[REDACTED]"
			} else {
				env[pair[0]] = pair[1]
			}
		}
	}
	return env
}

func getStateInfo() (StateInfo, error) {
	info := StateInfo{}
	stateDir := "state"

	err := filepath.Walk(stateDir, func(path string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !fileInfo.IsDir() {
			info.TotalFiles++
			info.TotalSize += fileInfo.Size()
			if fileInfo.ModTime().After(info.LastModified) {
				info.LastModified = fileInfo.ModTime()
			}
		}
		return nil
	})

	return info, err
}

func getConfigInfo(configPath string) (ConfigInfo, error) {
	info := ConfigInfo{}
	
	cfg, err := config.Load(configPath)
	if err != nil {
		return info, err
	}

	info.Queries = make([]string, len(cfg.Queries))
	for i, query := range cfg.Queries {
		info.Queries[i] = query.Name
	}

	// Count email recipients
	emailTo := os.Getenv("EMAIL_TO")
	if emailTo != "" {
		info.EmailRecipients = len(strings.Split(emailTo, ","))
	}

	return info, nil
}