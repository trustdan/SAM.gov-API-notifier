package monitor

import (
	"fmt"
	"strings"
	"time"

	"github.com/yourusername/sam-gov-monitor/internal/config"
)

// QueryBuilder converts config queries into SAM.gov API parameters
type QueryBuilder struct {
	lookbackDays int
}

// NewQueryBuilder creates a new query builder
func NewQueryBuilder(lookbackDays int) *QueryBuilder {
	return &QueryBuilder{
		lookbackDays: lookbackDays,
	}
}

// BuildParams converts a query configuration to API parameters
func (qb *QueryBuilder) BuildParams(query config.Query) (map[string]string, error) {
	params := make(map[string]string)

	// Set default date range
	to := time.Now()
	from := to.AddDate(0, 0, -qb.lookbackDays)
	
	// Check if query has custom lookback
	if customLookback, ok := query.Parameters["lookbackDays"]; ok {
		if days, ok := customLookback.(int); ok && days > 0 {
			from = to.AddDate(0, 0, -days)
		}
	}

	params["postedFrom"] = from.Format("01/02/2006")
	params["postedTo"] = to.Format("01/02/2006")

	// Set pagination defaults
	params["limit"] = "100"
	params["offset"] = "0"

	// Process query-specific parameters
	for key, value := range query.Parameters {
		// Skip internal parameters
		if key == "lookbackDays" {
			continue
		}

		param, err := qb.convertParameter(key, value)
		if err != nil {
			return nil, fmt.Errorf("converting parameter %s: %w", key, err)
		}

		if param != "" {
			params[key] = param
		}
	}

	// Apply query-specific overrides
	qb.applyQueryOverrides(params, query)

	return params, nil
}

// convertParameter converts a config parameter value to a string suitable for the API
func (qb *QueryBuilder) convertParameter(key string, value interface{}) (string, error) {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v), nil
		
	case []interface{}:
		// Handle array parameters
		if len(v) == 0 {
			return "", nil
		}
		
		switch key {
		case "ptype":
			// Posting types: join multiple values with commas
			var ptypes []string
			for _, item := range v {
				if str, ok := item.(string); ok {
					ptypes = append(ptypes, str)
				} else {
					return "", fmt.Errorf("ptype array contains non-string value")
				}
			}
			return strings.Join(ptypes, ","), nil
			
		case "typeOfSetAside":
			// Set-aside types: comma-separated or first value
			if len(v) == 1 {
				if str, ok := v[0].(string); ok {
					return str, nil
				}
			}
			// Multiple values - convert to comma-separated
			strs := make([]string, 0, len(v))
			for _, item := range v {
				if str, ok := item.(string); ok {
					strs = append(strs, str)
				}
			}
			return strings.Join(strs, ","), nil
			
		case "naicsCode":
			// NAICS codes: typically single value
			if str, ok := v[0].(string); ok {
				return str, nil
			}
			return "", fmt.Errorf("naicsCode array contains non-string value")
			
		default:
			// For other arrays, join with commas
			strs := make([]string, 0, len(v))
			for _, item := range v {
				if str, ok := item.(string); ok {
					strs = append(strs, str)
				}
			}
			return strings.Join(strs, ","), nil
		}
		
	case []string:
		// Handle string arrays
		if len(v) == 0 {
			return "", nil
		}
		
		switch key {
		case "ptype":
			return v[0], nil // Take first for now
		case "typeOfSetAside", "naicsCode":
			return strings.Join(v, ","), nil
		default:
			return strings.Join(v, ","), nil
		}
		
	case int:
		return fmt.Sprintf("%d", v), nil
		
	case float64:
		return fmt.Sprintf("%.0f", v), nil
		
	case bool:
		if v {
			return "true", nil
		}
		return "false", nil
		
	default:
		return "", fmt.Errorf("unsupported parameter type for %s: %T", key, value)
	}
}

// applyQueryOverrides applies query-specific parameter adjustments
func (qb *QueryBuilder) applyQueryOverrides(params map[string]string, query config.Query) {
	// Adjust limit based on query type
	if query.Notification.Priority == "high" {
		// High priority queries might need more results
		params["limit"] = "200"
	}

	// Handle organization name variations
	if orgName, exists := params["organizationName"]; exists {
		params["organizationName"] = qb.normalizeOrganizationName(orgName)
	}

	// Handle title search optimization
	if title, exists := params["title"]; exists {
		params["title"] = qb.optimizeTitle(title)
	}

	// Set sort order for consistency
	params["sortBy"] = "postedDate"
	params["sortOrder"] = "desc"
}

// normalizeOrganizationName standardizes organization names for better matching
func (qb *QueryBuilder) normalizeOrganizationName(name string) string {
	// Common organization name mappings
	mappings := map[string]string{
		"DARPA":                                    "DEFENSE ADVANCED RESEARCH PROJECTS AGENCY",
		"DOD":                                      "DEPARTMENT OF DEFENSE",
		"DOE":                                      "DEPARTMENT OF ENERGY",
		"NSF":                                      "NATIONAL SCIENCE FOUNDATION",
		"NASA":                                     "NATIONAL AERONAUTICS AND SPACE ADMINISTRATION",
		"DHS":                                      "DEPARTMENT OF HOMELAND SECURITY",
		"NAVY":                                     "DEPARTMENT OF THE NAVY",
		"ARMY":                                     "DEPARTMENT OF THE ARMY",
		"AIR FORCE":                               "DEPARTMENT OF THE AIR FORCE",
		"VA":                                       "DEPARTMENT OF VETERANS AFFAIRS",
		"GSA":                                      "GENERAL SERVICES ADMINISTRATION",
	}

	upper := strings.ToUpper(strings.TrimSpace(name))
	
	if expanded, exists := mappings[upper]; exists {
		return expanded
	}
	
	return name
}

