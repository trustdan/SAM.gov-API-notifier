package config

import (
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string `json:"field"`
	Value   string `json:"value"`
	Message string `json:"message"`
	Level   string `json:"level"` // "error", "warning", "info"
}

// ValidationResult contains the results of configuration validation
type ValidationResult struct {
	Valid   bool              `json:"valid"`
	Errors  []ValidationError `json:"errors"`
	Warnings []ValidationError `json:"warnings"`
}

// ConfigValidator provides comprehensive configuration validation
type ConfigValidator struct {
	strict bool // If true, warnings are treated as errors
}

// NewConfigValidator creates a new configuration validator
func NewConfigValidator(strict bool) *ConfigValidator {
	return &ConfigValidator{
		strict: strict,
	}
}

// Validate performs comprehensive validation of the configuration
func (cv *ConfigValidator) Validate(config *Config) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   make([]ValidationError, 0),
		Warnings: make([]ValidationError, 0),
	}

	if config == nil {
		cv.addError(result, "config", "", "Configuration is nil")
		return result
	}

	// Validate queries
	cv.validateQueries(config, result)

	// Set overall validity
	result.Valid = len(result.Errors) == 0 && (!cv.strict || len(result.Warnings) == 0)

	return result
}

// validateQueries validates all queries in the configuration
func (cv *ConfigValidator) validateQueries(config *Config, result *ValidationResult) {
	if len(config.Queries) == 0 {
		cv.addError(result, "queries", "", "No queries defined in configuration")
		return
	}

	enabledCount := 0
	queryNames := make(map[string]bool)

	for i, query := range config.Queries {
		fieldPrefix := fmt.Sprintf("queries[%d]", i)
		
		// Validate individual query
		cv.validateQuery(query, fieldPrefix, result)

		// Check for enabled queries
		if query.Enabled {
			enabledCount++
		}

		// Check for duplicate names
		if queryNames[query.Name] {
			cv.addError(result, fieldPrefix+".name", query.Name, "Duplicate query name")
		} else {
			queryNames[query.Name] = true
		}
	}

	// Check that at least one query is enabled
	if enabledCount == 0 {
		cv.addWarning(result, "queries", "", "No queries are enabled - no monitoring will occur")
	}

	// Warn if too many queries (performance concern)
	if len(config.Queries) > 20 {
		cv.addWarning(result, "queries", fmt.Sprintf("%d", len(config.Queries)), 
			"Large number of queries may impact performance")
	}
}

// validateQuery validates a single query configuration
func (cv *ConfigValidator) validateQuery(query Query, fieldPrefix string, result *ValidationResult) {
	// Validate query name
	if strings.TrimSpace(query.Name) == "" {
		cv.addError(result, fieldPrefix+".name", query.Name, "Query name cannot be empty")
	} else {
		// Check for valid characters in name
		if !cv.isValidName(query.Name) {
			cv.addWarning(result, fieldPrefix+".name", query.Name, 
				"Query name contains special characters that may cause issues")
		}
	}

	// Validate parameters
	cv.validateQueryParameters(query, fieldPrefix+".parameters", result)

	// Validate notification configuration
	cv.validateNotificationConfig(query.Notification, fieldPrefix+".notification", result)
}

// validateQueryParameters validates query parameters
func (cv *ConfigValidator) validateQueryParameters(query Query, fieldPrefix string, result *ValidationResult) {
	if len(query.Parameters) == 0 {
		cv.addError(result, fieldPrefix, "", "Query parameters cannot be empty")
		return
	}

	// Check for at least one search criterion
	searchCriteria := []string{"title", "organizationName", "naicsCode", "typeOfSetAside", "state"}
	hasSearchCriteria := false

	for _, criterion := range searchCriteria {
		if value, exists := query.Parameters[criterion]; exists {
			if cv.isNonEmptyValue(value) {
				hasSearchCriteria = true
				break
			}
		}
	}

	if !hasSearchCriteria {
		cv.addError(result, fieldPrefix, "", 
			"Query must have at least one search criterion (title, organizationName, naicsCode, typeOfSetAside, or state)")
	}

	// Validate specific parameters
	for key, value := range query.Parameters {
		paramField := fmt.Sprintf("%s.%s", fieldPrefix, key)
		cv.validateParameter(key, value, paramField, result)
	}

	// Validate advanced parameters if present
	if advanced, exists := query.Parameters["advanced"]; exists {
		cv.validateAdvancedParameters(advanced, fieldPrefix+".advanced", result)
	}
}

