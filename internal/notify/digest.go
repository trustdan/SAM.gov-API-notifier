package notify

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/yourusername/sam-gov-monitor/internal/samgov"
)

// DigestManager handles notification grouping and batching
type DigestManager struct {
	notifications []PendingNotification
	verbose       bool
}

// PendingNotification represents a notification waiting to be sent
type PendingNotification struct {
	Notification Notification  `json:"notification"`
	QueryName    string        `json:"query_name"`
	Priority     Priority      `json:"priority"`
	CreatedAt    time.Time     `json:"created_at"`
}

// NewDigestManager creates a new digest manager
func NewDigestManager(verbose bool) *DigestManager {
	return &DigestManager{
		notifications: make([]PendingNotification, 0),
		verbose:       verbose,
	}
}

// AddNotification adds a notification to the digest queue
func (dm *DigestManager) AddNotification(notification Notification) {
	pending := PendingNotification{
		Notification: notification,
		QueryName:    notification.QueryName,
		Priority:     notification.Priority,
		CreatedAt:    time.Now(),
	}
	
	dm.notifications = append(dm.notifications, pending)
	
	if dm.verbose {
		log.Printf("Added notification to digest queue: %s (priority: %s)", 
			notification.QueryName, notification.Priority)
	}
}

// ShouldSendImmediately determines if a notification should bypass digest mode
func (dm *DigestManager) ShouldSendImmediately(notification Notification) bool {
	// High priority notifications always sent immediately
	if notification.Priority == PriorityHigh {
		return true
	}
	
	// Notifications with urgent deadlines
	if dm.hasUrgentDeadlines(notification.Opportunities) {
		return true
	}
	
	// Large number of opportunities might be worth immediate attention
	if len(notification.Opportunities) >= 10 {
		return true
	}
	
	return false
}

// ProcessDigest creates digest notifications and clears the queue
func (dm *DigestManager) ProcessDigest(ctx context.Context, notifyMgr *NotificationManager) error {
	if len(dm.notifications) == 0 {
		return nil
	}
	
	if dm.verbose {
		log.Printf("Processing digest with %d pending notifications", len(dm.notifications))
	}
	
	// Group notifications by priority and query
	groups := dm.groupNotifications()
	
	// Send digest for each group
	for groupKey, notifications := range groups {
		digest, err := dm.createDigestNotification(groupKey, notifications)
		if err != nil {
			return fmt.Errorf("creating digest for %s: %w", groupKey, err)
		}
		
		if err := notifyMgr.SendNotification(ctx, digest); err != nil {
			return fmt.Errorf("sending digest for %s: %w", groupKey, err)
		}
		
		if dm.verbose {
			log.Printf("Sent digest notification for %s with %d items", groupKey, len(notifications))
		}
	}
	
	// Clear the queue
	dm.notifications = dm.notifications[:0]
	
	return nil
}

// groupNotifications groups pending notifications by priority and query type
func (dm *DigestManager) groupNotifications() map[string][]PendingNotification {
	groups := make(map[string][]PendingNotification)
	
	for _, pending := range dm.notifications {
		// Group by priority level
		groupKey := fmt.Sprintf("%s-priority", string(pending.Priority))
		
		if groups[groupKey] == nil {
			groups[groupKey] = make([]PendingNotification, 0)
		}
		
		groups[groupKey] = append(groups[groupKey], pending)
	}
	
	// Sort each group by creation time (oldest first)
	for _, group := range groups {
		sort.Slice(group, func(i, j int) bool {
			return group[i].CreatedAt.Before(group[j].CreatedAt)
		})
	}
	
	return groups
}

