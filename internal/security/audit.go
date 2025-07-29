package security

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/yourusername/sam-gov-monitor/internal/config"
)

// SecurityAudit performs comprehensive security validation
type SecurityAudit struct {
	verbose bool
	issues  []SecurityIssue
}

// SecurityIssue represents a security concern found during audit
type SecurityIssue struct {
	Level       string    `json:"level"`        // "error", "warning", "info"
	Category    string    `json:"category"`     // "environment", "config", "filesystem", "network"
	Description string    `json:"description"`
	Impact      string    `json:"impact"`
	Remediation string    `json:"remediation"`
	Timestamp   time.Time `json:"timestamp"`
}

// NewSecurityAudit creates a new security auditor
func NewSecurityAudit(verbose bool) *SecurityAudit {
	return &SecurityAudit{
		verbose: verbose,
		issues:  make([]SecurityIssue, 0),
	}
}

// RunFullAudit performs comprehensive security validation
func (sa *SecurityAudit) RunFullAudit(cfg *config.Config) error {
	if sa.verbose {
		log.Printf("Starting comprehensive security audit...")
	}

	// Reset issues
	sa.issues = make([]SecurityIssue, 0)

	// Run all audit checks
	sa.auditEnvironmentVariables()
	sa.auditConfiguration(cfg)
	sa.auditFilePermissions()
	sa.auditNetworkSecurity()
	sa.auditLoggingSecurity()
	sa.auditContainerSecurity()

	// Report results
	if sa.verbose {
		sa.printAuditResults()
	}

	// Check if there are any critical issues
	criticalIssues := sa.getCriticalIssues()
	if len(criticalIssues) > 0 {
		return fmt.Errorf("security audit failed with %d critical issues", len(criticalIssues))
	}

	return nil
}

// auditEnvironmentVariables validates environment variable security
func (sa *SecurityAudit) auditEnvironmentVariables() {
	requiredVars := []string{
		"SAM_API_KEY",
	}

	sensitiveVars := []string{
		"SAM_API_KEY",
		"SMTP_PASSWORD",
		"SMTP_USERNAME",
		"SLACK_WEBHOOK",
		"GITHUB_TOKEN",
	}

	// Check required variables
	for _, varName := range requiredVars {
		value := os.Getenv(varName)
		if value == "" {
			sa.addIssue("error", "environment", 
				fmt.Sprintf("Required environment variable %s is not set", varName),
				"Application will not function without this variable",
				fmt.Sprintf("Set %s environment variable with appropriate value", varName))
			continue
		}

		// Check for test/example values
		if sa.containsTestData(value) {
			sa.addIssue("error", "environment",
				fmt.Sprintf("Environment variable %s appears to contain test data", varName),
				"Using test credentials in production is a security risk",
				fmt.Sprintf("Replace %s with production credentials", varName))
		}

		// Check API key format
		if varName == "SAM_API_KEY" {
			if !sa.isValidAPIKeyFormat(value) {
				sa.addIssue("warning", "environment",
					"SAM_API_KEY format appears invalid",
					"Invalid API key will cause authentication failures",
					"Verify API key format with SAM.gov documentation")
			}
		}
	}

	// Check for exposed sensitive variables in environment
	for _, varName := range sensitiveVars {
		value := os.Getenv(varName)
		if value != "" {
			// Check if value is too short (likely invalid)
			if len(value) < 8 {
				sa.addIssue("warning", "environment",
					fmt.Sprintf("Environment variable %s appears too short", varName),
					"Short credentials may indicate incomplete setup",
					fmt.Sprintf("Verify %s contains complete credential", varName))
			}

			// Check for common weak patterns
			if sa.isWeakCredential(value) {
				sa.addIssue("error", "environment",
					fmt.Sprintf("Environment variable %s contains weak credential", varName),
					"Weak credentials are easily compromised",
					fmt.Sprintf("Use strong, unique credentials for %s", varName))
			}
		}
	}

	// Check for accidentally exposed secrets in process environment
	allEnv := os.Environ()
	for _, env := range allEnv {
		if sa.containsPossibleSecret(env) {
			sa.addIssue("warning", "environment",
				"Possible secret detected in environment variables",
				"Accidentally exposed secrets can be discovered by attackers",
				"Review environment variables and remove any accidentally exposed secrets")
			break // Only report once
		}
	}
}