// validateParameter validates a specific parameter
func (cv *ConfigValidator) validateParameter(key string, value interface{}, fieldPrefix string, result *ValidationResult) {
	switch key {
	case "title":
		cv.validateTitle(value, fieldPrefix, result)
	case "organizationName":
		cv.validateOrganizationName(value, fieldPrefix, result)
	case "naicsCode":
		cv.validateNAICSCode(value, fieldPrefix, result)
	case "typeOfSetAside":
		cv.validateSetAsideType(value, fieldPrefix, result)
	case "state":
		cv.validateStateCode(value, fieldPrefix, result)
	case "ptype":
		cv.validatePostingType(value, fieldPrefix, result)
	case "limit":
		cv.validateLimit(value, fieldPrefix, result)
	case "lookbackDays":
		cv.validateLookbackDays(value, fieldPrefix, result)
	default:
		// Unknown parameter - warn but don't error
		cv.addWarning(result, fieldPrefix, fmt.Sprintf("%v", value), 
			fmt.Sprintf("Unknown parameter '%s' - may be ignored by SAM.gov API", key))
	}
}

// Parameter-specific validation methods

func (cv *ConfigValidator) validateTitle(value interface{}, fieldPrefix string, result *ValidationResult) {
	str, ok := value.(string)
	if !ok {
		cv.addError(result, fieldPrefix, fmt.Sprintf("%v", value), "Title must be a string")
		return
	}

	str = strings.TrimSpace(str)
	if len(str) == 0 {
		cv.addError(result, fieldPrefix, str, "Title cannot be empty")
		return
	}

	if len(str) < 3 {
		cv.addWarning(result, fieldPrefix, str, "Very short title may not match many opportunities")
	}

	if len(str) > 100 {
		cv.addWarning(result, fieldPrefix, str, "Very long title may be truncated by API")
	}

	// Check for suspicious patterns
	if cv.containsSuspiciousPatterns(str) {
		cv.addWarning(result, fieldPrefix, str, "Title contains potentially suspicious patterns")
	}
}

func (cv *ConfigValidator) validateOrganizationName(value interface{}, fieldPrefix string, result *ValidationResult) {
	str, ok := value.(string)
	if !ok {
		cv.addError(result, fieldPrefix, fmt.Sprintf("%v", value), "Organization name must be a string")
		return
	}

	str = strings.TrimSpace(str)
	if len(str) == 0 {
		cv.addError(result, fieldPrefix, str, "Organization name cannot be empty")
		return
	}

	// Check for common organization name patterns
	commonOrgs := []string{"DARPA", "DOD", "NASA", "NSF", "DOE", "DHS", "VA", "GSA"}
	upperStr := strings.ToUpper(str)
	
	found := false
	for _, org := range commonOrgs {
		if strings.Contains(upperStr, org) {
			found = true
			break
		}
	}

	if !found && len(str) < 10 {
		cv.addWarning(result, fieldPrefix, str, 
			"Organization name is short and may not match expected agencies")
	}
}

func (cv *ConfigValidator) validateNAICSCode(value interface{}, fieldPrefix string, result *ValidationResult) {
	codes := cv.extractStringArray(value)
	if len(codes) == 0 {
		cv.addError(result, fieldPrefix, fmt.Sprintf("%v", value), "NAICS code cannot be empty")
		return
	}

	naicsRegex := regexp.MustCompile(`^\d{6}$`)
	for _, code := range codes {
		if !naicsRegex.MatchString(code) {
			cv.addError(result, fieldPrefix, code, "NAICS code must be exactly 6 digits")
		}
	}
}

