package notify

import (
	"context"
	"log"
	"time"

	"github.com/yourusername/sam-gov-monitor/internal/samgov"
)

// Notifier defines the interface for sending notifications
type Notifier interface {
	Send(ctx context.Context, notification Notification) error
	GetType() string
	IsEnabled() bool
}

// Notification represents a notification to be sent
type Notification struct {
	QueryName     string                `json:"query_name"`
	Priority      Priority              `json:"priority"`
	Recipients    []string              `json:"recipients"`
	Subject       string                `json:"subject"`
	Body          Body                  `json:"body"`
	Opportunities []samgov.Opportunity  `json:"opportunities"`
	FilteredOut   []samgov.Opportunity  `json:"filtered_out,omitempty"`
	Summary       NotificationSummary   `json:"summary"`
	Metadata      map[string]interface{} `json:"metadata"`
	Timestamp     time.Time             `json:"timestamp"`
	Attachments   []Attachment          `json:"attachments,omitempty"`
}

// Body contains the notification content in different formats
type Body struct {
	Text string `json:"text"`
	HTML string `json:"html"`
	Markdown string `json:"markdown,omitempty"`
}

// Priority defines notification urgency levels
type Priority string

const (
	PriorityHigh   Priority = "high"
	PriorityMedium Priority = "medium"
	PriorityLow    Priority = "low"
)

// NotificationSummary provides quick stats about the notification
type NotificationSummary struct {
	NewOpportunities     int `json:"new_opportunities"`
	FilteredOpportunities int `json:"filtered_opportunities,omitempty"`
	UpdatedOpportunities int `json:"updated_opportunities"`
	TotalValue           float64 `json:"total_value,omitempty"`
	UpcomingDeadlines    int `json:"upcoming_deadlines"`
}

// Attachment represents a file attachment
type Attachment struct {
	Name        string `json:"name"`
	Content     []byte `json:"content"`
	ContentType string `json:"content_type"`
}

// NotificationConfig holds configuration for notifications
type NotificationConfig struct {
	Email  EmailConfig  `json:"email"`
	Slack  SlackConfig  `json:"slack"`
	GitHub GitHubConfig `json:"github"`
}

// EmailConfig configures email notifications
type EmailConfig struct {
	Enabled     bool     `json:"enabled"`
	SMTPHost    string   `json:"smtp_host"`
	SMTPPort    int      `json:"smtp_port"`
	Username    string   `json:"username"`
	Password    string   `json:"password"`
	FromAddress string   `json:"from_address"`
	ToAddresses []string `json:"to_addresses"`
	UseTLS      bool     `json:"use_tls"`
}

// SlackConfig configures Slack notifications
type SlackConfig struct {
	Enabled    bool   `json:"enabled"`
	WebhookURL string `json:"webhook_url"`
	Channel    string `json:"channel,omitempty"`
	Username   string `json:"username,omitempty"`
	IconEmoji  string `json:"icon_emoji,omitempty"`
}

// GitHubConfig configures GitHub issue notifications
type GitHubConfig struct {
	Enabled     bool   `json:"enabled"`
	Token       string `json:"token"`
	Owner       string `json:"owner"`
	Repository  string `json:"repository"`
	Labels      []string `json:"labels"`
	AssignUsers []string `json:"assign_users,omitempty"`
}

// NotificationManager orchestrates multiple notification channels
type NotificationManager struct {
	notifiers []Notifier
	config    NotificationConfig
	verbose   bool
}

// NewNotificationManager creates a new notification manager
func NewNotificationManager(config NotificationConfig, verbose bool) *NotificationManager {
	manager := &NotificationManager{
		notifiers: make([]Notifier, 0),
		config:    config,
		verbose:   verbose,
	}

	// Initialize enabled notifiers
	if config.Email.Enabled {
		emailNotifier := NewEmailNotifier(config.Email, verbose)
		manager.notifiers = append(manager.notifiers, emailNotifier)
	}

	if config.Slack.Enabled {
		slackNotifier := NewSlackNotifier(config.Slack, verbose)
		manager.notifiers = append(manager.notifiers, slackNotifier)
	}

	if config.GitHub.Enabled {
		githubNotifier := NewGitHubNotifier(config.GitHub, verbose)
		manager.notifiers = append(manager.notifiers, githubNotifier)
		if verbose {
			log.Printf("Added GitHub notifier to notification manager")
		}
	} else if verbose {
		log.Printf("GitHub notifier DISABLED - not added to notification manager")
	}

	return manager
}

