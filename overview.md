# SAM.gov Opportunity Monitor (Go) - System Overview

## Executive Summary

High-performance monitoring system for SAM.gov opportunities built in Go. Leverages Go's concurrency for parallel query execution, type safety for reliable API integration, and single-binary deployment for maximum reliability. Runs twice daily via GitHub Actions to ensure no opportunities are missed within their critical 30-day window.

## System Architecture

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│  GitHub Actions │────▶│  Go Binary       │────▶│  Notifications  │
│  (2x Daily)     │     │  (Concurrent)    │     │  (Email/Slack)  │
└─────────────────┘     └────────┬─────────┘     └─────────────────┘
                                  │
                          ┌───────┴────────┐
                          ▼                ▼
                    ┌──────────┐    ┌──────────────┐
                    │ SAM.gov  │    │ State Store  │
                    │   API     │    │   (JSON)     │
                    └──────────┘    └──────────────┘
```

## Configuration Example

```yaml
# config/queries.yaml
queries:
  - name: "AI and Machine Learning Opportunities"
    enabled: true
    parameters:
      title: "artificial intelligence"
      organizationName: "DEFENSE ADVANCED RESEARCH PROJECTS AGENCY"
      ptype: ["s", "p", "o"]  # Special Notice, Pre-solicitation, Solicitation
    notification:
      priority: high
      recipients: ["ai-team@company.com"]
      
  - name: "DARPA AIE Specific"
    enabled: true
    parameters:
      title: "AIE"
      organizationName: "DARPA"
      ptype: "s"
    notification:
      priority: high
      
  - name: "Small Business IT Contracts"
    enabled: true
    parameters:
      typeOfSetAside: ["SBA", "WOSB", "SDVOSBC"]
      naicsCode: "541512"  # Computer Systems Design
    notification:
      priority: medium
```

## Why Go?

- **Single Binary**: No runtime dependencies, instant GitHub Action startup
- **Concurrent Queries**: Native goroutines for parallel API calls
- **Type Safety**: Strongly typed API responses prevent runtime errors
- **Performance**: 10x faster than interpreted languages
- **Error Handling**: Explicit error handling for reliability

## Use Cases

1. **Research Organizations**: Monitor DARPA AIE and other advanced research opportunities
2. **Small Businesses**: Track set-aside opportunities in specific NAICS codes
3. **Defense Contractors**: Watch for opportunities from specific agencies
4. **Universities**: Find research grants and cooperative agreements
5. **General Contractors**: Monitor by location, value, or type

## Core Features (Gherkin)

### Feature: SAM.gov Opportunity Detection

```gherkin
Feature: Detect new SAM.gov opportunities
  As a government contractor or researcher
  I want to be notified of matching opportunities
  So that I can respond within the deadline window

  Background:
    Given the SAM.gov API is accessible
    And I have a valid API key configured
    And the monitoring system is active

  Scenario: New opportunity matching criteria
    Given a new opportunity was posted today
    And it matches my configured query parameters
    When the scheduled check runs
    Then the system should detect the opportunity
    And send an immediate notification
    And store the opportunity ID to prevent duplicate alerts

  Scenario: Weekend posting detection
    Given an opportunity was posted on Saturday
    When the Monday check runs
    Then the system should detect the weekend opportunity
    And include the posting date in the notification
    
  Scenario: Concurrent query execution
    Given I have 10 different queries configured
    When the monitor runs
    Then all queries should execute in parallel
    And results should be aggregated efficiently
    And total execution time should be under 10 seconds
    
  Scenario: Multiple notification channels
    Given a high-priority opportunity is found
    When notifications are triggered
    Then an email should be sent immediately
    And a Slack webhook should be called
    And a GitHub issue should be created
```

### Feature: Configurable Query Matching

```gherkin
Feature: Match opportunities based on configurable criteria
  As a monitoring system
  I need to accurately filter opportunities
  So that users only receive relevant notifications

  Scenario Outline: Query parameter matching
    Given a query configured with "<parameter>" = "<value>"
    When an opportunity is evaluated
    And the opportunity has "<parameter>" = "<actual_value>"
    Then the match result should be "<result>"

    Examples:
      | parameter        | value    | actual_value | result  |
      | organizationName | DARPA    | DARPA        | match   |
      | ptype           | s        | s            | match   |
      | title           | AI       | Artificial   | match   |
      | naicsCode       | 541715   | 541715       | match   |
      | ptype           | a        | s            | no match|
