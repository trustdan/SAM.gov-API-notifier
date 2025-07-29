package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"text/template"
	"time"

	"github.com/yourusername/sam-gov-monitor/internal/samgov"
)

// GitHubNotifier implements GitHub issue notifications
type GitHubNotifier struct {
	config    GitHubConfig
	verbose   bool
	client    *http.Client
	templates *template.Template
}

// NewGitHubNotifier creates a new GitHub notifier
func NewGitHubNotifier(config GitHubConfig, verbose bool) *GitHubNotifier {
	notifier := &GitHubNotifier{
		config:  config,
		verbose: verbose,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	
	notifier.loadTemplates()
	return notifier
}

// Send creates GitHub issues for opportunities
func (gn *GitHubNotifier) Send(ctx context.Context, notification Notification) error {
	if !gn.config.Enabled {
		return nil
	}
	
	// Verify configuration is complete
	if gn.config.Token == "" || gn.config.Owner == "" || gn.config.Repository == "" {
		return fmt.Errorf("GitHub notifier enabled but missing required configuration (token/owner/repository)")
	}

	if gn.verbose {
		log.Printf("Creating GitHub issues for notification: %s", notification.Subject)
	}

	// Create issues for high-priority opportunities individually
	// For lower priority, create one summary issue
	if notification.Priority == PriorityHigh {
		return gn.createIndividualIssues(ctx, notification)
	} else {
		return gn.createSummaryIssue(ctx, notification)
	}
}

// GetType returns the notifier type
func (gn *GitHubNotifier) GetType() string {
	return "github"
}

// IsEnabled returns whether GitHub notifications are enabled
func (gn *GitHubNotifier) IsEnabled() bool {
	return gn.config.Enabled
}

// createIndividualIssues creates separate issues for each opportunity
func (gn *GitHubNotifier) createIndividualIssues(ctx context.Context, notification Notification) error {
	for _, opp := range notification.Opportunities {
		if err := gn.createOpportunityIssue(ctx, opp, notification); err != nil {
			return fmt.Errorf("creating issue for %s: %w", opp.NoticeID, err)
		}
	}
	return nil
}

// createSummaryIssue creates one issue summarizing all opportunities
func (gn *GitHubNotifier) createSummaryIssue(ctx context.Context, notification Notification) error {
	title := fmt.Sprintf("%s - %d New Opportunities", 
		notification.QueryName, len(notification.Opportunities))
	
	body, err := gn.buildSummaryIssueBody(notification)
	if err != nil {
		return fmt.Errorf("building summary issue body: %w", err)
	}

	labels := gn.buildLabels(notification.Priority, false)
	
	issue := &GitHubIssue{
		Title:     title,
		Body:      body,
		Labels:    labels,
		Assignees: gn.config.AssignUsers,
	}

	return gn.createIssue(ctx, issue)
}

// createOpportunityIssue creates an issue for a single opportunity
func (gn *GitHubNotifier) createOpportunityIssue(ctx context.Context, opp samgov.Opportunity, notification Notification) error {
	title := fmt.Sprintf("🚨 %s - %s", opp.NoticeID, opp.Title)
	
	body, err := gn.buildOpportunityIssueBody(opp, notification)
	if err != nil {
		return fmt.Errorf("building opportunity issue body: %w", err)
	}

	labels := gn.buildLabels(notification.Priority, true)
	
	// Add deadline label if urgent
	if opp.ResponseDeadline != nil && gn.isUrgentDeadline(*opp.ResponseDeadline) {
		labels = append(labels, "urgent-deadline")
	}

	issue := &GitHubIssue{
		Title:     title,
		Body:      body,
		Labels:    labels,
		Assignees: gn.config.AssignUsers,
	}

	return gn.createIssue(ctx, issue)
}

// buildOpportunityIssueBody creates issue body for a single opportunity
func (gn *GitHubNotifier) buildOpportunityIssueBody(opp samgov.Opportunity, notification Notification) (string, error) {
	data := GitHubTemplateData{
		Opportunity:   opp,
		QueryName:     notification.QueryName,
		Priority:      string(notification.Priority),
		Timestamp:     notification.Timestamp,
		IsIndividual:  true,
	}

	var buf bytes.Buffer
	if err := gn.templates.ExecuteTemplate(&buf, "individual-issue", data); err != nil {
		return "", fmt.Errorf("executing individual issue template: %w", err)
	}

	return buf.String(), nil
}

// buildSummaryIssueBody creates issue body for summary of opportunities
func (gn *GitHubNotifier) buildSummaryIssueBody(notification Notification) (string, error) {
	data := GitHubTemplateData{
		Opportunities: notification.Opportunities,
		QueryName:     notification.QueryName,
		Priority:      string(notification.Priority),
		Timestamp:     notification.Timestamp,
		Summary:       notification.Summary,
		IsIndividual:  false,
	}

	var buf bytes.Buffer
	if err := gn.templates.ExecuteTemplate(&buf, "summary-issue", data); err != nil {
		return "", fmt.Errorf("executing summary issue template: %w", err)
	}

	return buf.String(), nil
}

// createIssue sends issue creation request to GitHub API
func (gn *GitHubNotifier) createIssue(ctx context.Context, issue *GitHubIssue) error {
	// Build API URL
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues", 
		gn.config.Owner, gn.config.Repository)

	// Marshal issue to JSON
	jsonData, err := json.Marshal(issue)
	if err != nil {
		return fmt.Errorf("marshaling issue: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("token %s", gn.config.Token))
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "SAM.gov-Monitor/1.0")

	// Send request
	resp, err := gn.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	if gn.verbose {
		log.Printf("GitHub issue created successfully: %s", issue.Title)
	}

	return nil
}

// buildLabels creates labels for the issue
func (gn *GitHubNotifier) buildLabels(priority Priority, isIndividual bool) []string {
	labels := make([]string, 0)
	
	// Add configured labels
	labels = append(labels, gn.config.Labels...)
	
	// Add priority label
	labels = append(labels, fmt.Sprintf("priority-%s", string(priority)))
	
	// Add type label
	if isIndividual {
		labels = append(labels, "individual-opportunity")
	} else {
		labels = append(labels, "opportunity-summary")
	}
	
	// Add SAM.gov label
	labels = append(labels, "sam-gov")
	
	return labels
}

// isUrgentDeadline checks if deadline is within 7 days
func (gn *GitHubNotifier) isUrgentDeadline(deadlineStr string) bool {
	deadline, err := time.Parse("2006-01-02", deadlineStr)
	if err != nil {
		return false
	}
	
	return time.Until(deadline) <= 7*24*time.Hour
}

// loadTemplates loads GitHub issue templates
func (gn *GitHubNotifier) loadTemplates() {
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}
	
	gn.templates = template.Must(template.New("github").Funcs(funcMap).Parse(individualIssueTemplate))
	template.Must(gn.templates.New("summary-issue").Funcs(funcMap).Parse(summaryIssueTemplate))
}