// SendNotification sends a notification through all enabled channels
func (nm *NotificationManager) SendNotification(ctx context.Context, notification Notification) error {
	if len(nm.notifiers) == 0 {
		return nil // No notifiers configured
	}

	// Send through all channels concurrently
	errChan := make(chan error, len(nm.notifiers))
	
	for _, notifier := range nm.notifiers {
		go func(n Notifier) {
			err := n.Send(ctx, notification)
			if err != nil {
				errChan <- err
			} else {
				errChan <- nil
			}
		}(notifier)
	}

	// Collect results
	var errors []error
	for i := 0; i < len(nm.notifiers); i++ {
		if err := <-errChan; err != nil {
			errors = append(errors, err)
		}
	}

	// Return combined error if any failed
	if len(errors) > 0 {
		return &MultiNotificationError{Errors: errors}
	}

	return nil
}

// GetEnabledNotifiers returns list of enabled notification types
func (nm *NotificationManager) GetEnabledNotifiers() []string {
	types := make([]string, 0, len(nm.notifiers))
	for _, notifier := range nm.notifiers {
		if notifier.IsEnabled() {
			types = append(types, notifier.GetType())
		}
	}
	return types
}

// MultiNotificationError represents errors from multiple notification channels
type MultiNotificationError struct {
	Errors []error `json:"errors"`
}

func (e *MultiNotificationError) Error() string {
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}
	
	msg := "multiple notification errors: "
	for i, err := range e.Errors {
		if i > 0 {
			msg += "; "
		}
		msg += err.Error()
	}
	return msg
}

// NotificationBuilder helps construct notifications
type NotificationBuilder struct {
	notification Notification
}

// NewNotificationBuilder creates a new notification builder
func NewNotificationBuilder() *NotificationBuilder {
	return &NotificationBuilder{
		notification: Notification{
			Timestamp: time.Now(),
			Metadata:  make(map[string]interface{}),
		},
	}
}

// WithQuery sets the query information
func (nb *NotificationBuilder) WithQuery(queryName string, priority Priority) *NotificationBuilder {
	nb.notification.QueryName = queryName
	nb.notification.Priority = priority
	return nb
}

// WithRecipients sets the notification recipients
func (nb *NotificationBuilder) WithRecipients(recipients []string) *NotificationBuilder {
	nb.notification.Recipients = recipients
	return nb
}

// WithOpportunities sets the opportunities to notify about
func (nb *NotificationBuilder) WithOpportunities(opportunities []samgov.Opportunity) *NotificationBuilder {
	nb.notification.Opportunities = opportunities
	
	// Calculate summary statistics
	summary := NotificationSummary{
		NewOpportunities: len(opportunities),
	}
	
	upcomingDeadlines := 0
	now := time.Now()
	cutoff := now.AddDate(0, 0, 30) // 30 days from now
	
	for _, opp := range opportunities {
		if opp.ResponseDeadline != nil {
			if deadline, err := time.Parse("2006-01-02", *opp.ResponseDeadline); err == nil {
				if deadline.After(now) && deadline.Before(cutoff) {
					upcomingDeadlines++
				}
			}
		}
	}
	
	summary.UpcomingDeadlines = upcomingDeadlines
	nb.notification.Summary = summary
	
	return nb
}

// WithUpdatedOpportunities marks opportunities as updated
func (nb *NotificationBuilder) WithUpdatedOpportunities(opportunities []samgov.Opportunity) *NotificationBuilder {
	nb.notification.Opportunities = opportunities
	nb.notification.Summary.UpdatedOpportunities = len(opportunities)
	nb.notification.Summary.NewOpportunities = 0
	return nb
}

// WithSubject sets the notification subject
func (nb *NotificationBuilder) WithSubject(subject string) *NotificationBuilder {
	nb.notification.Subject = subject
	return nb
}

// WithFilteredOpportunities sets opportunities that were filtered out
func (nb *NotificationBuilder) WithFilteredOpportunities(filteredOut []samgov.Opportunity) *NotificationBuilder {
	nb.notification.FilteredOut = filteredOut
	nb.notification.Summary.FilteredOpportunities = len(filteredOut)
	return nb
}

// WithMetadata adds custom metadata
func (nb *NotificationBuilder) WithMetadata(key string, value interface{}) *NotificationBuilder {
	nb.notification.Metadata[key] = value
	return nb
}

// Build returns the constructed notification
func (nb *NotificationBuilder) Build() Notification {
	return nb.notification
}

// ValidateNotification checks if a notification is properly constructed
func ValidateNotification(notification Notification) error {
	if notification.QueryName == "" {
		return &ValidationError{Field: "QueryName", Message: "query name is required"}
	}
	
	if notification.Subject == "" {
		return &ValidationError{Field: "Subject", Message: "subject is required"}
	}
	
	if len(notification.Opportunities) == 0 {
		return &ValidationError{Field: "Opportunities", Message: "at least one opportunity is required"}
	}
	
	if notification.Priority != PriorityHigh && notification.Priority != PriorityMedium && notification.Priority != PriorityLow {
		return &ValidationError{Field: "Priority", Message: "invalid priority level"}
	}
	
	return nil
}

// ValidationError represents a notification validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}