```

## Technical Requirements

### Go Project Structure
```
sam-gov-monitor/
├── .github/
│   ├── workflows/
│   │   └── monitor.yml
│   └── ISSUE_TEMPLATE/
│       └── opportunity.md
├── cmd/
│   └── monitor/
│       └── main.go
├── internal/
│   ├── config/
│   │   ├── config.go
│   │   └── validator.go
│   ├── samgov/
│   │   ├── client.go
│   │   ├── types.go
│   │   └── retry.go
│   ├── monitor/
│   │   ├── monitor.go
│   │   ├── state.go
│   │   └── differ.go
│   ├── notify/
│   │   ├── email.go
│   │   ├── slack.go
│   │   └── interface.go
│   └── cache/
│       └── cache.go
├── config/
│   └── queries.yaml
├── test/
│   ├── features/
│   │   ├── monitor.feature
│   │   └── notifications.feature
│   └── integration/
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

### Environment Variables
```yaml
SAM_API_KEY: Required - SAM.gov API authentication
SMTP_HOST: Required - Email server host
SMTP_PORT: Required - Email server port
SMTP_USERNAME: Required - Email authentication
SMTP_PASSWORD: Required - Email password
EMAIL_FROM: Required - Sender email address
EMAIL_TO: Required - Recipient email addresses
SLACK_WEBHOOK: Optional - Slack notification webhook
GITHUB_TOKEN: Automatic - For creating issues
```

### Example Search Scenarios

### Scenario 1: DARPA AIE Opportunities
```yaml
- name: "DARPA AIE Opportunities"
  parameters:
    organizationName: "DEFENSE ADVANCED RESEARCH PROJECTS AGENCY"
    title: "AIE"
    ptype: "s"  # Special notices where AIE opportunities appear
```

### Scenario 2: Small Business IT Opportunities
```yaml
- name: "Small Business IT Set-Asides"
  parameters:
    typeOfSetAside: ["SBA", "8A", "WOSB"]
    naicsCode: "541512"
    state: "CA"  # California opportunities
```

### Scenario 3: High-Value Defense Contracts
```yaml
- name: "Large Defense Contracts"
  parameters:
    organizationName: "DEPARTMENT OF DEFENSE"
    ptype: ["o", "k"]  # Solicitations and Combined Synopsis
    minValue: 1000000  # Over $1M
```

## API Integration Specifications

#### SAM.gov API Client (Go)
```go
type Client struct {
    apiKey     string
    baseURL    string
    httpClient *http.Client
}

// API Request
func (c *Client) Search(ctx context.Context, params map[string]string) (*SearchResponse, error) {
    // Build URL with parameters
    u, _ := url.Parse(c.baseURL)
    q := u.Query()
    q.Set("api_key", c.apiKey)
    
    // Add search parameters
    for k, v := range params {
        q.Set(k, v)
    }
    u.RawQuery = q.Encode()
    
    // Execute request with context
    req, _ := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
    resp, err := c.httpClient.Do(req)
    // ... error handling and response parsing
}
```

#### Go Type Definitions
```go
type Opportunity struct {
    NoticeID         string     `json:"noticeId"`
    Title           string     `json:"title"`
    SolicitationNum string     `json:"solicitationNumber"`
    FullParentPath  string     `json:"fullParentPathName"`
    PostedDate      string     `json:"postedDate"`
    Type            string     `json:"type"`
    ResponseDeadline *string    `json:"responseDeadLine"`
    UILink          string     `json:"uiLink"`
    Active          string     `json:"active"`
    PointOfContact  []Contact  `json:"pointOfContact"`
}

type SearchResponse struct {
    TotalRecords      int            `json:"totalRecords"`
    OpportunitiesData []Opportunity `json:"opportunitiesData"`
}
```

## Notification Mechanisms

