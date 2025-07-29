# SAM.gov Opportunity Monitor (Go) - Development Roadmap

## 🎯 Project Goal
Build a GitHub Action in Go that monitors SAM.gov opportunities based on configurable search queries, running twice daily and sending email alerts for matches.

## 📋 Prerequisites Checklist
- [ ] SAM.gov account created
- [ ] SAM.gov API key obtained
- [ ] GitHub repository created
- [ ] Go 1.21+ installed locally
- [ ] Email service chosen (SMTP/SendGrid/AWS SES)
- [ ] Test data identified

---

## 📅 Day 1: Foundation & Go Setup

### Morning Session (2-3 hours)
**Goal: Repository structure and Go project initialization**

#### Tasks:
1. **Create Go Project Structure**
   ```
   sam-gov-monitor/
   ├── .github/
   │   ├── workflows/
   │   │   └── monitor.yml
   │   └── ISSUE_TEMPLATE/
   ├── cmd/
   │   └── monitor/
   │       └── main.go
   ├── internal/
   │   ├── config/
   │   │   └── config.go
   │   ├── samgov/
   │   │   ├── client.go
   │   │   └── types.go
   │   ├── monitor/
   │   │   └── monitor.go
   │   └── notify/
   │       └── email.go
   ├── config/
   │   └── queries.yaml
   ├── test/
   │   └── features/
   │       └── monitor.feature
   ├── go.mod
   ├── go.sum
   ├── Makefile
   └── README.md
   ```

2. **Initialize Go Module**
   ```bash
   go mod init github.com/yourusername/sam-gov-monitor
   ```

3. **Define Core Types**
   ```go
   // internal/samgov/types.go
   package samgov

   type Opportunity struct {
       NoticeID         string     `json:"noticeId"`
       Title           string     `json:"title"`
       SolicitationNum string     `json:"solicitationNumber"`
       Department      string     `json:"department"`
       PostedDate      string     `json:"postedDate"`
       ResponseDeadline *string    `json:"responseDeadLine"`
       Type            string     `json:"type"`
       UILink          string     `json:"uiLink"`
       Description     string     `json:"description"`
   }

   type SearchResponse struct {
       TotalRecords      int            `json:"totalRecords"`
       Limit            int            `json:"limit"`
       Offset           int            `json:"offset"`
       OpportunitiesData []Opportunity `json:"opportunitiesData"`
   }
   ```

4. **Create Configuration Schema**
   ```go
   // internal/config/config.go
   package config

   type Config struct {
       Queries []Query `yaml:"queries"`
   }

   type Query struct {
       Name         string                 `yaml:"name"`
       Enabled      bool                  `yaml:"enabled"`
       Parameters   map[string]interface{} `yaml:"parameters"`
       Notification NotificationConfig     `yaml:"notification"`
   }

   type NotificationConfig struct {
       Priority string   `yaml:"priority"`
       Recipients []string `yaml:"recipients,omitempty"`
   }
   ```

### Afternoon Session (2-3 hours)
**Goal: Basic API client and testing setup**

#### Tasks:
1. **Implement SAM.gov Client**
   ```go
   // internal/samgov/client.go
   package samgov

   import (
       "context"
       "encoding/json"
       "fmt"
       "net/http"
       "net/url"
       "time"
   )

   type Client struct {
       apiKey     string
       baseURL    string
       httpClient *http.Client
   }

   func NewClient(apiKey string) *Client {
       return &Client{
           apiKey:  apiKey,
           baseURL: "https://api.sam.gov/opportunities/v2/search",
           httpClient: &http.Client{
               Timeout: 30 * time.Second,
           },
       }
   }
   ```

2. **Create Gherkin Feature File**
   ```gherkin
   # test/features/monitor.feature
   Feature: SAM.gov Opportunity Monitoring
     As a government contractor
     I want to monitor SAM.gov for relevant opportunities
     So that I can respond to them quickly

     Background:
       Given I have a valid SAM.gov API key
       And I have configured search queries

     Scenario: Successfully retrieve opportunities
       Given the following search parameters:
         | field            | value                    |
         | title           | artificial intelligence   |
         | organizationName| DARPA                    |
         | ptype           | s                        |
       When I execute the search
       Then I should receive a list of opportunities
       And each opportunity should have a notice ID
       And each opportunity should have a title

     Scenario: Handle no results gracefully
       Given a search with very specific criteria
       When I execute the search
       And no opportunities match
       Then I should receive an empty result set
       And no error should occur
   ```