// GitHubIssue represents a GitHub issue
type GitHubIssue struct {
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	Labels    []string `json:"labels,omitempty"`
	Assignees []string `json:"assignees,omitempty"`
}

// GitHubTemplateData holds data for GitHub templates
type GitHubTemplateData struct {
	Opportunity   samgov.Opportunity      `json:"opportunity,omitempty"`
	Opportunities []samgov.Opportunity    `json:"opportunities,omitempty"`
	QueryName     string                  `json:"query_name"`
	Priority      string                  `json:"priority"`
	Timestamp     time.Time               `json:"timestamp"`
	Summary       NotificationSummary     `json:"summary"`
	IsIndividual  bool                    `json:"is_individual"`
}

// GitHub issue templates
const individualIssueTemplate = `## 🚨 New SAM.gov Opportunity: {{.Opportunity.Title}}

**Notice ID:** {{.Opportunity.NoticeID}}  
**Organization:** {{.Opportunity.FullParentPath}}  
**Posted:** {{.Opportunity.PostedDate}}  
{{if .Opportunity.ResponseDeadline}}**Deadline:** ⏰ {{.Opportunity.ResponseDeadline}}{{end}}

### Quick Actions
- [View on SAM.gov]({{.Opportunity.UILink}})
- [Download Solicitation]({{.Opportunity.UILink}}?tab=documents)

### Opportunity Details
{{if .Opportunity.Type}}**Type:** {{.Opportunity.Type}}{{end}}
{{if .Opportunity.TypeOfSetAside}}**Set-Aside:** {{.Opportunity.TypeOfSetAside}}{{end}}
{{if .Opportunity.NAICSCode}}**NAICS Code:** {{.Opportunity.NAICSCode}}{{end}}

{{if .Opportunity.Description}}
### Description
{{.Opportunity.Description}}
{{end}}

### Next Steps
- [ ] Review requirements and eligibility
- [ ] Go/No-Go decision
- [ ] Assign proposal team
- [ ] Create proposal timeline
- [ ] Prepare questions for Q&A period

---
**Query:** {{.QueryName}} | **Priority:** {{.Priority}} | **Generated:** {{.Timestamp.Format "Jan 2, 2006 at 3:04 PM MST"}}

🤖 _Automated issue created by SAM.gov Monitor_`

const summaryIssueTemplate = `## 📋 SAM.gov Opportunities Summary - {{.QueryName}}

Found **{{len .Opportunities}} new opportunities** matching your search criteria.

{{if .Summary.UpcomingDeadlines}}
### ⚠️ Urgent: {{.Summary.UpcomingDeadlines}} opportunities with deadlines in the next 30 days
{{end}}

### Opportunities Found

{{range $index, $opp := .Opportunities}}
#### {{add $index 1}}. [{{$opp.Title}}]({{$opp.UILink}})
- **Notice ID:** {{$opp.NoticeID}}
- **Organization:** {{$opp.FullParentPath}}
- **Posted:** {{$opp.PostedDate}}
{{if $opp.ResponseDeadline}}  - **Deadline:** ⏰ {{$opp.ResponseDeadline}}{{end}}
{{if $opp.TypeOfSetAside}}  - **Set-Aside:** {{$opp.TypeOfSetAside}}{{end}}
{{if $opp.NAICSCode}}  - **NAICS:** {{$opp.NAICSCode}}{{end}}

{{end}}

### Batch Actions
- [ ] Review all opportunities for relevance
- [ ] Prioritize by deadline and value
- [ ] Assign opportunities to team members
- [ ] Create individual tracking issues for high-priority opportunities

### Summary Statistics
- **Total Opportunities:** {{len .Opportunities}}
- **Query Priority:** {{.Priority}}
- **Search Query:** {{.QueryName}}

---
**Generated:** {{.Timestamp.Format "Jan 2, 2006 at 3:04 PM MST"}}

🤖 _Automated issue created by SAM.gov Monitor_`