### Email Notification (Go Implementation)
```go
type EmailNotifier struct {
    smtp SMTPConfig
    from string
}

func (e *EmailNotifier) Send(ctx context.Context, notification Notification) error {
    // Build email using templates
    tmpl := template.Must(template.New("opportunity").Parse(emailTemplate))
    
    var body bytes.Buffer
    if err := tmpl.Execute(&body, notification); err != nil {
        return fmt.Errorf("template execution: %w", err)
    }
    
    // Send via SMTP
    auth := smtp.PlainAuth("", e.smtp.Username, e.smtp.Password, e.smtp.Host)
    return smtp.SendMail(
        fmt.Sprintf("%s:%d", e.smtp.Host, e.smtp.Port),
        auth,
        e.from,
        notification.Recipients,
        body.Bytes(),
    )
}
```

### GitHub Issue Creation
```go
func CreateGitHubIssue(ctx context.Context, opp Opportunity) error {
    client := github.NewClient(nil).WithAuthToken(os.Getenv("GITHUB_TOKEN"))
    
    body := formatIssueBody(opp) // Uses template
    issue := &github.IssueRequest{
        Title:  github.String(fmt.Sprintf("🚨 Opportunity: %s", opp.Title)),
        Body:   github.String(body),
        Labels: &[]string{"opportunity", "urgent"},
    }
    
    _, _, err := client.Issues.Create(ctx, owner, repo, issue)
    return err
}
```

### GitHub Issue Template
```markdown
## 🚨 New Opportunity: {{.Title}}

**Notice ID:** {{.NoticeID}}
**Organization:** {{.FullParentPath}}
**Posted:** {{.PostedDate}}
**Deadline:** {{.ResponseDeadline}}

### Quick Actions
- [View on SAM.gov]({{.UILink}})
- [Download Description]({{.Description}}?api_key=YOUR_KEY)

### Details
- **Type:** {{.Type}}
- **Set-Aside:** {{.SetAside}}
- **NAICS:** {{.NAICSCode}}
- **Place of Performance:** {{.PlaceOfPerformance}}

### Next Steps
- [ ] Review requirements
- [ ] Go/No-Go decision
- [ ] Assign proposal team
- [ ] Create timeline
```

### Email Template Example
```html
<!DOCTYPE html>
<html>
<head>
    <style>
        .opportunity { 
            border: 1px solid #ddd; 
            padding: 15px; 
            margin: 10px 0;
            border-radius: 5px;
        }
        .high-priority { border-left: 5px solid #ff4444; }
        .deadline { color: #ff4444; font-weight: bold; }
    </style>
</head>
<body>
    <h2>{{.QueryName}} - New Opportunities</h2>
    <p>Found {{len .Opportunities}} new opportunities</p>
    
    {{range .Opportunities}}
    <div class="opportunity {{.PriorityClass}}">
        <h3>{{.Title}}</h3>
        <p><strong>Notice ID:</strong> {{.NoticeID}}</p>
        <p><strong>Posted:</strong> {{.PostedDate}}</p>
        {{if .ResponseDeadline}}
        <p class="deadline"><strong>Deadline:</strong> {{.ResponseDeadline}}</p>
        {{end}}
        <a href="{{.UILink}}">View on SAM.gov</a>
    </div>
    {{end}}
</body>
</html>
```

### Slack Notification Format
```json
{
  "text": "🚨 New SAM.gov Opportunities Found!",
  "blocks": [
    {
      "type": "header",
      "text": {
        "type": "plain_text",
        "text": "{{.QueryName}}"
      }
    },
    {
      "type": "section",
      "fields": [
        {"type": "mrkdwn", "text": "*Title:*\n{{.Title}}"},
        {"type": "mrkdwn", "text": "*Notice ID:*\n{{.NoticeID}}"},
        {"type": "mrkdwn", "text": "*Posted:*\n{{.PostedDate}}"},
        {"type": "mrkdwn", "text": "*Deadline:*\n{{.ResponseDeadline}}"}
      ]
    },
    {
      "type": "actions",
      "elements": [
        {
          "type": "button",
          "text": {"type": "plain_text", "text": "View on SAM.gov"},
          "url": "{{.UILink}}"
        }
      ]
    }
  ]
}
```

### Configuration Structure
```go
type Query struct {
    Name         string                 `yaml:"name"`
    Enabled      bool                  `yaml:"enabled"`
    Parameters   map[string]interface{} `yaml:"parameters"`
    Notification NotificationConfig     `yaml:"notification"`
}
```