func (cv *ConfigValidator) validateSetAsideType(value interface{}, fieldPrefix string, result *ValidationResult) {
	types := cv.extractStringArray(value)
	if len(types) == 0 {
		cv.addError(result, fieldPrefix, fmt.Sprintf("%v", value), "Set-aside type cannot be empty")
		return
	}

	validTypes := map[string]bool{
		"SBA": true, "8A": true, "WOSB": true, "SDVOSBC": true, "HZ": true,
		"SBR": true, "IEE": true, "FS": true, "EDW": true,
	}

	for _, setAsideType := range types {
		if !validTypes[strings.ToUpper(setAsideType)] {
			cv.addWarning(result, fieldPrefix, setAsideType, 
				fmt.Sprintf("Unknown set-aside type '%s'", setAsideType))
		}
	}
}

func (cv *ConfigValidator) validateStateCode(value interface{}, fieldPrefix string, result *ValidationResult) {
	codes := cv.extractStringArray(value)
	if len(codes) == 0 {
		cv.addError(result, fieldPrefix, fmt.Sprintf("%v", value), "State code cannot be empty")
		return
	}

	validStates := map[string]bool{
		"AL": true, "AK": true, "AZ": true, "AR": true, "CA": true, "CO": true,
		"CT": true, "DE": true, "FL": true, "GA": true, "HI": true, "ID": true,
		"IL": true, "IN": true, "IA": true, "KS": true, "KY": true, "LA": true,
		"ME": true, "MD": true, "MA": true, "MI": true, "MN": true, "MS": true,
		"MO": true, "MT": true, "NE": true, "NV": true, "NH": true, "NJ": true,
		"NM": true, "NY": true, "NC": true, "ND": true, "OH": true, "OK": true,
		"OR": true, "PA": true, "RI": true, "SC": true, "SD": true, "TN": true,
		"TX": true, "UT": true, "VT": true, "VA": true, "WA": true, "WV": true,
		"WI": true, "WY": true, "DC": true, "PR": true, "VI": true, "GU": true,
		"AS": true, "MP": true,
	}

	for _, code := range codes {
		if !validStates[strings.ToUpper(code)] {
			cv.addError(result, fieldPrefix, code, fmt.Sprintf("Invalid state code '%s'", code))
		}
	}
}

func (cv *ConfigValidator) validatePostingType(value interface{}, fieldPrefix string, result *ValidationResult) {
	types := cv.extractStringArray(value)
	if len(types) == 0 {
		cv.addError(result, fieldPrefix, fmt.Sprintf("%v", value), "Posting type cannot be empty")
		return
	}

	validTypes := map[string]bool{
		"s": true, "p": true, "o": true, "k": true, "r": true,
		"g": true, "a": true, "i": true, "u": true,
	}

	for _, ptype := range types {
		if !validTypes[strings.ToLower(ptype)] {
			cv.addError(result, fieldPrefix, ptype, 
				fmt.Sprintf("Invalid posting type '%s'. Valid types: s, p, o, k, r, g, a, i, u", ptype))
		}
	}
}

func (cv *ConfigValidator) validateLimit(value interface{}, fieldPrefix string, result *ValidationResult) {
	var limit int
	var err error

	switch v := value.(type) {
	case int:
		limit = v
	case string:
		limit, err = strconv.Atoi(v)
		if err != nil {
			cv.addError(result, fieldPrefix, fmt.Sprintf("%v", value), "Limit must be a number")
			return
		}
	default:
		cv.addError(result, fieldPrefix, fmt.Sprintf("%v", value), "Limit must be a number")
		return
	}

	if limit < 1 {
		cv.addError(result, fieldPrefix, fmt.Sprintf("%d", limit), "Limit must be at least 1")
	} else if limit > 1000 {
		cv.addWarning(result, fieldPrefix, fmt.Sprintf("%d", limit), 
			"Very high limit may cause performance issues")
	}
}

func (cv *ConfigValidator) validateLookbackDays(value interface{}, fieldPrefix string, result *ValidationResult) {
	var days int
	var err error

	switch v := value.(type) {
	case int:
		days = v
	case string:
		days, err = strconv.Atoi(v)
		if err != nil {
			cv.addError(result, fieldPrefix, fmt.Sprintf("%v", value), "Lookback days must be a number")
			return
		}
	default:
		cv.addError(result, fieldPrefix, fmt.Sprintf("%v", value), "Lookback days must be a number")
		return
	}

	if days < 1 {
		cv.addError(result, fieldPrefix, fmt.Sprintf("%d", days), "Lookback days must be at least 1")
	} else if days > 365 {
		cv.addWarning(result, fieldPrefix, fmt.Sprintf("%d", days), 
			"Very long lookback period may return excessive results")
	}
}

