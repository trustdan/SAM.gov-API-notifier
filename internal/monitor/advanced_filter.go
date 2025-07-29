package monitor

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/yourusername/sam-gov-monitor/internal/config"
	"github.com/yourusername/sam-gov-monitor/internal/samgov"
)

// AdvancedQuery represents advanced filtering criteria
type AdvancedQuery struct {
	Include    []string               `yaml:"include"`
	Exclude    []string               `yaml:"exclude"`
	MinValue   *float64               `yaml:"minValue"`
	MaxValue   *float64               `yaml:"maxValue"`
	MaxDaysOld *int                   `yaml:"maxDaysOld"`
	NAICSCodes []string               `yaml:"naicsCodes"`
	SetAsides  []string               `yaml:"setAsideTypes"`
	Custom     map[string]interface{} `yaml:"custom"`
}

// AdvancedFilter provides sophisticated filtering capabilities
type AdvancedFilter struct {
	verbose bool
}

// NewAdvancedFilter creates a new advanced filter
func NewAdvancedFilter(verbose bool) *AdvancedFilter {
	return &AdvancedFilter{
		verbose: verbose,
	}
}

// FilterOpportunities applies advanced filtering to a list of opportunities
func (af *AdvancedFilter) FilterOpportunities(opportunities []samgov.Opportunity, query config.Query) ([]samgov.Opportunity, error) {
	// Extract advanced query parameters
	advanced, err := af.extractAdvancedQuery(query)
	if err != nil {
		return nil, fmt.Errorf("extracting advanced query: %w", err)
	}

	if advanced == nil {
		// No advanced filtering, return all
		return opportunities, nil
	}

	filtered := make([]samgov.Opportunity, 0)

	for _, opp := range opportunities {
		if af.matchesAdvancedCriteria(opp, *advanced) {
			filtered = append(filtered, opp)
		}
	}

	if af.verbose {
		fmt.Printf("Advanced filtering: %d -> %d opportunities\n", 
			len(opportunities), len(filtered))
	}

	return filtered, nil
}

// extractAdvancedQuery extracts advanced filtering parameters from query config
func (af *AdvancedFilter) extractAdvancedQuery(query config.Query) (*AdvancedQuery, error) {
	advancedRaw, exists := query.Parameters["advanced"]
	if !exists {
		return nil, nil
	}

	switch v := advancedRaw.(type) {
	case map[string]interface{}:
		return af.parseAdvancedMap(v)
	case map[interface{}]interface{}:
		// Convert interface{} keys to strings
		stringMap := make(map[string]interface{})
		for key, val := range v {
			if strKey, ok := key.(string); ok {
				stringMap[strKey] = val
			}
		}
		return af.parseAdvancedMap(stringMap)
	default:
		return nil, fmt.Errorf("advanced parameters must be a map, got %T", v)
	}
}

// parseAdvancedMap parses advanced filtering parameters from a map
func (af *AdvancedFilter) parseAdvancedMap(params map[string]interface{}) (*AdvancedQuery, error) {
	advanced := &AdvancedQuery{}

	// Parse include keywords
	if include, exists := params["include"]; exists {
		advanced.Include = af.parseStringArray(include)
	}

	// Parse exclude keywords
	if exclude, exists := params["exclude"]; exists {
		advanced.Exclude = af.parseStringArray(exclude)
	}

	// Parse value filters
	if minVal, exists := params["minValue"]; exists {
		if val, err := af.parseFloat(minVal); err == nil {
			advanced.MinValue = &val
		}
	}

	if maxVal, exists := params["maxValue"]; exists {
		if val, err := af.parseFloat(maxVal); err == nil {
			advanced.MaxValue = &val
		}
	}

	// Parse age filter
	if maxDays, exists := params["maxDaysOld"]; exists {
		if days, err := af.parseInt(maxDays); err == nil {
			advanced.MaxDaysOld = &days
		}
	}

	// Parse NAICS codes
	if naics, exists := params["naicsCodes"]; exists {
		advanced.NAICSCodes = af.parseStringArray(naics)
	}

	// Parse set-aside types
	if setAsides, exists := params["setAsideTypes"]; exists {
		advanced.SetAsides = af.parseStringArray(setAsides)
	}

	// Store any custom parameters
	advanced.Custom = make(map[string]interface{})
	for key, val := range params {
		if !af.isBuiltinParameter(key) {
			advanced.Custom[key] = val
		}
	}

	return advanced, nil
}

// matchesAdvancedCriteria checks if an opportunity matches advanced filtering criteria
func (af *AdvancedFilter) matchesAdvancedCriteria(opp samgov.Opportunity, advanced AdvancedQuery) bool {
	// Check include keywords
	if len(advanced.Include) > 0 {
		if !af.containsAnyKeyword(opp, advanced.Include) {
			return false
		}
	}

	// Check exclude keywords
	if len(advanced.Exclude) > 0 {
		if af.containsAnyKeyword(opp, advanced.Exclude) {
			return false
		}
	}

	// Check value filters
	if advanced.MinValue != nil || advanced.MaxValue != nil {
		if !af.matchesValueFilter(opp, advanced.MinValue, advanced.MaxValue) {
			return false
		}
	}

	// Check age filter
	if advanced.MaxDaysOld != nil {
		if !af.matchesAgeFilter(opp, *advanced.MaxDaysOld) {
			return false
		}
	}

	// Check NAICS codes
	if len(advanced.NAICSCodes) > 0 {
		if !af.matchesNAICSFilter(opp, advanced.NAICSCodes) {
			return false
		}
	}

	// Check set-aside types
	if len(advanced.SetAsides) > 0 {
		if !af.matchesSetAsideFilter(opp, advanced.SetAsides) {
			return false
		}
	}

	return true
}

