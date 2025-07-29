package monitor

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/yourusername/sam-gov-monitor/internal/samgov"
)

// State manages the persistent state of monitored opportunities
type State struct {
	mu            sync.RWMutex
	Opportunities map[string]samgov.OpportunityState `json:"opportunities"`
	LastRun       time.Time                          `json:"last_run"`
	QueryMetrics  map[string]QueryMetrics            `json:"query_metrics"`
	filepath      string
	modified      bool
}


// LoadState loads the state from a file, or creates a new empty state
func LoadState(filePath string) (*State, error) {
	state := &State{
		Opportunities: make(map[string]samgov.OpportunityState),
		QueryMetrics:  make(map[string]QueryMetrics),
		filepath:      filePath,
	}

	if filePath == "" {
		return state, nil // In-memory only
	}

	// Create directory if it doesn't exist
	if dir := filepath.Dir(filePath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("creating state directory: %w", err)
		}
	}

	// Try to load existing state
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, start with empty state
			return state, nil
		}
		return nil, fmt.Errorf("reading state file %s: %w", filePath, err)
	}

	// Parse JSON
	if err := json.Unmarshal(data, state); err != nil {
		// If we can't parse, log warning and start fresh
		fmt.Printf("Warning: corrupt state file %s, starting fresh: %v\n", filePath, err)
		return &State{
			Opportunities: make(map[string]samgov.OpportunityState),
			QueryMetrics:  make(map[string]QueryMetrics),
			filepath:      filePath,
		}, nil
	}

	// Initialize maps if they're nil (backward compatibility)
	if state.Opportunities == nil {
		state.Opportunities = make(map[string]samgov.OpportunityState)
	}
	if state.QueryMetrics == nil {
		state.QueryMetrics = make(map[string]QueryMetrics)
	}

	return state, nil
}

// Save persists the state to disk
func (s *State) Save() error {
	if s.filepath == "" {
		return nil // In-memory only
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.modified {
		return nil // No changes to save
	}

	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(s.filepath), 0755); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}

	// Write atomically by writing to temp file then renaming
	tempFile := s.filepath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("writing temp state file: %w", err)
	}

	if err := os.Rename(tempFile, s.filepath); err != nil {
		os.Remove(tempFile) // Clean up temp file
		return fmt.Errorf("renaming state file: %w", err)
	}

	s.modified = false
	return nil
}

// AddOpportunity adds or updates an opportunity in the state
func (s *State) AddOpportunity(opp samgov.Opportunity) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	hash := s.calculateOpportunityHash(opp)

	if existing, exists := s.Opportunities[opp.NoticeID]; exists {
		// Update existing opportunity
		existing.LastSeen = now
		if existing.Hash != hash {
			existing.LastModified = now
			existing.Hash = hash
			existing.Title = opp.Title
			existing.Deadline = opp.ResponseDeadline
		}
		s.Opportunities[opp.NoticeID] = existing
		s.modified = true
		return false // Not new
	}

	// Add new opportunity
	s.Opportunities[opp.NoticeID] = samgov.OpportunityState{
		FirstSeen:    now,
		LastSeen:     now,
		LastModified: now,
		NoticeID:     opp.NoticeID,
		Title:        opp.Title,
		Deadline:     opp.ResponseDeadline,
		Hash:         hash,
	}
	s.modified = true
	return true // New opportunity
}

// GetOpportunity retrieves an opportunity from state
func (s *State) GetOpportunity(noticeID string) (samgov.OpportunityState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	opp, exists := s.Opportunities[noticeID]
	return opp, exists
}

// SetLastRun updates the last run timestamp
func (s *State) SetLastRun(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.LastRun = t
	s.modified = true
}

// GetLastRun returns the last run timestamp
func (s *State) GetLastRun() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.LastRun
}

