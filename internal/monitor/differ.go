package monitor

import (
	"crypto/sha256"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yourusername/sam-gov-monitor/internal/samgov"
)

// OpportunityDiffer handles detection of new and changed opportunities
type OpportunityDiffer struct {
	verbose bool
}

// NewOpportunityDiffer creates a new opportunity differ
func NewOpportunityDiffer(verbose bool) *OpportunityDiffer {
	return &OpportunityDiffer{
		verbose: verbose,
	}
}

// DiffOpportunities compares current opportunities against stored state
func (d *OpportunityDiffer) DiffOpportunities(current []samgov.Opportunity, state *State) samgov.DiffResult {
	result := samgov.DiffResult{
		New:      make([]samgov.Opportunity, 0),
		Updated:  make([]samgov.Opportunity, 0),
		Existing: make([]samgov.Opportunity, 0),
	}

	if d.verbose {
		log.Printf("Comparing %d current opportunities against state", len(current))
	}

	for _, opp := range current {
		classification := d.classifyOpportunity(opp, state)
		
		switch classification.Type {
		case "new":
			result.New = append(result.New, opp)
			if d.verbose {
				log.Printf("NEW: %s - %s", opp.NoticeID, opp.Title)
			}
			
		case "updated":
			result.Updated = append(result.Updated, opp)
			if d.verbose {
				log.Printf("UPDATED: %s - %s (Changes: %s)", 
					opp.NoticeID, opp.Title, strings.Join(classification.Changes, ", "))
			}
			
		case "existing":
			result.Existing = append(result.Existing, opp)
			if d.verbose && len(result.Existing) <= 3 { // Limit verbose output
				log.Printf("EXISTING: %s - %s", opp.NoticeID, opp.Title)
			}
		}
	}

	if d.verbose {
		log.Printf("Diff complete: %d new, %d updated, %d existing", 
			len(result.New), len(result.Updated), len(result.Existing))
	}

	return result
}

// OpportunityClassification describes how an opportunity has changed
type OpportunityClassification struct {
	Type     string   // "new", "updated", "existing"
	Changes  []string // List of fields that changed
	Previous *samgov.OpportunityState
}

// classifyOpportunity determines if an opportunity is new, updated, or existing
func (d *OpportunityDiffer) classifyOpportunity(current samgov.Opportunity, state *State) OpportunityClassification {
	previous, exists := state.GetOpportunity(current.NoticeID)
	if !exists {
		return OpportunityClassification{
			Type: "new",
		}
	}

	changes := d.detectChanges(previous, current)
	if len(changes) > 0 {
		return OpportunityClassification{
			Type:     "updated",
			Changes:  changes,
			Previous: &previous,
		}
	}

	return OpportunityClassification{
		Type:     "existing",
		Previous: &previous,
	}
}

// detectChanges identifies what fields have changed in an opportunity
func (d *OpportunityDiffer) detectChanges(previous samgov.OpportunityState, current samgov.Opportunity) []string {
	changes := make([]string, 0)

	// Check title change
	if previous.Title != current.Title {
		changes = append(changes, "title")
	}

	// Check deadline change
	prevDeadline := ""
	if previous.Deadline != nil {
		prevDeadline = *previous.Deadline
	}
	
	currentDeadline := ""
	if current.ResponseDeadline != nil {
		currentDeadline = *current.ResponseDeadline
	}
	
	if prevDeadline != currentDeadline {
		changes = append(changes, "deadline")
	}

	// Check content hash change (detects description or other field changes)
	currentHash := d.calculateContentHash(current)
	if previous.Hash != currentHash {
		// Only add if we haven't already detected specific changes
		if len(changes) == 0 {
			changes = append(changes, "content")
		}
	}

	return changes
}

// calculateContentHash creates a hash of the opportunity's content for change detection
func (d *OpportunityDiffer) calculateContentHash(opp samgov.Opportunity) string {
	// Include key fields that we care about for change detection
	content := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s",
		opp.NoticeID,
		opp.Title,
		opp.PostedDate,
		opp.Type,
		opp.TypeOfSetAside,
		opp.NAICSCode,
		func() string {
			if opp.ResponseDeadline != nil {
				return *opp.ResponseDeadline
			}
			return ""
		}(),
		// Truncate description for hash to avoid noise from minor formatting changes
		func() string {
			desc := strings.TrimSpace(opp.Description)
			if len(desc) > 500 {
				desc = desc[:500]
			}
			return desc
		}(),
	)

	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)
}

// FilterSignificantChanges filters out minor changes that don't warrant notifications
func (d *OpportunityDiffer) FilterSignificantChanges(updated []samgov.Opportunity, state *State) []samgov.Opportunity {
	significant := make([]samgov.Opportunity, 0)

	for _, opp := range updated {
		if d.isSignificantChange(opp, state) {
			significant = append(significant, opp)
		} else if d.verbose {
			log.Printf("Filtering out minor change for %s - %s", opp.NoticeID, opp.Title)
		}
	}

	return significant
}