// optimizeTitle optimizes title search terms for better API results
func (qb *QueryBuilder) optimizeTitle(title string) string {
	title = strings.TrimSpace(title)
	
	// Handle common abbreviations and variations
	replacements := map[string]string{
		"ai":                    "artificial intelligence",
		"ml":                    "machine learning",
		"cyber":                 "cybersecurity",
		"it":                    "information technology",
		"r&d":                   "research and development",
		"o&m":                   "operations and maintenance",
	}
	
	lower := strings.ToLower(title)
	for abbrev, expanded := range replacements {
		if strings.Contains(lower, abbrev) {
			// Add both abbreviated and expanded forms for better matching
			if !strings.Contains(lower, expanded) {
				title = title + " " + expanded
			}
		}
	}
	
	return title
}

// BuildMultipleQueries handles queries that need to be split into multiple API calls
func (qb *QueryBuilder) BuildMultipleQueries(query config.Query) ([]map[string]string, error) {
	// Check if we need to split the query
	ptypes := qb.extractStringArray(query.Parameters, "ptype")
	
	if len(ptypes) <= 1 {
		// Single query is sufficient
		params, err := qb.BuildParams(query)
		if err != nil {
			return nil, err
		}
		return []map[string]string{params}, nil
	}
	
	// Split into multiple queries, one per ptype
	queries := make([]map[string]string, 0, len(ptypes))
	
	for _, ptype := range ptypes {
		// Create a copy of the query with single ptype
		queryCopy := query
		paramsCopy := make(map[string]interface{})
		for k, v := range query.Parameters {
			paramsCopy[k] = v
		}
		paramsCopy["ptype"] = ptype
		queryCopy.Parameters = paramsCopy
		
		params, err := qb.BuildParams(queryCopy)
		if err != nil {
			return nil, fmt.Errorf("building query for ptype %s: %w", ptype, err)
		}
		
		queries = append(queries, params)
	}
	
	return queries, nil
}

// extractStringArray safely extracts a string array from parameters
func (qb *QueryBuilder) extractStringArray(params map[string]interface{}, key string) []string {
	value, exists := params[key]
	if !exists {
		return nil
	}
	
	switch v := value.(type) {
	case string:
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
		return nil
	}
}

// ValidateParameters checks if the query parameters are valid for the SAM.gov API
func (qb *QueryBuilder) ValidateParameters(query config.Query) error {
	params := query.Parameters
	
	// Check required parameters
	hasSearchCriteria := false
	searchFields := []string{"title", "organizationName", "naicsCode", "typeOfSetAside", "state"}
	
	for _, field := range searchFields {
		if value, exists := params[field]; exists {
			if str, ok := value.(string); ok && strings.TrimSpace(str) != "" {
				hasSearchCriteria = true
				break
			}
			if arr, ok := value.([]string); ok && len(arr) > 0 {
				hasSearchCriteria = true
				break
			}
			if arr, ok := value.([]interface{}); ok && len(arr) > 0 {
				hasSearchCriteria = true
				break
			}
		}
	}
	
	if !hasSearchCriteria {
		return fmt.Errorf("query must have at least one search criteria (title, organizationName, naicsCode, typeOfSetAside, or state)")
	}
	
	// Validate ptype values
	if ptypes := qb.extractStringArray(params, "ptype"); len(ptypes) > 0 {
		validPtypes := map[string]bool{
			"s": true, // Solicitation
			"p": true, // Pre-solicitation
			"o": true, // Special Notice
			"k": true, // Combined Synopsis/Solicitation
			"r": true, // Sources Sought
			"g": true, // Sale of Surplus Property
			"a": true, // Award Notice
			"i": true, // Intent to Bundle
			"u": true, // Justification and Authorization
		}
		
		for _, ptype := range ptypes {
			if !validPtypes[strings.ToLower(ptype)] {
				return fmt.Errorf("invalid ptype '%s', valid values are: s, p, o, k, r, g, a, i, u", ptype)
			}
		}
	}
	
	// Validate NAICS codes (basic format check)
	if naicsCodes := qb.extractStringArray(params, "naicsCode"); len(naicsCodes) > 0 {
		for _, code := range naicsCodes {
			if len(code) != 6 {
				return fmt.Errorf("NAICS code '%s' must be exactly 6 digits", code)
			}
			// Check if all characters are digits
			for _, char := range code {
				if char < '0' || char > '9' {
					return fmt.Errorf("NAICS code '%s' must contain only digits", code)
				}
			}
		}
	}
	
	// Validate state codes
	if states := qb.extractStringArray(params, "state"); len(states) > 0 {
		validStates := []string{
			"AL", "AK", "AZ", "AR", "CA", "CO", "CT", "DE", "FL", "GA",
			"HI", "ID", "IL", "IN", "IA", "KS", "KY", "LA", "ME", "MD",
			"MA", "MI", "MN", "MS", "MO", "MT", "NE", "NV", "NH", "NJ",
			"NM", "NY", "NC", "ND", "OH", "OK", "OR", "PA", "RI", "SC",
			"SD", "TN", "TX", "UT", "VT", "VA", "WA", "WV", "WI", "WY",
			"DC", "PR", "VI", "GU", "AS", "MP",
		}
		
		stateMap := make(map[string]bool)
		for _, state := range validStates {
			stateMap[state] = true
		}
		
		for _, state := range states {
			if !stateMap[strings.ToUpper(state)] {
				return fmt.Errorf("invalid state code '%s'", state)
			}
		}
	}
	
	return nil
}