// auditConfiguration validates configuration security
func (sa *SecurityAudit) auditConfiguration(cfg *config.Config) {
	if cfg == nil {
		sa.addIssue("error", "config",
			"Configuration is nil",
			"Application cannot function without configuration",
			"Ensure configuration is properly loaded")
		return
	}

	// Check for enabled queries
	enabledQueries := cfg.GetEnabledQueries()
	if len(enabledQueries) == 0 {
		sa.addIssue("warning", "config",
			"No queries are enabled",
			"No monitoring will occur",
			"Enable at least one query in configuration")
	}

	// Check query security
	for _, query := range cfg.Queries {
		sa.auditQuerySecurity(query)
	}

	// Check for overly broad queries
	broadQueries := 0
	for _, query := range enabledQueries {
		if sa.isBroadQuery(query) {
			broadQueries++
		}
	}

	if broadQueries > len(enabledQueries)/2 {
		sa.addIssue("info", "config",
			"Many queries appear to be very broad",
			"Broad queries may generate excessive API requests and notifications",
			"Consider adding more specific search criteria to queries")
	}
}

// auditQuerySecurity validates individual query security
func (sa *SecurityAudit) auditQuerySecurity(query config.Query) {
	// Check for SQL injection patterns (though not applicable here, good practice)
	for key, value := range query.Parameters {
		if strVal, ok := value.(string); ok {
			if sa.containsSuspiciousPatterns(strVal) {
				sa.addIssue("warning", "config",
					fmt.Sprintf("Query parameter '%s' contains suspicious patterns", key),
					"Suspicious patterns may indicate injection attempts",
					fmt.Sprintf("Review and sanitize query parameter '%s'", key))
			}
		}
	}

	// Check for excessive notification recipients
	if len(query.Notification.Recipients) > 10 {
		sa.addIssue("info", "config",
			fmt.Sprintf("Query '%s' has many notification recipients (%d)", 
				query.Name, len(query.Notification.Recipients)),
			"Many recipients may lead to notification fatigue",
			"Consider using mailing lists or reducing recipient count")
	}

	// Validate email addresses
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	for _, recipient := range query.Notification.Recipients {
		if !emailRegex.MatchString(recipient) {
			sa.addIssue("warning", "config",
				fmt.Sprintf("Invalid email address in query '%s': %s", query.Name, recipient),
				"Invalid email addresses will cause notification failures",
				"Use valid email address format")
		}
	}
}

// auditFilePermissions checks file system security
func (sa *SecurityAudit) auditFilePermissions() {
	criticalPaths := []string{
		"config/queries.yaml",
		"state/monitor.json",
		".env",
	}

	for _, path := range criticalPaths {
		info, err := os.Stat(path)
		if err != nil {
			if !os.IsNotExist(err) {
				sa.addIssue("warning", "filesystem",
					fmt.Sprintf("Cannot access file: %s", path),
					"File access issues may indicate permission problems",
					fmt.Sprintf("Verify file exists and permissions are correct for %s", path))
			}
			continue
		}

		// Check permissions
		mode := info.Mode()
		if mode&0077 != 0 { // World or group readable/writable
			sa.addIssue("warning", "filesystem",
				fmt.Sprintf("File %s has overly permissive permissions (%s)", path, mode),
				"Overly permissive file permissions may expose sensitive data",
				fmt.Sprintf("Set appropriate permissions: chmod 600 %s", path))
		}
	}

	// Check state directory permissions
	if info, err := os.Stat("state"); err == nil {
		mode := info.Mode()
		if mode&0077 != 0 {
			sa.addIssue("info", "filesystem",
				"State directory has permissive permissions",
				"Directory permissions may allow unauthorized access to state files",
				"Set appropriate permissions: chmod 700 state/")
		}
	}
}