// isSignificantChange determines if a change is significant enough to notify about
func (d *OpportunityDiffer) isSignificantChange(current samgov.Opportunity, state *State) bool {
	previous, exists := state.GetOpportunity(current.NoticeID)
	if !exists {
		return true // New opportunities are always significant
	}

	changes := d.detectChanges(previous, current)
	
	// Title changes are always significant
	for _, change := range changes {
		if change == "title" {
			return true
		}
	}

	// Deadline changes are significant
	for _, change := range changes {
		if change == "deadline" {
			return true
		}
	}

	// Content changes are significant if the opportunity was seen recently
	// (within last 7 days), indicating it's an active opportunity
	if len(changes) > 0 {
		daysSinceFirstSeen := int(time.Since(previous.FirstSeen).Hours() / 24)
		if daysSinceFirstSeen <= 7 {
			return true
		}
	}

	return false
}

// DetectExpiredOpportunities finds opportunities that are no longer active
func (d *OpportunityDiffer) DetectExpiredOpportunities(current []samgov.Opportunity, state *State, maxAge time.Duration) []samgov.OpportunityState {
	expired := make([]samgov.OpportunityState, 0)
	
	// Create a map of current opportunity IDs for quick lookup
	currentIDs := make(map[string]bool)
	for _, opp := range current {
		currentIDs[opp.NoticeID] = true
	}

	// Check stored opportunities
	cutoff := time.Now().Add(-maxAge)
	for noticeID, stored := range state.Opportunities {
		// Skip if we saw this opportunity in current results
		if currentIDs[noticeID] {
			continue
		}

		// Consider expired if not seen recently and past cutoff
		if stored.LastSeen.Before(cutoff) {
			expired = append(expired, stored)
		}
	}

	if d.verbose && len(expired) > 0 {
		log.Printf("Detected %d expired opportunities", len(expired))
	}

	return expired
}

// AnalyzeOpportunityTrends provides insights about opportunity patterns
func (d *OpportunityDiffer) AnalyzeOpportunityTrends(current []samgov.Opportunity, state *State) TrendAnalysis {
	analysis := TrendAnalysis{
		TotalOpportunities: len(current),
		ByType:            make(map[string]int),
		ByOrganization:    make(map[string]int),
		BySetAside:        make(map[string]int),
		NewToday:          0,
		NewThisWeek:       0,
	}

	today := time.Now().Truncate(24 * time.Hour)
	weekAgo := today.AddDate(0, 0, -7)

	for _, opp := range current {
		// Count by type
		analysis.ByType[opp.Type]++
		
		// Count by organization
		analysis.ByOrganization[opp.FullParentPath]++
		
		// Count by set-aside type
		if opp.TypeOfSetAside != "" {
			analysis.BySetAside[opp.TypeOfSetAside]++
		}

		// Check if this is new
		if stored, exists := state.GetOpportunity(opp.NoticeID); exists {
			if stored.FirstSeen.After(today) {
				analysis.NewToday++
			}
			if stored.FirstSeen.After(weekAgo) {
				analysis.NewThisWeek++
			}
		}
	}

	return analysis
}

// TrendAnalysis provides statistical insights about opportunities
type TrendAnalysis struct {
	TotalOpportunities int            `json:"total_opportunities"`
	ByType            map[string]int `json:"by_type"`
	ByOrganization    map[string]int `json:"by_organization"`
	BySetAside        map[string]int `json:"by_set_aside"`
	NewToday          int            `json:"new_today"`
	NewThisWeek       int            `json:"new_this_week"`
}

// GenerateDiffReport creates a human-readable report of changes
func (d *OpportunityDiffer) GenerateDiffReport(diff samgov.DiffResult, queryName string) string {
	var report strings.Builder
	
	report.WriteString(fmt.Sprintf("=== Diff Report for Query: %s ===\n", queryName))
	report.WriteString(fmt.Sprintf("New Opportunities: %d\n", len(diff.New)))
	report.WriteString(fmt.Sprintf("Updated Opportunities: %d\n", len(diff.Updated)))
	report.WriteString(fmt.Sprintf("Existing Opportunities: %d\n", len(diff.Existing)))
	report.WriteString("\n")

	if len(diff.New) > 0 {
		report.WriteString("NEW OPPORTUNITIES:\n")
		for i, opp := range diff.New {
			if i >= 5 { // Limit to first 5 for readability
				report.WriteString(fmt.Sprintf("... and %d more\n", len(diff.New)-5))
				break
			}
			report.WriteString(fmt.Sprintf("  • %s: %s (Posted: %s)\n", 
				opp.NoticeID, opp.Title, opp.PostedDate))
		}
		report.WriteString("\n")
	}

	if len(diff.Updated) > 0 {
		report.WriteString("UPDATED OPPORTUNITIES:\n")
		for i, opp := range diff.Updated {
			if i >= 5 { // Limit to first 5 for readability
				report.WriteString(fmt.Sprintf("... and %d more\n", len(diff.Updated)-5))
				break
			}
			report.WriteString(fmt.Sprintf("  • %s: %s\n", opp.NoticeID, opp.Title))
		}
		report.WriteString("\n")
	}

	return report.String()
}