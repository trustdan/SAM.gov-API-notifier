package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

// Config represents the complete configuration for the monitor
type Config struct {
	Queries []Query `yaml:"queries"`
}

// Query represents a single search query configuration
type Query struct {
	Name         string                 `yaml:"name"`
	Enabled      bool                   `yaml:"enabled"`
	Parameters   map[string]interface{} `yaml:"parameters"`
	Notification NotificationConfig     `yaml:"notification"`
	Advanced     AdvancedQuery          `yaml:"advanced,omitempty"`
}

// NotificationConfig defines how notifications should be sent
type NotificationConfig struct {
	Priority    string   `yaml:"priority"`    // high, medium, low
	Recipients  []string `yaml:"recipients,omitempty"`
	Channels    []string `yaml:"channels,omitempty"` // email, slack, github
	Template    string   `yaml:"template,omitempty"`
	Digest      bool     `yaml:"digest,omitempty"`   // group notifications
}

// AdvancedQuery provides additional filtering options
type AdvancedQuery struct {
	Include       []string  `yaml:"include,omitempty"`        // Keywords that must be present
	Exclude       []string  `yaml:"exclude,omitempty"`        // Keywords that must not be present
	MinValue      float64   `yaml:"minValue,omitempty"`       // Minimum contract value
	MaxValue      float64   `yaml:"maxValue,omitempty"`       // Maximum contract value
	MaxDaysOld    int       `yaml:"maxDaysOld,omitempty"`     // Maximum age in days
	SetAsideTypes []string  `yaml:"setAsideTypes,omitempty"`  // Required set-aside types
	NAICSCodes    []string  `yaml:"naicsCodes,omitempty"`     // Required NAICS codes
}

// Load reads and parses the configuration file
func Load(filepath string) (*Config, error) {
	if filepath == "" {
		return nil, errors.New("config file path is required")
	}

	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", filepath, err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", filepath, err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return &config, nil
}

// Validate checks the configuration for errors and inconsistencies
func (c *Config) Validate() error {
	if len(c.Queries) == 0 {
		return errors.New("no queries configured")
	}

	enabledCount := 0
	for i, query := range c.Queries {
		if err := query.Validate(); err != nil {
			return fmt.Errorf("query %d (%s): %w", i, query.Name, err)
		}
		if query.Enabled {
			enabledCount++
		}
	}

	if enabledCount == 0 {
		return errors.New("no enabled queries found")
	}

	return nil
}

// Validate checks a single query configuration
func (q *Query) Validate() error {
	if q.Name == "" {
		return errors.New("query name is required")
	}

	if q.Name == "test" || strings.Contains(strings.ToLower(q.Name), "example") {
		return fmt.Errorf("query name '%s' appears to be a placeholder", q.Name)
	}

	// Validate notification priority
	validPriorities := map[string]bool{"high": true, "medium": true, "low": true}
	if q.Notification.Priority != "" && !validPriorities[q.Notification.Priority] {
		return fmt.Errorf("invalid notification priority '%s', must be high, medium, or low", q.Notification.Priority)
	}

	// Validate notification channels
	validChannels := map[string]bool{"email": true, "slack": true, "github": true}
	for _, channel := range q.Notification.Channels {
		if !validChannels[channel] {
			return fmt.Errorf("invalid notification channel '%s'", channel)
		}
	}

	// Validate advanced query parameters
	if q.Advanced.MaxDaysOld < 0 {
		return errors.New("maxDaysOld cannot be negative")
	}
	if q.Advanced.MaxDaysOld > 365 {
		return errors.New("maxDaysOld cannot exceed 365 days")
	}

	if q.Advanced.MinValue < 0 {
		return errors.New("minValue cannot be negative")
	}
	if q.Advanced.MaxValue > 0 && q.Advanced.MinValue > q.Advanced.MaxValue {
		return errors.New("minValue cannot be greater than maxValue")
	}

	// Validate lookback days parameter
	if lookbackDays, ok := q.Parameters["lookbackDays"].(int); ok {
		if lookbackDays < 1 {
			return errors.New("lookbackDays must be at least 1")
		}
		if lookbackDays > 365 {
			return errors.New("lookbackDays cannot exceed 365")
		}
	}

	return nil
}

// GetEnabledQueries returns only the enabled queries
func (c *Config) GetEnabledQueries() []Query {
	enabled := make([]Query, 0)
	for _, query := range c.Queries {
		if query.Enabled {
			enabled = append(enabled, query)
		}
	}
	return enabled
}

// GetHighPriorityQueries returns queries with high priority notifications
func (c *Config) GetHighPriorityQueries() []Query {
	highPriority := make([]Query, 0)
	for _, query := range c.Queries {
		if query.Enabled && query.Notification.Priority == "high" {
			highPriority = append(highPriority, query)
		}
	}
	return highPriority
}