// validateAdvancedParameters validates advanced query parameters
func (cv *ConfigValidator) validateAdvancedParameters(value interface{}, fieldPrefix string, result *ValidationResult) {
	advanced, ok := value.(map[string]interface{})
	if !ok {
		// Try map[interface{}]interface{} (YAML sometimes parses to this)
		if advancedInterface, ok := value.(map[interface{}]interface{}); ok {
			advanced = make(map[string]interface{})
			for k, v := range advancedInterface {
				if keyStr, ok := k.(string); ok {
					advanced[keyStr] = v
				}
			}
		} else {
			cv.addError(result, fieldPrefix, fmt.Sprintf("%v", value), 
				"Advanced parameters must be a map")
			return
		}
	}

	// Validate specific advanced parameters
	if include, exists := advanced["include"]; exists {
		cv.validateStringArray(include, fieldPrefix+".include", "Include keywords", result)
	}

	if exclude, exists := advanced["exclude"]; exists {
		cv.validateStringArray(exclude, fieldPrefix+".exclude", "Exclude keywords", result)
	}

	if minValue, exists := advanced["minValue"]; exists {
		cv.validateNumericValue(minValue, fieldPrefix+".minValue", "Minimum value", 0, 1000000000, result)
	}

	if maxValue, exists := advanced["maxValue"]; exists {
		cv.validateNumericValue(maxValue, fieldPrefix+".maxValue", "Maximum value", 0, 1000000000, result)
	}

	if maxDays, exists := advanced["maxDaysOld"]; exists {
		cv.validateNumericValue(maxDays, fieldPrefix+".maxDaysOld", "Maximum days old", 1, 365, result)
	}
}

// validateNotificationConfig validates notification configuration
func (cv *ConfigValidator) validateNotificationConfig(notification NotificationConfig, fieldPrefix string, result *ValidationResult) {
	// Validate priority
	validPriorities := map[string]bool{"low": true, "medium": true, "high": true}
	if !validPriorities[strings.ToLower(notification.Priority)] {
		cv.addError(result, fieldPrefix+".priority", notification.Priority, 
			"Priority must be 'low', 'medium', or 'high'")
	}

	// Validate recipients
	if len(notification.Recipients) == 0 {
		cv.addWarning(result, fieldPrefix+".recipients", "", 
			"No recipients specified - notifications will not be sent")
	} else {
		for i, recipient := range notification.Recipients {
			recipientField := fmt.Sprintf("%s.recipients[%d]", fieldPrefix, i)
			if !cv.isValidEmail(recipient) {
				cv.addError(result, recipientField, recipient, "Invalid email address format")
			}
		}

		// Warn about too many recipients
		if len(notification.Recipients) > 10 {
			cv.addWarning(result, fieldPrefix+".recipients", 
				fmt.Sprintf("%d recipients", len(notification.Recipients)), 
				"Large number of recipients may cause notification fatigue")
		}
	}

	// Validate channels if present
	if len(notification.Channels) > 0 {
		validChannels := map[string]bool{"email": true, "slack": true, "github": true}
		for i, channel := range notification.Channels {
			channelField := fmt.Sprintf("%s.channels[%d]", fieldPrefix, i)
			if !validChannels[strings.ToLower(channel)] {
				cv.addWarning(result, channelField, channel, 
					"Unknown notification channel - supported: email, slack, github")
			}
		}
	}
}

// Helper methods

func (cv *ConfigValidator) isValidName(name string) bool {
	// Allow alphanumeric, spaces, hyphens, underscores
	validName := regexp.MustCompile(`^[a-zA-Z0-9\s\-_]+$`)
	return validName.MatchString(name)
}

func (cv *ConfigValidator) isValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

func (cv *ConfigValidator) isValidURL(urlStr string) bool {
	_, err := url.Parse(urlStr)
	return err == nil
}

func (cv *ConfigValidator) isNonEmptyValue(value interface{}) bool {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) != ""
	case []string:
		return len(v) > 0
	case []interface{}:
		return len(v) > 0
	default:
		return value != nil
	}
}