3. **Set Up Testing Framework**
   ```bash
   go get github.com/cucumber/godog
   go get github.com/stretchr/testify
   ```

4. **Create Basic Integration Test**
   ```go
   // test/integration_test.go
   func TestAPIConnection(t *testing.T) {
       apiKey := os.Getenv("SAM_API_KEY")
       if apiKey == "" {
           t.Skip("SAM_API_KEY not set")
       }

       client := samgov.NewClient(apiKey)
       ctx := context.Background()
       
       params := map[string]string{
           "postedFrom": time.Now().AddDate(0, 0, -7).Format("01/02/2006"),
           "postedTo":   time.Now().Format("01/02/2006"),
           "limit":      "1",
       }

       resp, err := client.Search(ctx, params)
       assert.NoError(t, err)
       assert.NotNil(t, resp)
   }
   ```

**Deliverables:**
- ✅ Go project structure created
- ✅ Core types defined
- ✅ Basic API client structure
- ✅ Gherkin features defined
- ✅ Testing framework set up

---

## 📅 Day 2: Core Search Implementation

### Morning Session (3 hours)
**Goal: Complete API client and search logic**

#### Tasks:
1. **Implement Search Method**
   ```go
   // internal/samgov/client.go
   func (c *Client) Search(ctx context.Context, params map[string]string) (*SearchResponse, error) {
       u, _ := url.Parse(c.baseURL)
       q := u.Query()
       
       // Add API key
       q.Set("api_key", c.apiKey)
       
       // Add all parameters
       for k, v := range params {
           q.Set(k, v)
       }
       u.RawQuery = q.Encode()

       req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
       if err != nil {
           return nil, fmt.Errorf("creating request: %w", err)
       }

       resp, err := c.httpClient.Do(req)
       if err != nil {
           return nil, fmt.Errorf("executing request: %w", err)
       }
       defer resp.Body.Close()

       if resp.StatusCode != http.StatusOK {
           return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
       }

       var result SearchResponse
       if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
           return nil, fmt.Errorf("decoding response: %w", err)
       }

       return &result, nil
   }
   ```

2. **Create Query Builder**
   ```go
   // internal/monitor/query_builder.go
   package monitor

   import (
       "fmt"
       "time"
   )

   type QueryBuilder struct {
       lookbackDays int
   }

   func NewQueryBuilder(lookbackDays int) *QueryBuilder {
       return &QueryBuilder{lookbackDays: lookbackDays}
   }

   func (qb *QueryBuilder) BuildParams(query config.Query) (map[string]string, error) {
       params := make(map[string]string)
       
       // Always add date range
       to := time.Now()
       from := to.AddDate(0, 0, -qb.lookbackDays)
       
       params["postedFrom"] = from.Format("01/02/2006")
       params["postedTo"] = to.Format("01/02/2006")
       params["limit"] = "100"
       
       // Add query-specific parameters
       for k, v := range query.Parameters {
           switch val := v.(type) {
           case string:
               params[k] = val
           case []interface{}:
               // Handle array parameters (e.g., multiple ptypes)
               if k == "ptype" && len(val) > 0 {
                   params[k] = fmt.Sprintf("%v", val[0])
               }
           }
       }
       
       return params, nil
   }
   ```

3. **Implement Concurrent Query Execution**
   ```go
   // internal/monitor/monitor.go
   package monitor

   type Monitor struct {
       client  *samgov.Client
       config  *config.Config
       builder *QueryBuilder
   }

   func (m *Monitor) RunQueries(ctx context.Context) ([]QueryResult, error) {
       results := make([]QueryResult, 0)
       resultsChan := make(chan QueryResult, len(m.config.Queries))
       errChan := make(chan error, len(m.config.Queries))
       
       // Run queries concurrently
       for _, query := range m.config.Queries {
           if !query.Enabled {
               continue
           }
           
           go func(q config.Query) {
               result, err := m.executeQuery(ctx, q)
               if err != nil {
                   errChan <- err
                   return
               }
               resultsChan <- result
           }(query)
       }
       
       // Collect results
       // ... (timeout and error handling)
       
       return results, nil
   }
   ```

### Afternoon Session (3 hours)
**Goal: State management and deduplication**