// createDigestNotification creates a digest notification from grouped notifications
func (dm *DigestManager) createDigestNotification(groupKey string, notifications []PendingNotification) (Notification, error) {
	if len(notifications) == 0 {
		return Notification{}, fmt.Errorf("empty notification group")
	}
	
	// Extract priority from group key
	priority := PriorityMedium
	if len(notifications) > 0 {
		priority = notifications[0].Priority
	}
	
	// Collect all opportunities
	allOpportunities := make([]samgov.Opportunity, 0)
	queryNames := make(map[string]bool)
	totalNew := 0
	totalUpdated := 0
	
	for _, pending := range notifications {
		allOpportunities = append(allOpportunities, pending.Notification.Opportunities...)
		queryNames[pending.QueryName] = true
		totalNew += pending.Notification.Summary.NewOpportunities
		totalUpdated += pending.Notification.Summary.UpdatedOpportunities
	}
	
	// Create query names list
	queries := make([]string, 0, len(queryNames))
	for name := range queryNames {
		queries = append(queries, name)
	}
	sort.Strings(queries)
	
	// Build digest subject
	subject := dm.buildDigestSubject(priority, totalNew, totalUpdated, queries)
	
	// Build digest notification
	digestNotification := NewNotificationBuilder().
		WithQuery(fmt.Sprintf("Digest (%s)", groupKey), priority).
		WithOpportunities(allOpportunities).
		WithSubject(subject).
		WithMetadata("digest", true).
		WithMetadata("query_count", len(queries)).
		WithMetadata("notification_count", len(notifications)).
		Build()
	
	// Update summary with correct counts
	digestNotification.Summary = NotificationSummary{
		NewOpportunities:     totalNew,
		UpdatedOpportunities: totalUpdated,
		UpcomingDeadlines:    dm.countUpcomingDeadlines(allOpportunities),
	}
	
	return digestNotification, nil
}

// buildDigestSubject creates an appropriate subject line for digest notifications
func (dm *DigestManager) buildDigestSubject(priority Priority, newCount, updatedCount int, queries []string) string {
	emoji := "📊"
	if priority == PriorityHigh {
		emoji = "🚨"
	} else if priority == PriorityLow {
		emoji = "📋"
	}
	
	totalCount := newCount + updatedCount
	
	subject := fmt.Sprintf("%s Daily Digest: %d SAM.gov Opportunities", emoji, totalCount)
	
	if len(queries) == 1 {
		subject += fmt.Sprintf(" - %s", queries[0])
	} else if len(queries) <= 3 {
		subject += fmt.Sprintf(" - %s", joinQueries(queries))
	} else {
		subject += fmt.Sprintf(" - %d Queries", len(queries))
	}
	
	return subject
}

// hasUrgentDeadlines checks if any opportunities have deadlines within 3 days
func (dm *DigestManager) hasUrgentDeadlines(opportunities []samgov.Opportunity) bool {
	urgentCutoff := time.Now().AddDate(0, 0, 3) // 3 days from now
	
	for _, opp := range opportunities {
		if opp.ResponseDeadline == nil {
			continue
		}
		
		deadline, err := time.Parse("2006-01-02", *opp.ResponseDeadline)
		if err != nil {
			continue
		}
		
		if deadline.Before(urgentCutoff) && deadline.After(time.Now()) {
			return true
		}
	}
	
	return false
}

// countUpcomingDeadlines counts deadlines in the next 30 days
func (dm *DigestManager) countUpcomingDeadlines(opportunities []samgov.Opportunity) int {
	count := 0
	cutoff := time.Now().AddDate(0, 0, 30) // 30 days from now
	
	for _, opp := range opportunities {
		if opp.ResponseDeadline == nil {
			continue
		}
		
		deadline, err := time.Parse("2006-01-02", *opp.ResponseDeadline)
		if err != nil {
			continue
		}
		
		if deadline.After(time.Now()) && deadline.Before(cutoff) {
			count++
		}
	}
	
	return count
}

// GetPendingCount returns the number of pending notifications
func (dm *DigestManager) GetPendingCount() int {
	return len(dm.notifications)
}