func (cv *ConfigValidator) extractStringArray(value interface{}) []string {
	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return []string{}
		}
		return []string{v}
	case []string:
		return v
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	default:
		return []string{}
	}
}

func (cv *ConfigValidator) validateStringArray(value interface{}, fieldPrefix, description string, result *ValidationResult) {
	strings := cv.extractStringArray(value)
	if len(strings) == 0 {
		cv.addWarning(result, fieldPrefix, fmt.Sprintf("%v", value), 
			fmt.Sprintf("%s list is empty", description))
		return
	}

	for i, str := range strings {
		if strings.TrimSpace(str) == "" {
			cv.addWarning(result, fmt.Sprintf("%s[%d]", fieldPrefix, i), str, 
				fmt.Sprintf("Empty %s entry", strings.ToLower(description)))
		}
	}
}

func (cv *ConfigValidator) validateNumericValue(value interface{}, fieldPrefix, description string, min, max float64, result *ValidationResult) {
	var num float64
	var err error

	switch v := value.(type) {
	case int:
		num = float64(v)
	case float64:
		num = v
	case string:
		num, err = strconv.ParseFloat(v, 64)
		if err != nil {
			cv.addError(result, fieldPrefix, fmt.Sprintf("%v", value), 
				fmt.Sprintf("%s must be a number", description))
			return
		}
	default:
		cv.addError(result, fieldPrefix, fmt.Sprintf("%v", value), 
			fmt.Sprintf("%s must be a number", description))
		return
	}

	if num < min {
		cv.addError(result, fieldPrefix, fmt.Sprintf("%.0f", num), 
			fmt.Sprintf("%s must be at least %.0f", description, min))
	}

	if num > max {
		cv.addWarning(result, fieldPrefix, fmt.Sprintf("%.0f", num), 
			fmt.Sprintf("%s is very high (%.0f), may cause issues", description, num))
	}
}

func (cv *ConfigValidator) containsSuspiciousPatterns(value string) bool {
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

func (cv *ConfigValidator) addError(result *ValidationResult, field, value, message string) {
	result.Errors = append(result.Errors, ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
		Level:   "error",
	})
}

func (cv *ConfigValidator) addWarning(result *ValidationResult, field, value, message string) {
	result.Warnings = append(result.Warnings, ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
		Level:   "warning",
	})
}

// GenerateValidationReport creates a human-readable validation report
func (result *ValidationResult) GenerateValidationReport() string {
	report := fmt.Sprintf("# Configuration Validation Report\n\n")
	report += fmt.Sprintf("Generated: %s\n\n", time.Now().Format(time.RFC3339))

	// Summary
	report += fmt.Sprintf("## Summary\n")
	report += fmt.Sprintf("- Overall Status: ")
	if result.Valid {
		report += "✅ VALID\n"
	} else {
		report += "❌ INVALID\n"
	}
	report += fmt.Sprintf("- Errors: %d\n", len(result.Errors))
	report += fmt.Sprintf("- Warnings: %d\n\n", len(result.Warnings))

	// Errors
	if len(result.Errors) > 0 {
		report += fmt.Sprintf("## ❌ Errors\n")
		for _, err := range result.Errors {
			report += fmt.Sprintf("- **%s**: %s", err.Field, err.Message)
			if err.Value != "" {
				report += fmt.Sprintf(" (value: '%s')", err.Value)
			}
			report += "\n"
		}
		report += "\n"
	}

	// Warnings
	if len(result.Warnings) > 0 {
		report += fmt.Sprintf("## ⚠️ Warnings\n")
		for _, warn := range result.Warnings {
			report += fmt.Sprintf("- **%s**: %s", warn.Field, warn.Message)
			if warn.Value != "" {
				report += fmt.Sprintf(" (value: '%s')", warn.Value)
			}
			report += "\n"
		}
		report += "\n"
	}

	if result.Valid {
		report += fmt.Sprintf("## Recommendations\n")
		report += fmt.Sprintf("Configuration is valid and ready for production use.\n")
		if len(result.Warnings) > 0 {
			report += fmt.Sprintf("Consider addressing warnings for optimal performance.\n")
		}
	}

	return report
}