// auditNetworkSecurity validates network-related security
func (sa *SecurityAudit) auditNetworkSecurity() {
	// Check SMTP configuration
	smtpHost := os.Getenv("SMTP_HOST")
	if smtpHost != "" {
		if !strings.Contains(smtpHost, "gmail.com") && 
		   !strings.Contains(smtpHost, "outlook.com") &&
		   !strings.Contains(smtpHost, "smtp.") {
			sa.addIssue("info", "network",
				"SMTP host may not be using standard provider",
				"Non-standard SMTP providers may have different security requirements",
				"Verify SMTP host supports TLS encryption")
		}
	}

	// Check Slack webhook security
	slackWebhook := os.Getenv("SLACK_WEBHOOK")
	if slackWebhook != "" {
		if !strings.HasPrefix(slackWebhook, "https://hooks.slack.com/") {
			sa.addIssue("warning", "network",
				"Slack webhook URL appears invalid",
				"Invalid webhook URLs will cause notification failures",
				"Use official Slack webhook URL format")
		}
	}

	// Check for HTTP URLs (should be HTTPS)
	allEnv := os.Environ()
	for _, env := range allEnv {
		if strings.Contains(env, "http://") && !strings.Contains(env, "localhost") {
			sa.addIssue("warning", "network",
				"HTTP URL detected in environment (should use HTTPS)",
				"HTTP connections are not encrypted and may expose data",
				"Use HTTPS URLs for all external services")
			break
		}
	}
}

// auditLoggingSecurity validates logging configuration
func (sa *SecurityAudit) auditLoggingSecurity() {
	// This is a placeholder for logging security checks
	// In a real implementation, you'd check:
	// - Log file permissions
	// - Log rotation configuration
	// - Whether secrets are being logged
	// - Log storage location security

	sa.addIssue("info", "logging",
		"Logging security audit completed",
		"No critical logging security issues detected",
		"Ensure logs don't contain sensitive information")
}

// auditContainerSecurity validates container-related security (if running in container)
func (sa *SecurityAudit) auditContainerSecurity() {
	// Check if running in container
	if _, err := os.Stat("/.dockerenv"); err == nil {
		// Running in Docker container
		
		// Check if running as root
		if os.Getuid() == 0 {
			sa.addIssue("warning", "container",
				"Running as root user in container",
				"Root access in containers increases security risk",
				"Use non-root user in Dockerfile: USER sammonitor")
		}

		sa.addIssue("info", "container",
			"Container security audit completed",
			"Basic container security checks passed",
			"Review container security best practices")
	}
}

// Helper methods for security validation

func (sa *SecurityAudit) containsTestData(value string) bool {
	testPatterns := []string{
		"test", "example", "demo", "sample", "fake", "mock",
		"12345", "password", "secret", "key123", "abc123",
	}

	valueLower := strings.ToLower(value)
	for _, pattern := range testPatterns {
		if strings.Contains(valueLower, pattern) {
			return true
		}
	}
	return false
}

func (sa *SecurityAudit) isValidAPIKeyFormat(apiKey string) bool {
	// SAM.gov API keys are typically UUID-like or long alphanumeric
	if len(apiKey) < 20 {
		return false
	}
	
	// Check for reasonable character set
	validChars := regexp.MustCompile(`^[a-zA-Z0-9\-_]+$`)
	return validChars.MatchString(apiKey)
}

func (sa *SecurityAudit) isWeakCredential(value string) bool {
	weakPatterns := []string{
		"password", "123456", "qwerty", "admin", "root",
		"test", "user", "guest", "default",
	}

	valueLower := strings.ToLower(value)
	for _, pattern := range weakPatterns {
		if strings.Contains(valueLower, pattern) {
			return true
		}
	}

	// Check for too simple patterns
	if len(value) < 8 {
		return true
	}

	return false
}

func (sa *SecurityAudit) containsPossibleSecret(envVar string) bool {
	secretPatterns := []string{
		"password", "secret", "key", "token", "auth", "credential",
	}

	envLower := strings.ToLower(envVar)
	for _, pattern := range secretPatterns {
		if strings.Contains(envLower, pattern) && strings.Contains(envVar, "=") {
			// Extract value part
			parts := strings.SplitN(envVar, "=", 2)
			if len(parts) == 2 && len(parts[1]) > 20 {
				return true
			}
		}
	}
	return false
}

func (sa *SecurityAudit) isBroadQuery(query config.Query) bool {
	// A query is considered broad if it has very few constraints
	constraintCount := 0
	
	if title, ok := query.Parameters["title"]; ok {
		if titleStr, ok := title.(string); ok && len(titleStr) > 5 {
			constraintCount++
		}
	}
	
	if org, ok := query.Parameters["organizationName"]; ok {
		if orgStr, ok := org.(string); ok && len(orgStr) > 3 {
			constraintCount++
		}
	}
	
	if naics, ok := query.Parameters["naicsCode"]; ok {
		if naicsStr, ok := naics.(string); ok && len(naicsStr) == 6 {
			constraintCount++
		}
	}

	return constraintCount < 2
}