// GetPendingByPriority returns pending notifications grouped by priority
func (dm *DigestManager) GetPendingByPriority() map[Priority]int {
	counts := make(map[Priority]int)
	
	for _, pending := range dm.notifications {
		counts[pending.Priority]++
	}
	
	return counts
}

// ClearPending removes all pending notifications (useful for testing)
func (dm *DigestManager) ClearPending() {
	dm.notifications = dm.notifications[:0]
}

// GetOldestPending returns the creation time of the oldest pending notification
func (dm *DigestManager) GetOldestPending() *time.Time {
	if len(dm.notifications) == 0 {
		return nil
	}
	
	oldest := dm.notifications[0].CreatedAt
	for _, pending := range dm.notifications[1:] {
		if pending.CreatedAt.Before(oldest) {
			oldest = pending.CreatedAt
		}
	}
	
	return &oldest
}

// ShouldProcessDigest determines if it's time to send digest notifications
func (dm *DigestManager) ShouldProcessDigest(maxAge time.Duration) bool {
	if len(dm.notifications) == 0 {
		return false
	}
	
	oldest := dm.GetOldestPending()
	if oldest == nil {
		return false
	}
	
	return time.Since(*oldest) >= maxAge
}

// Enhanced notification manager with digest support
type DigestNotificationManager struct {
	*NotificationManager
	digest       *DigestManager
	digestMode   bool
	digestMaxAge time.Duration
}

// NewDigestNotificationManager creates a notification manager with digest support
func NewDigestNotificationManager(config NotificationConfig, verbose bool, digestMode bool) *DigestNotificationManager {
	return &DigestNotificationManager{
		NotificationManager: NewNotificationManager(config, verbose),
		digest:              NewDigestManager(verbose),
		digestMode:          digestMode,
		digestMaxAge:        4 * time.Hour, // Process digest every 4 hours
	}
}

// SendNotificationWithDigest sends notification immediately or adds to digest queue
func (dnm *DigestNotificationManager) SendNotificationWithDigest(ctx context.Context, notification Notification) error {
	if !dnm.digestMode || dnm.digest.ShouldSendImmediately(notification) {
		// Send immediately
		return dnm.NotificationManager.SendNotification(ctx, notification)
	}
	
	// Add to digest queue
	dnm.digest.AddNotification(notification)
	
	// Check if we should process digest now
	if dnm.digest.ShouldProcessDigest(dnm.digestMaxAge) {
		return dnm.digest.ProcessDigest(ctx, dnm.NotificationManager)
	}
	
	return nil
}

// ProcessPendingDigests processes any pending digest notifications
func (dnm *DigestNotificationManager) ProcessPendingDigests(ctx context.Context) error {
	return dnm.digest.ProcessDigest(ctx, dnm.NotificationManager)
}

// GetDigestStats returns statistics about the digest queue
func (dnm *DigestNotificationManager) GetDigestStats() DigestStats {
	return DigestStats{
		PendingCount:     dnm.digest.GetPendingCount(),
		PendingByPriority: dnm.digest.GetPendingByPriority(),
		OldestPending:    dnm.digest.GetOldestPending(),
		DigestMode:       dnm.digestMode,
	}
}

// DigestStats provides information about the digest queue
type DigestStats struct {
	PendingCount      int                `json:"pending_count"`
	PendingByPriority map[Priority]int   `json:"pending_by_priority"`
	OldestPending     *time.Time         `json:"oldest_pending,omitempty"`
	DigestMode        bool               `json:"digest_mode"`
}

// Helper function to join query names with proper grammar
func joinQueries(queries []string) string {
	if len(queries) == 0 {
		return ""
	}
	if len(queries) == 1 {
		return queries[0]
	}
	if len(queries) == 2 {
		return queries[0] + " and " + queries[1]
	}
	
	result := ""
	for i, query := range queries {
		if i == len(queries)-1 {
			result += "and " + query
		} else if i > 0 {
			result += ", " + query
		} else {
			result += query
		}
	}
	
	return result
}