## State Management

### Go State Implementation
```go
type State struct {
    mu            sync.RWMutex
    Opportunities map[string]OpportunityState `json:"opportunities"`
    LastRun       time.Time                   `json:"last_run"`
    QueryMetrics  map[string]QueryMetrics     `json:"query_metrics"`
}

type OpportunityState struct {
    FirstSeen    time.Time `json:"first_seen"`
    LastSeen     time.Time `json:"last_seen"`
    NoticeID     string    `json:"notice_id"`
    Title        string    `json:"title"`
    Deadline     *string   `json:"deadline,omitempty"`
}

// Thread-safe state operations
func (s *State) AddOpportunity(opp Opportunity) bool {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    if _, exists := s.Opportunities[opp.NoticeID]; exists {
        return false // Not new
    }
    
    s.Opportunities[opp.NoticeID] = OpportunityState{
        FirstSeen: time.Now(),
        LastSeen:  time.Now(),
        NoticeID:  opp.NoticeID,
        Title:     opp.Title,
        Deadline:  opp.ResponseDeadline,
    }
    return true // New opportunity
}
```

## Error Handling

### Feature: Resilient Error Handling with Go

```gherkin
Feature: Handle errors gracefully
  As a monitoring system
  I need to handle errors without losing opportunities
  So that temporary issues don't cause missed notifications

  Scenario: API rate limiting with retry
    Given the SAM.gov API returns a 429 error
    When executing a search
    Then the system should use exponential backoff
    And retry up to 3 times
    And log each retry attempt

  Scenario: Concurrent query failure isolation
    Given 5 queries are running in parallel
    When 1 query fails with a network error
    Then the other 4 queries should complete successfully
    And only the failed query should be retried
    And errors should be aggregated in the final report
```

### Go Error Handling Pattern
```go
// Retry with exponential backoff
func (c *Client) SearchWithRetry(ctx context.Context, params map[string]string) (*SearchResponse, error) {
    backoff := backoff.NewExponentialBackOff()
    backoff.MaxElapsedTime = 30 * time.Second
    
    var result *SearchResponse
    err := backoff.Retry(func() error {
        resp, err := c.Search(ctx, params)
        if err != nil {
            if isRetryable(err) {
                return err // Will retry
            }
            return backoff.Permanent(err) // Won't retry
        }
        result = resp
        return nil
    })
    
    return result, err
}
```

## Performance Considerations

- **Concurrent Execution**: Go routines for parallel query processing
- **Connection Pooling**: Reuse HTTP connections across queries
- **Binary Size**: ~10MB standalone executable
- **Memory Usage**: ~50MB for typical workload
- **Startup Time**: <100ms (no runtime dependencies)
- **Query Timeout**: 30-second timeout per API call with context cancellation

## Security Considerations

- API keys stored as GitHub Secrets
- No credentials in binary or logs
- TLS verification for all API calls
- Input validation on all query parameters
- Secure SMTP connections for email
- Sanitized error messages

## Success Metrics

1. **Zero Missed Opportunities**: 100% detection rate for matching criteria
2. **Performance**: <10s total execution time for 10 concurrent queries
3. **Reliability**: 99.9% success rate (GitHub Actions SLA)
4. **Notification Speed**: Alerts sent within 30 seconds of detection
5. **Resource Efficiency**: <100MB memory usage

## Future Enhancement Considerations

- GraphQL support when SAM.gov implements it
- Prometheus metrics export for monitoring
- Kubernetes CronJob deployment option
- REST API for query management
- Machine learning for relevance scoring
- Natural language query configuration

## Quick Start

1. **Get SAM.gov API Key**: Register at sam.gov → Account Details → Request Public API Key
2. **Fork Repository**: Fork the sam-gov-monitor repository
3. **Configure Queries**: Edit `config/queries.yaml` with your search criteria
4. **Set GitHub Secrets**: Add API keys and email credentials
5. **Enable Actions**: Go to Actions tab and enable workflows
6. **Test Run**: Manually trigger with dry-run option
7. **Monitor**: Check email for notifications!

The system is designed to be operational within 30 minutes of setup.