#### Tasks:
1. **Define State Storage**
   ```go
   // internal/monitor/state.go
   package monitor

   import (
       "encoding/json"
       "os"
       "sync"
       "time"
   )

   type State struct {
       mu            sync.RWMutex
       Opportunities map[string]OpportunityState `json:"opportunities"`
       LastRun       time.Time                   `json:"last_run"`
       filepath      string
   }

   type OpportunityState struct {
       FirstSeen    time.Time `json:"first_seen"`
       LastSeen     time.Time `json:"last_seen"`
       LastModified time.Time `json:"last_modified"`
       NoticeID     string    `json:"notice_id"`
       Title        string    `json:"title"`
   }

   func LoadState(filepath string) (*State, error) {
       state := &State{
           Opportunities: make(map[string]OpportunityState),
           filepath:      filepath,
       }
       
       data, err := os.ReadFile(filepath)
       if err != nil {
           if os.IsNotExist(err) {
               return state, nil
           }
           return nil, err
       }
       
       return state, json.Unmarshal(data, state)
   }
   ```

2. **Implement Deduplication Logic**
   ```gherkin
   # test/features/deduplication.feature
   Feature: Opportunity Deduplication
     As a monitor system
     I need to track which opportunities have been seen
     So that I only notify about new opportunities

     Scenario: First time seeing an opportunity
       Given an empty state file
       When I process opportunity "DARPA-001"
       Then it should be marked as new
       And it should be added to the state

     Scenario: Previously seen opportunity
       Given opportunity "DARPA-001" was seen yesterday
       When I process opportunity "DARPA-001" again
       Then it should not be marked as new
       And the last_seen date should be updated

     Scenario: Modified opportunity
       Given opportunity "DARPA-001" with modification date "2024-01-01"
       When I process the same opportunity with modification date "2024-01-02"
       Then it should be marked as updated
       And a notification should be sent
   ```

3. **Create Opportunity Differ**
   ```go
   // internal/monitor/differ.go
   func (m *Monitor) DiffOpportunities(current []samgov.Opportunity) DiffResult {
       diff := DiffResult{
           New:      []samgov.Opportunity{},
           Updated:  []samgov.Opportunity{},
           Existing: []samgov.Opportunity{},
       }
       
       m.state.mu.RLock()
       defer m.state.mu.RUnlock()
       
       for _, opp := range current {
           if prev, exists := m.state.Opportunities[opp.NoticeID]; exists {
               if m.hasChanged(prev, opp) {
                   diff.Updated = append(diff.Updated, opp)
               } else {
                   diff.Existing = append(diff.Existing, opp)
               }
           } else {
               diff.New = append(diff.New, opp)
           }
       }
       
       return diff
   }
   ```

**Deliverables:**
- ✅ Complete API client with search
- ✅ Concurrent query execution
- ✅ State management system
- ✅ Deduplication with diff logic
- ✅ BDD tests for deduplication

---

## 📅 Day 3: Notification System

### Morning Session (3 hours)
**Goal: Email notification implementation**

#### Tasks:
1. **Create Email Service Interface**
   ```go
   // internal/notify/interface.go
   package notify

   type Notifier interface {
       Send(ctx context.Context, notification Notification) error
   }

   type Notification struct {
       Recipients []string
       Subject    string
       Body       Body
       Priority   Priority
       Attachments []Attachment
   }

   type Body struct {
       HTML string
       Text string
   }
   ```

2. **Implement SMTP Email Sender**
   ```go
   // internal/notify/smtp.go
   package notify

   import (
       "bytes"
       "fmt"
       "html/template"
       "net/smtp"
   )

   type SMTPNotifier struct {
       host     string
       port     int
       username string
       password string
       from     string
   }

   func (s *SMTPNotifier) Send(ctx context.Context, n Notification) error {
       // Build email with template
       tmpl := s.getTemplate(n.Priority)
       
       var body bytes.Buffer
       if err := tmpl.Execute(&body, n); err != nil {
           return fmt.Errorf("executing template: %w", err)
       }
       
       // Send via SMTP
       auth := smtp.PlainAuth("", s.username, s.password, s.host)
       addr := fmt.Sprintf("%s:%d", s.host, s.port)
       
       return smtp.SendMail(addr, auth, s.from, n.Recipients, body.Bytes())
   }
   ```