// containsAnyKeyword checks if opportunity contains any of the specified keywords
func (af *AdvancedFilter) containsAnyKeyword(opp samgov.Opportunity, keywords []string) bool {
	// Combine searchable text fields
	searchText := strings.ToLower(fmt.Sprintf("%s %s %s %s", 
		opp.Title, opp.Description, opp.FullParentPath, opp.Type))

	for _, keyword := range keywords {
		if strings.Contains(searchText, strings.ToLower(keyword)) {
			return true
		}
	}

	return false
}

// matchesValueFilter checks if opportunity matches value constraints
func (af *AdvancedFilter) matchesValueFilter(opp samgov.Opportunity, minVal, maxVal *float64) bool {
	// Extract value from description or award amount fields
	// This is a simplified implementation - real implementation would need
	// to parse various currency formats and contract value representations
	
	// For now, we'll skip value filtering if we can't determine the value
	// In a real implementation, you'd parse the description for dollar amounts
	
	return true // Placeholder - implement value extraction logic
}

// matchesAgeFilter checks if opportunity is within the age limit
func (af *AdvancedFilter) matchesAgeFilter(opp samgov.Opportunity, maxDays int) bool {
	postedDate, err := time.Parse("2006-01-02", opp.PostedDate)
	if err != nil {
		// Try alternative date format
		postedDate, err = time.Parse("01/02/2006", opp.PostedDate)
		if err != nil {
			// If we can't parse the date, don't filter by age
			return true
		}
	}

	daysSincePosted := int(time.Since(postedDate).Hours() / 24)
	return daysSincePosted <= maxDays
}

// matchesNAICSFilter checks if opportunity matches NAICS code requirements
func (af *AdvancedFilter) matchesNAICSFilter(opp samgov.Opportunity, naicsCodes []string) bool {
	// This would need to be implemented based on how NAICS codes are 
	// represented in the opportunity data structure
	// Placeholder implementation
	return true
}

// matchesSetAsideFilter checks if opportunity matches set-aside requirements
func (af *AdvancedFilter) matchesSetAsideFilter(opp samgov.Opportunity, setAsides []string) bool {
	// This would need to be implemented based on how set-aside information
	// is represented in the opportunity data structure
	// Placeholder implementation
	return true
}

// Helper functions for parsing parameters

func (af *AdvancedFilter) parseStringArray(value interface{}) []string {
	switch v := value.(type) {
	case string:
		// Split comma-separated string
		parts := strings.Split(v, ",")
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
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
		return nil
	}
}

func (af *AdvancedFilter) parseFloat(value interface{}) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("cannot parse %T as float", value)
	}
}

func (af *AdvancedFilter) parseInt(value interface{}) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("cannot parse %T as int", value)
	}
}

func (af *AdvancedFilter) isBuiltinParameter(key string) bool {
	builtins := map[string]bool{
		"include":       true,
		"exclude":       true,
		"minValue":      true,
		"maxValue":      true,
		"maxDaysOld":    true,
		"naicsCodes":    true,
		"setAsideTypes": true,
	}
	return builtins[key]
}

// GenerateFilterReport creates a report of filtering results
func (af *AdvancedFilter) GenerateFilterReport(original, filtered []samgov.Opportunity, query config.Query) string {
	report := fmt.Sprintf("# Advanced Filtering Report for Query: %s\n\n", query.Name)
	report += fmt.Sprintf("- Original opportunities: %d\n", len(original))
	report += fmt.Sprintf("- Filtered opportunities: %d\n", len(filtered))
	report += fmt.Sprintf("- Filter efficiency: %.1f%%\n", 
		float64(len(filtered))/float64(len(original))*100)

	// Extract and display filtering criteria
	advanced, err := af.extractAdvancedQuery(query)
	if err != nil || advanced == nil {
		report += "\nNo advanced filtering criteria applied.\n"
		return report
	}

	report += "\n## Applied Filters:\n"
	
	if len(advanced.Include) > 0 {
		report += fmt.Sprintf("- Include keywords: %s\n", strings.Join(advanced.Include, ", "))
	}
	
	if len(advanced.Exclude) > 0 {
		report += fmt.Sprintf("- Exclude keywords: %s\n", strings.Join(advanced.Exclude, ", "))
	}
	
	if advanced.MinValue != nil {
		report += fmt.Sprintf("- Minimum value: $%.2f\n", *advanced.MinValue)
	}
	
	if advanced.MaxValue != nil {
		report += fmt.Sprintf("- Maximum value: $%.2f\n", *advanced.MaxValue)
	}
	
	if advanced.MaxDaysOld != nil {
		report += fmt.Sprintf("- Maximum age: %d days\n", *advanced.MaxDaysOld)
	}

	return report
}