// UpdateQueryMetrics updates metrics for a query
func (s *State) UpdateQueryMetrics(queryName string, executionTime time.Duration, opportunityCount int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	metrics, exists := s.QueryMetrics[queryName]
	if !exists {
		metrics = QueryMetrics{}
	}

	metrics.LastExecuted = time.Now()
	metrics.ExecutionCount++
	metrics.LastOpportunityCount = opportunityCount
	metrics.TotalOpportunitiesFound += opportunityCount

	// Update average execution time
	if metrics.ExecutionCount == 1 {
		metrics.AverageTime = executionTime
	} else {
		// Rolling average
		total := time.Duration(metrics.ExecutionCount-1) * metrics.AverageTime + executionTime
		metrics.AverageTime = total / time.Duration(metrics.ExecutionCount)
	}

	if err != nil {
		metrics.ErrorCount++
		metrics.LastError = err.Error()
	} else {
		metrics.LastError = ""
	}

	s.QueryMetrics[queryName] = metrics
	s.modified = true
}

// GetQueryMetrics returns metrics for a query
func (s *State) GetQueryMetrics(queryName string) (QueryMetrics, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metrics, exists := s.QueryMetrics[queryName]
	return metrics, exists
}

// CleanupOldOpportunities removes opportunities older than the specified duration
func (s *State) CleanupOldOpportunities(maxAge time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for noticeID, opp := range s.Opportunities {
		if opp.LastSeen.Before(cutoff) {
			delete(s.Opportunities, noticeID)
			removed++
		}
	}

	if removed > 0 {
		s.modified = true
	}

	return removed
}

// GetStats returns statistics about the current state
func (s *State) GetStats() StateStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := StateStats{
		TotalOpportunities: len(s.Opportunities),
		LastRun:           s.LastRun,
		TotalQueries:      len(s.QueryMetrics),
	}

	// Calculate age distribution
	now := time.Now()
	for _, opp := range s.Opportunities {
		daysSince := int(now.Sub(opp.FirstSeen).Hours() / 24)
		if daysSince <= 7 {
			stats.OpportunitiesLastWeek++
		}
		if daysSince <= 30 {
			stats.OpportunitiesLastMonth++
		}
	}

	// Calculate query success rate
	totalExecutions := 0
	successfulExecutions := 0
	for _, metrics := range s.QueryMetrics {
		totalExecutions += metrics.ExecutionCount
		successfulExecutions += metrics.ExecutionCount - metrics.ErrorCount
	}

	if totalExecutions > 0 {
		stats.QuerySuccessRate = float64(successfulExecutions) / float64(totalExecutions)
	}

	return stats
}

// StateStats provides statistics about the state
type StateStats struct {
	TotalOpportunities      int       `json:"total_opportunities"`
	OpportunitiesLastWeek   int       `json:"opportunities_last_week"`
	OpportunitiesLastMonth  int       `json:"opportunities_last_month"`
	LastRun                 time.Time `json:"last_run"`
	TotalQueries            int       `json:"total_queries"`
	QuerySuccessRate        float64   `json:"query_success_rate"`
}

// calculateOpportunityHash creates a hash of the opportunity content for change detection
func (s *State) calculateOpportunityHash(opp samgov.Opportunity) string {
	// Create a string representation of key fields
	content := fmt.Sprintf("%s|%s|%s|%s|%s", 
		opp.NoticeID, 
		opp.Title, 
		opp.PostedDate,
		opp.Type,
		func() string {
			if opp.ResponseDeadline != nil {
				return *opp.ResponseDeadline
			}
			return ""
		}(),
	)

	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)
}

// ExportToJSON exports the state to a JSON string for backup/debugging
func (s *State) ExportToJSON() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling state for export: %w", err)
	}

	return string(data), nil
}

// GetOpportunitiesByAge returns opportunities grouped by age
func (s *State) GetOpportunitiesByAge() map[string][]samgov.OpportunityState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	groups := map[string][]samgov.OpportunityState{
		"today":     make([]samgov.OpportunityState, 0),
		"this_week": make([]samgov.OpportunityState, 0),
		"this_month": make([]samgov.OpportunityState, 0),
		"older":     make([]samgov.OpportunityState, 0),
	}

	for _, opp := range s.Opportunities {
		daysSince := int(now.Sub(opp.FirstSeen).Hours() / 24)
		
		if daysSince == 0 {
			groups["today"] = append(groups["today"], opp)
		} else if daysSince <= 7 {
			groups["this_week"] = append(groups["this_week"], opp)
		} else if daysSince <= 30 {
			groups["this_month"] = append(groups["this_month"], opp)
		} else {
			groups["older"] = append(groups["older"], opp)
		}
	}

	return groups
}