3. **Create HTML Templates**
   ```go
   // internal/notify/templates.go
   const opportunityTemplate = `
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
           .medium-priority { border-left: 5px solid #ffaa00; }
           .deadline { color: #ff4444; font-weight: bold; }
       </style>
   </head>
   <body>
       <h2>{{.QueryName}} - New Opportunities Found</h2>
       <p>Found {{len .Opportunities}} new opportunities</p>
       
       {{range .Opportunities}}
       <div class="opportunity {{.PriorityClass}}">
           <h3>{{.Title}}</h3>
           <p><strong>Notice ID:</strong> {{.NoticeID}}</p>
           <p><strong>Posted:</strong> {{.PostedDate}}</p>
           {{if .ResponseDeadline}}
           <p class="deadline"><strong>Deadline:</strong> {{.ResponseDeadline}}</p>
           {{end}}
           <p><strong>Type:</strong> {{.Type}}</p>
           <a href="{{.UILink}}">View on SAM.gov</a>
       </div>
       {{end}}
   </body>
   </html>
   `
   ```

### Afternoon Session (2 hours)
**Goal: Advanced notification features**

#### Tasks:
1. **Implement Digest Mode**
   ```gherkin
   # test/features/notifications.feature
   Feature: Email Notifications
     As a user
     I want to receive email notifications about opportunities
     So that I can respond quickly

     Scenario: High priority immediate notification
       Given a high priority query finds new opportunities
       When the notification is triggered
       Then an email should be sent immediately
       And the subject should indicate high priority

     Scenario: Daily digest for medium priority
       Given multiple medium priority queries with results
       When the daily digest time is reached
       Then a single digest email should be sent
       And it should group opportunities by query

     Scenario: Include calendar attachment for deadlines
       Given opportunities with response deadlines
       When sending notifications
       Then an .ics file should be attached
       And it should contain deadline reminders
   ```

2. **Create Calendar Generator**
   ```go
   // internal/notify/calendar.go
   package notify

   import (
       "fmt"
       "strings"
       "time"
   )

   func GenerateICS(opportunities []samgov.Opportunity) string {
       var ics strings.Builder
       
       ics.WriteString("BEGIN:VCALENDAR\r\n")
       ics.WriteString("VERSION:2.0\r\n")
       ics.WriteString("PRODID:-//SAM.gov Monitor//EN\r\n")
       
       for _, opp := range opportunities {
           if opp.ResponseDeadline == nil {
               continue
           }
           
           deadline, _ := time.Parse("2006-01-02", *opp.ResponseDeadline)
           reminder := deadline.AddDate(0, 0, -5) // 5 days before
           
           ics.WriteString("BEGIN:VEVENT\r\n")
           ics.WriteString(fmt.Sprintf("UID:%s@samgov-monitor\r\n", opp.NoticeID))
           ics.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", time.Now().Format("20060102T150405Z")))
           ics.WriteString(fmt.Sprintf("DTSTART:%s\r\n", reminder.Format("20060102")))
           ics.WriteString(fmt.Sprintf("SUMMARY:SAM.gov Deadline: %s\r\n", opp.Title))
           ics.WriteString(fmt.Sprintf("DESCRIPTION:Response deadline for %s\\n%s\r\n", 
               opp.NoticeID, opp.UILink))
           ics.WriteString("END:VEVENT\r\n")
       }
       
       ics.WriteString("END:VCALENDAR\r\n")
       return ics.String()
   }
   ```

3. **Add Webhook Support**
   ```go
   // internal/notify/webhook.go
   type WebhookNotifier struct {
       webhookURL string
       client     *http.Client
   }

   func (w *WebhookNotifier) Send(ctx context.Context, n Notification) error {
       payload := map[string]interface{}{
           "text": n.Subject,
           "blocks": w.buildSlackBlocks(n),
       }
       
       data, _ := json.Marshal(payload)
       req, _ := http.NewRequestWithContext(ctx, "POST", w.webhookURL, bytes.NewReader(data))
       req.Header.Set("Content-Type", "application/json")
       
       resp, err := w.client.Do(req)
       if err != nil {
           return err
       }
       defer resp.Body.Close()
       
       return nil
   }
   ```

**Deliverables:**
- ✅ Email service with templates
- ✅ Priority-based notification logic
- ✅ Calendar file generation
- ✅ Webhook support (Slack/Teams)
- ✅ BDD tests for notifications

---

## 📅 Day 4: GitHub Action Integration

### Morning Session (3 hours)
**Goal: Create GitHub Action workflow and CLI**

#### Tasks:
1. **Build CLI Application**
   ```go
   // cmd/monitor/main.go
   package main

   import (
       "context"
       "flag"
       "log"
       "os"
       
       "github.com/yourusername/sam-gov-monitor/internal/config"
       "github.com/yourusername/sam-gov-monitor/internal/monitor"
   )

   func main() {
       var (
           configPath = flag.String("config", "config/queries.yaml", "Path to config file")
           stateFile  = flag.String("state", "state/monitor.json", "Path to state file")
           dryRun     = flag.Bool("dry-run", false, "Run without sending notifications")
           verbose    = flag.Bool("v", false, "Verbose output")
       )
       flag.Parse()

       // Load configuration
       cfg, err := config.Load(*configPath)
       if err != nil {
           log.Fatalf("loading config: %v", err)
       }

       // Validate environment
       apiKey := os.Getenv("SAM_API_KEY")
       if apiKey == "" {
           log.Fatal("SAM_API_KEY environment variable is required")
       }

       // Create and run monitor
       m, err := monitor.New(monitor.Options{
           APIKey:    apiKey,
           Config:    cfg,
           StateFile: *stateFile,
           DryRun:    *dryRun,
           Verbose:   *verbose,
       })
       if err != nil {
           log.Fatalf("creating monitor: %v", err)
       }

       ctx := context.Background()
       if err := m.Run(ctx); err != nil {
           log.Fatalf("running monitor: %v", err)
       }
   }
   ```

2. **Create GitHub Action Workflow**
   ```yaml
   # .github/workflows/monitor.yml
   name: SAM.gov Opportunity Monitor

   on:
     schedule:
       - cron: '0 12 * * *'  # Noon UTC (8 AM ET)
       - cron: '0 22 * * *'  # 10 PM UTC (6 PM ET)
     workflow_dispatch:
       inputs:
         dry_run:
           description: 'Run without sending notifications'
           required: false
           type: boolean
           default: false
         verbose:
           description: 'Enable verbose logging'
           required: false
           type: boolean
           default: false

   jobs:
     monitor:
       runs-on: ubuntu-latest
       
       steps:
       - name: Checkout
         uses: actions/checkout@v4
         
       - name: Set up Go
         uses: actions/setup-go@v4
         with:
           go-version: '1.21'
           
       - name: Cache Go modules
         uses: actions/cache@v3
         with:
           path: ~/go/pkg/mod
           key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
           restore-keys: |
             ${{ runner.os }}-go-
             
       - name: Build
         run: go build -o monitor ./cmd/monitor
         
       - name: Run tests
         run: go test ./...
         env:
           SAM_API_KEY: ${{ secrets.SAM_API_KEY }}
           
       - name: Run monitor
         run: |
           ./monitor \
             -config config/queries.yaml \
             -state state/monitor.json \
             ${{ github.event.inputs.dry_run == 'true' && '-dry-run' || '' }} \
             ${{ github.event.inputs.verbose == 'true' && '-v' || '' }}
         env:
           SAM_API_KEY: ${{ secrets.SAM_API_KEY }}
           SMTP_HOST: ${{ secrets.SMTP_HOST }}
           SMTP_PORT: ${{ secrets.SMTP_PORT }}
           SMTP_USERNAME: ${{ secrets.SMTP_USERNAME }}
           SMTP_PASSWORD: ${{ secrets.SMTP_PASSWORD }}
           EMAIL_FROM: ${{ secrets.EMAIL_FROM }}
           EMAIL_TO: ${{ secrets.EMAIL_TO }}
           
       - name: Upload state
         if: always()
         uses: actions/upload-artifact@v3
         with:
           name: monitor-state
           path: state/
           
       - name: Upload logs
         if: failure()
         uses: actions/upload-artifact@v3
         with:
           name: monitor-logs
           path: logs/
   ```

3. **Implement Status Reporting**
   ```go
   // internal/monitor/report.go
   package monitor

   type RunReport struct {
       StartTime    time.Time
       EndTime      time.Time
       QueriesRun   int
       NewOpps      int
       UpdatedOpps  int
       Notifications int
       Errors       []error
   }

   func (r *RunReport) GenerateMarkdown() string {
       return fmt.Sprintf(`# SAM.gov Monitor Run Report

   ## Summary
   - Started: %s
   - Completed: %s
   - Duration: %s
   
   ## Results
   - Queries Run: %d
   - New Opportunities: %d
   - Updated Opportunities: %d
   - Notifications Sent: %d
   
   ## Status: %s
   `,
       r.StartTime.Format(time.RFC3339),
       r.EndTime.Format(time.RFC3339),
       r.EndTime.Sub(r.StartTime).String(),
       r.QueriesRun,
       r.NewOpps,
       r.UpdatedOpps,
       r.Notifications,
       r.getStatus(),
   )
   }
   ```

### Afternoon Session (2 hours)
**Goal: Testing and error handling**

#### Tasks:
1. **Create Integration Tests**
   ```go
   // test/integration/monitor_test.go
   func TestMonitorIntegration(t *testing.T) {
       if testing.Short() {
           t.Skip("skipping integration test")
       }

       // Create test config
       cfg := &config.Config{
           Queries: []config.Query{
               {
                   Name:    "Test Query",
                   Enabled: true,
                   Parameters: map[string]interface{}{
                       "title": "test",
                   },
               },
           },
       }

       // Run monitor in dry-run mode
       m, _ := monitor.New(monitor.Options{
           APIKey: os.Getenv("SAM_API_KEY"),
           Config: cfg,
           DryRun: true,
       })

       ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
       defer cancel()

       err := m.Run(ctx)
       assert.NoError(t, err)
   }
   ```

2. **Add Makefile for Common Tasks**
   ```makefile
   # Makefile
   .PHONY: build test run lint docker

   build:
       go build -o bin/monitor ./cmd/monitor

   test:
       go test -v ./...

   test-integration:
       go test -v -tags=integration ./test/...

   run:
       go run ./cmd/monitor -config config/queries.yaml

   lint:
       golangci-lint run

   docker:
       docker build -t sam-gov-monitor .

   coverage:
       go test -coverprofile=coverage.out ./...
       go tool cover -html=coverage.out
   ```

3. **Create Docker Support**
   ```dockerfile
   # Dockerfile
   FROM golang:1.21-alpine AS builder
   WORKDIR /app
   COPY go.mod go.sum ./
   RUN go mod download
   COPY . .
   RUN go build -o monitor ./cmd/monitor

   FROM alpine:latest
   RUN apk --no-cache add ca-certificates
   WORKDIR /root/
   COPY --from=builder /app/monitor .
   COPY config/queries.yaml ./config/
   CMD ["./monitor"]
   ```

**Deliverables:**
- ✅ CLI application with flags
- ✅ Complete GitHub Action workflow
- ✅ Integration test suite
- ✅ Build automation with Makefile
- ✅ Docker support

---

## 📅 Day 5: Advanced Features & Error Handling

### Morning Session (3 hours)
**Goal: Enhanced query capabilities and resilience**

#### Tasks:
1. **Implement Advanced Query Logic**
   ```go
   // internal/monitor/advanced_query.go
   package monitor

   type AdvancedQuery struct {
       Include  []string               `yaml:"include"`
       Exclude  []string               `yaml:"exclude"`
       MinValue float64               `yaml:"minValue"`
       MaxDays  int                   `yaml:"maxDaysOld"`
       Complex  map[string]interface{} `yaml:"complex"`
   }

   func (m *Monitor) FilterResults(opportunities []samgov.Opportunity, query AdvancedQuery) []samgov.Opportunity {
       filtered := make([]samgov.Opportunity, 0)
       
       for _, opp := range opportunities {
           if m.matchesAdvancedCriteria(opp, query) {
               filtered = append(filtered, opp)
           }
       }
       
       return filtered
   }
   ```

2. **Add Retry Logic and Circuit Breaker**
   ```go
   // internal/samgov/retry.go
   package samgov

   import (
       "context"
       "time"
       
       "github.com/cenkalti/backoff/v4"
   )

   type RetryClient struct {
       *Client
       maxRetries int
   }

   func (rc *RetryClient) SearchWithRetry(ctx context.Context, params map[string]string) (*SearchResponse, error) {
       var result *SearchResponse
       
       operation := func() error {
           resp, err := rc.Search(ctx, params)
           if err != nil {
               return err
           }
           result = resp
           return nil
       }
       
       backoffStrategy := backoff.NewExponentialBackOff()
       backoffStrategy.MaxElapsedTime = 30 * time.Second
       
       err := backoff.Retry(operation, backoff.WithContext(backoffStrategy, ctx))
       return result, err
   }
   ```

3. **Create Error Recovery System**
   ```gherkin
   # test/features/error_handling.feature
   Feature: Error Handling and Recovery
     As a monitoring system
     I need to handle errors gracefully
     So that temporary issues don't cause missed opportunities

     Scenario: API rate limit handling
       Given the API returns a 429 rate limit error
       When executing a search
       Then the system should wait and retry
       And log the rate limit event

     Scenario: Network timeout recovery
       Given a network timeout occurs
       When executing searches
       Then affected queries should be retried
       And successful queries should still process

     Scenario: Partial failure handling
       Given 3 queries are configured
       When 1 query fails and 2 succeed
       Then results from successful queries should be processed
       And an error report should be generated
       And the system should exit successfully
   ```

### Afternoon Session (3 hours)
**Goal: Performance optimization and monitoring**

#### Tasks:
1. **Implement Metrics Collection**
   ```go
   // internal/monitor/metrics.go
   package monitor

   import (
       "time"
       "sync"
   )

   type Metrics struct {
       mu sync.RWMutex
       
       QueryDurations map[string][]time.Duration
       APICallCount   int
       ErrorCount     int
       OpportunityCount map[string]int
   }

   func (m *Metrics) RecordQuery(name string, duration time.Duration) {
       m.mu.Lock()
       defer m.mu.Unlock()
       
       if m.QueryDurations == nil {
           m.QueryDurations = make(map[string][]time.Duration)
       }
       
       m.QueryDurations[name] = append(m.QueryDurations[name], duration)
   }

   func (m *Metrics) GenerateReport() MetricsReport {
       m.mu.RLock()
       defer m.mu.RUnlock()
       
       report := MetricsReport{
           Timestamp: time.Now(),
           Queries:   make(map[string]QueryMetrics),
       }
       
       for name, durations := range m.QueryDurations {
           var total time.Duration
           for _, d := range durations {
               total += d
           }
           
           report.Queries[name] = QueryMetrics{
               Count:    len(durations),
               AvgTime:  total / time.Duration(len(durations)),
               OppsFound: m.OpportunityCount[name],
           }
       }
       
       return report
   }
   ```

2. **Add Caching Layer**
   ```go
   // internal/cache/cache.go
   package cache

   import (
       "crypto/sha256"
       "encoding/json"
       "fmt"
       "time"
   )

   type Cache struct {
       store map[string]CacheEntry
       ttl   time.Duration
   }

   type CacheEntry struct {
       Data      interface{}
       ExpiresAt time.Time
   }

   func (c *Cache) Get(key string) (interface{}, bool) {
       entry, exists := c.store[key]
       if !exists || time.Now().After(entry.ExpiresAt) {
           return nil, false
       }
       return entry.Data, true
   }

   func (c *Cache) Set(key string, data interface{}) {
       c.store[key] = CacheEntry{
           Data:      data,
           ExpiresAt: time.Now().Add(c.ttl),
       }
   }

   func GenerateCacheKey(params map[string]string) string {
       data, _ := json.Marshal(params)
       hash := sha256.Sum256(data)
       return fmt.Sprintf("%x", hash)
   }
   ```

3. **Create Performance Dashboard**
   ```go
   // internal/dashboard/dashboard.go
   package dashboard

   func GenerateHTML(metrics MetricsReport, state *monitor.State) string {
       return fmt.Sprintf(`
   <!DOCTYPE html>
   <html>
   <head>
       <title>SAM.gov Monitor Dashboard</title>
       <style>
           .metric { 
               background: #f0f0f0; 
               padding: 20px; 
               margin: 10px;
               border-radius: 5px;
           }
           .chart { height: 300px; }
       </style>
   </head>
   <body>
       <h1>SAM.gov Monitor Dashboard</h1>
       <div class="metric">
           <h2>Last Run: %s</h2>
           <p>Total Opportunities Tracked: %d</p>
           <p>API Calls: %d</p>
           <p>Errors: %d</p>
       </div>
       
       <div class="metric">
           <h2>Query Performance</h2>
           %s
       </div>
   </body>
   </html>
   `, metrics.Timestamp.Format(time.RFC3339), 
      len(state.Opportunities),
      metrics.APICallCount,
      metrics.ErrorCount,
      generateQueryTable(metrics.Queries))
   }
   ```

**Deliverables:**
- ✅ Advanced query filtering
- ✅ Retry logic with backoff
- ✅ Error recovery system
- ✅ Performance metrics collection
- ✅ Caching implementation

---

## 📅 Day 6: Production Deployment & Polish

### Morning Session (2 hours)
**Goal: Security and production readiness**

#### Tasks:
1. **Security Audit Checklist**
   ```go
   // internal/security/audit.go
   package security

   func ValidateEnvironment() error {
       required := []string{
           "SAM_API_KEY",
           "SMTP_USERNAME",
           "SMTP_PASSWORD",
       }
       
       for _, env := range required {
           if val := os.Getenv(env); val == "" {
               return fmt.Errorf("missing required environment variable: %s", env)
           }
           
           // Check for common mistakes
           if strings.Contains(val, "example") || strings.Contains(val, "test") {
               return fmt.Errorf("environment variable %s appears to contain test data", env)
           }
       }
       
       return nil
   }
   ```

2. **Add Input Validation**
   ```go
   // internal/config/validator.go
   package config

   func (c *Config) Validate() error {
       if len(c.Queries) == 0 {
           return errors.New("no queries configured")
       }
       
       for i, query := range c.Queries {
           if query.Name == "" {
               return fmt.Errorf("query %d: name is required", i)
           }
           
           // Validate date parameters don't exceed 1 year
           if days, ok := query.Parameters["lookbackDays"].(int); ok && days > 365 {
               return fmt.Errorf("query %s: lookback days cannot exceed 365", query.Name)
           }
       }
       
       return nil
   }
   ```

3. **Create Deployment Guide**
   ```markdown
   # Deployment Guide

   ## Prerequisites
   - GitHub repository with Actions enabled
   - SAM.gov API key
   - SMTP credentials

   ## Setup Steps
   1. Fork this repository
   2. Configure secrets in Settings > Secrets
   3. Customize config/queries.yaml
   4. Enable GitHub Actions
   5. Run manual test with dry-run

   ## Monitoring
   - Check Actions tab for run history
   - Review artifacts for state files
   - Monitor email delivery
   ```

### Afternoon Session (2 hours)
**Goal: Final testing and launch**

#### Tasks:
1. **Run Full Integration Test**
   ```bash
   # Full system test script
   #!/bin/bash
   set -e

   echo "Running full system test..."
   
   # Validate environment
   go run ./cmd/monitor -validate-env
   
   # Run tests
   go test -v ./...
   
   # Dry run
   go run ./cmd/monitor -dry-run -v
   
   # Check metrics
   go run ./cmd/monitor -metrics
   
   echo "All tests passed!"
   ```

2. **Performance Benchmark**
   ```go
   // test/benchmark_test.go
   func BenchmarkQueryExecution(b *testing.B) {
       client := samgov.NewClient("test-key")
       params := map[string]string{
           "limit": "100",
           "postedFrom": "01/01/2024",
           "postedTo": "01/31/2024",
       }
       
       b.ResetTimer()
       for i := 0; i < b.N; i++ {
           _, _ = client.Search(context.Background(), params)
       }
   }
   ```

3. **Create Maintenance Automation**
   ```yaml
   # .github/workflows/maintenance.yml
   name: Weekly Maintenance

   on:
     schedule:
       - cron: '0 0 * * 0'  # Weekly on Sunday

   jobs:
     cleanup:
       runs-on: ubuntu-latest
       steps:
       - uses: actions/checkout@v4
       
       - name: Clean old state entries
         run: |
           go run ./cmd/monitor -cleanup -older-than 30d
           
       - name: Generate performance report
         run: |
           go run ./cmd/monitor -report -output reports/weekly.html
           
       - name: Upload reports
         uses: actions/upload-artifact@v3
         with:
           name: weekly-reports
           path: reports/
   ```

**Deliverables:**
- ✅ Security validation
- ✅ Input sanitization
- ✅ Deployment documentation
- ✅ Performance benchmarks
- ✅ Maintenance automation

---

## 🎉 Post-Launch Features

### Future Enhancements:
1. **GraphQL API Support** when SAM.gov implements it
2. **Machine Learning** for opportunity relevance scoring
3. **Multi-tenant Support** for teams
4. **Advanced Analytics** with Prometheus/Grafana
5. **Natural Language Queries** using AI

## 📊 Success Criteria

1. **Reliability**: 99.9% uptime (GitHub Actions SLA)
2. **Performance**: <5s per query execution
3. **Accuracy**: Zero false negatives
4. **Scalability**: Handle 100+ queries per run

## 🔧 Maintenance Checklist

- **Daily**: Automated runs (2x)
- **Weekly**: Review metrics, clean state
- **Monthly**: Update queries, review performance
- **Quarterly**: Security audit, feature review