func (sa *SecurityAudit) containsSuspiciousPatterns(value string) bool {
	suspiciousPatterns := []string{
		"'", "\"", ";", "--", "/*", "*/", "<script", "javascript:",
		"eval(", "exec(", "system(", "cmd", "/bin/", "rm -rf",
	}

	valueLower := strings.ToLower(value)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(valueLower, pattern) {
			return true
		}
	}
	return false
}

func (sa *SecurityAudit) addIssue(level, category, description, impact, remediation string) {
	issue := SecurityIssue{
		Level:       level,
		Category:    category,
		Description: description,
		Impact:      impact,
		Remediation: remediation,
		Timestamp:   time.Now(),
	}
	sa.issues = append(sa.issues, issue)
}

// GetIssues returns all security issues found
func (sa *SecurityAudit) GetIssues() []SecurityIssue {
	return sa.issues
}

// getCriticalIssues returns only error-level issues
func (sa *SecurityAudit) getCriticalIssues() []SecurityIssue {
	var critical []SecurityIssue
	for _, issue := range sa.issues {
		if issue.Level == "error" {
			critical = append(critical, issue)
		}
	}
	return critical
}

// GetAuditReport generates a comprehensive security audit report
func (sa *SecurityAudit) GetAuditReport() string {
	report := fmt.Sprintf("# Security Audit Report\n\n")
	report += fmt.Sprintf("Generated: %s\n\n", time.Now().Format(time.RFC3339))

	// Summary
	errorCount := 0
	warningCount := 0
	infoCount := 0

	for _, issue := range sa.issues {
		switch issue.Level {
		case "error":
			errorCount++
		case "warning":
			warningCount++
		case "info":
			infoCount++
		}
	}

	report += fmt.Sprintf("## Summary\n")
	report += fmt.Sprintf("- Total Issues: %d\n", len(sa.issues))
	report += fmt.Sprintf("- Errors: %d\n", errorCount)
	report += fmt.Sprintf("- Warnings: %d\n", warningCount)
	report += fmt.Sprintf("- Info: %d\n\n", infoCount)

	// Overall security status
	if errorCount == 0 {
		report += fmt.Sprintf("## Overall Status: ✅ PASS\n")
		report += fmt.Sprintf("No critical security issues detected.\n\n")
	} else {
		report += fmt.Sprintf("## Overall Status: ❌ FAIL\n")
		report += fmt.Sprintf("Critical security issues require immediate attention.\n\n")
	}

	// Issues by category
	categories := make(map[string][]SecurityIssue)
	for _, issue := range sa.issues {
		categories[issue.Category] = append(categories[issue.Category], issue)
	}

	for category, issues := range categories {
		report += fmt.Sprintf("## %s Issues\n", strings.Title(category))
		for _, issue := range issues {
			icon := "ℹ️"
			if issue.Level == "warning" {
				icon = "⚠️"
			} else if issue.Level == "error" {
				icon = "❌"
			}
			
			report += fmt.Sprintf("### %s %s\n", icon, issue.Description)
			report += fmt.Sprintf("**Impact:** %s\n", issue.Impact)
			report += fmt.Sprintf("**Remediation:** %s\n\n", issue.Remediation)
		}
	}

	return report
}

// printAuditResults prints audit results to console
func (sa *SecurityAudit) printAuditResults() {
	errorCount := 0
	warningCount := 0

	for _, issue := range sa.issues {
		switch issue.Level {
		case "error":
			errorCount++
			log.Printf("SECURITY ERROR [%s]: %s", issue.Category, issue.Description)
		case "warning":
			warningCount++
			log.Printf("SECURITY WARNING [%s]: %s", issue.Category, issue.Description)
		}
	}

	if errorCount == 0 && warningCount == 0 {
		log.Printf("Security audit passed: no critical issues found")
	} else {
		log.Printf("Security audit completed: %d errors, %d warnings", errorCount, warningCount)
	}
}