# SAM.gov Opportunity Monitor

A high-performance Go application that monitors SAM.gov opportunities based on configurable search queries, designed to run twice daily via GitHub Actions and send email alerts for matching opportunities.

## Features

- **Concurrent Query Execution**: Run multiple searches in parallel using Go goroutines
- **Intelligent Deduplication**: Track seen opportunities to prevent duplicate notifications  
- **Multi-Channel Notifications**: Email (HTML templates), Slack webhooks, and GitHub issues
- **Calendar Integration**: Automatic .ics files for opportunity deadlines
- **Digest Mode**: Batch low-priority notifications to reduce noise
- **Priority-Based Routing**: High-priority opportunities sent immediately
- **Flexible Configuration**: YAML-based query configuration with advanced filtering
- **Production Ready**: Single binary deployment with comprehensive error handling
- **BDD Testing**: Gherkin-based behavioral tests for reliability

## Quick Reference

**Common Search Terms by Industry:**
- **Defense Contractors**: `"artificial intelligence"`, `"surveillance"`, `"autonomous systems"`
- **IT Services**: `"cloud computing"`, `"cybersecurity"`, `"software development"`
- **Construction**: `"construction"`, `"renovation"`, `"facilities"`
- **Healthcare IT**: `"electronic health"`, `"medical records"`, `"telehealth"`

**Posting Types (ptype):**
- `s` = Solicitation (RFP/RFQ ready)
- `p` = Pre-solicitation (upcoming opportunities)
- `o` = Special Notice (announcements)
- `k` = Combined Synopsis/Solicitation
- `r` = Sources Sought (market research)

## Setup Guide for Your Organization

### Step 1: Get a SAM.gov API Key

1. **Register at SAM.gov**:
   - Go to [sam.gov](https://sam.gov)
   - Click "Sign In" → "Create an account"
   - Complete registration with your business email

2. **Request API Access**:
   - After logging in, go to your account settings
   - Navigate to "Account Details" → "Public API Key"
   - Click "Request Public API Key"
   - **Important**: API keys can take 1-2 business days to activate
   - You'll receive an email when your key is ready

### Step 2: Fork and Configure This Repository

1. **Fork this repository** to your GitHub account
2. **Clone your fork locally**:
   ```bash
   git clone https://github.com/YOUR-USERNAME/SAM.gov-API-notifier
   cd SAM.gov-API-notifier
   ```

### Step 3: Set Up GitHub Repository Secrets

Go to your repository's **Settings** → **Secrets and variables** → **Actions** and add these secrets:

#### Required Email Notification Secrets
| Secret Name | Description | Example |
|------------|-------------|---------|
| `SAM_API_KEY` | Your SAM.gov API key | `abc123xyz789...` |
| `EMAIL_FROM` | Sender email address | `notifications@yourcompany.com` |
| `EMAIL_TO` | Recipient email(s) - comma separated | `team@yourcompany.com` or `john@company.com,jane@company.com` |
| `SMTP_HOST` | SMTP server hostname | `smtp.gmail.com` or `smtp.office365.com` |
| `SMTP_PORT` | SMTP server port | `587` (common) or `465` |
| `SMTP_USERNAME` | SMTP authentication username | `notifications@yourcompany.com` |
| `SMTP_PASSWORD` | SMTP password/app password | See email provider instructions below |
| `SMTP_USE_TLS` | Use TLS encryption | `true` (recommended) |

#### Optional Notification Secrets

**For Slack Notifications:**
| Secret Name | Description | Example |
|------------|-------------|---------|
| `SLACK_WEBHOOK` | Slack webhook URL | `https://hooks.slack.com/services/T00/B00/XXX` |
| `SLACK_CHANNEL` | Channel to post to | `#opportunities` or `#contracts` |
| `SLACK_USERNAME` | Bot display name | `SAM.gov Monitor` |

**For GitHub Issue Creation:**
- `GITHUB_TOKEN` is automatically provided by GitHub Actions - no setup needed!

### Step 4: Configure Email Provider

<details>
<summary><b>Gmail Setup</b></summary>

1. Enable 2-factor authentication on your Google account
2. Generate an app password:
   - Go to [Google Account Settings](https://myaccount.google.com/)
   - Security → 2-Step Verification → App passwords
   - Generate a new app password for "Mail"
3. Use these settings:
   - `SMTP_HOST`: `smtp.gmail.com`
   - `SMTP_PORT`: `587`
   - `SMTP_USERNAME`: Your Gmail address
   - `SMTP_PASSWORD`: The generated app password (NOT your regular password)
</details>

<details>
<summary><b>Office 365 Setup</b></summary>

1. Use these settings:
   - `SMTP_HOST`: `smtp.office365.com`
   - `SMTP_PORT`: `587`
   - `SMTP_USERNAME`: Your Office 365 email
   - `SMTP_PASSWORD`: Your Office 365 password
2. Note: You may need to enable SMTP authentication in your admin panel
</details>

<details>
<summary><b>Other Email Providers</b></summary>

Common SMTP settings:
- **SendGrid**: Host: `smtp.sendgrid.net`, Port: `587`
- **AWS SES**: Host: `email-smtp.us-east-1.amazonaws.com`, Port: `587`
- **Mailgun**: Host: `smtp.mailgun.org`, Port: `587`
</details>

### Step 5: Customize Your Search Queries

Edit `config/queries.yaml` to match your organization's needs:

```yaml
queries:
  - name: "AI/ML Opportunities for [Your Company]"
    enabled: true
    parameters:
      title: "artificial intelligence"  # Main search term
      ptype: ["s", "p", "o", "k", "r"]  # Opportunity types you want
    notification:
      priority: high
      recipients: ["contracts@yourcompany.com"]
      channels: ["email"]
    advanced:
      include: ["your", "relevant", "keywords"]
      exclude: ["your", "exclusion", "terms"]
```

### Step 6: Test Your Setup

1. **Test locally** (optional):
   ```bash
   # Set up local environment
   export SAM_API_KEY="your-key"
   export EMAIL_TO="your-email@company.com"
   # ... other exports
   
   # Build and test
   make build
   ./bin/monitor -dry-run -v
   ```

2. **Test via GitHub Actions**:
   - Go to **Actions** tab in your repository
   - Click **"SAM.gov Opportunity Monitor"**
   - Click **"Run workflow"**
   - Select options:
     - ✅ `dry_run` (for testing)
     - ✅ `verbose` (for detailed output)
     - `lookback_days`: 7
   - Click **"Run workflow"**

### Step 7: Enable Automatic Monitoring

Once testing is successful:
1. The workflow automatically runs twice daily at 8 AM and 6 PM ET
2. You'll receive emails when new opportunities match your criteria
3. Monitor the **Actions** tab to ensure runs are successful

## Technical Details

### Local Development

For developers who want to run the monitor locally:

1. **Clone and build**:
   ```bash
   git clone https://github.com/YOUR-USERNAME/SAM.gov-API-notifier
   cd SAM.gov-API-notifier
   make build
   ```

2. **Set environment variables**:
   ```bash
   export SAM_API_KEY="your-api-key"
   export SMTP_HOST="smtp.gmail.com"
   export SMTP_PORT="587"
   export SMTP_USERNAME="your-email@gmail.com"
   export SMTP_PASSWORD="your-app-password"
   export EMAIL_FROM="your-email@gmail.com"
   export EMAIL_TO="recipient@company.com"
   ```

3. **Test your setup**:
   ```bash
   ./bin/monitor -validate-env
   ./bin/monitor -dry-run -v
   ```

4. **Run with your configuration**:
   ```bash
   ./bin/monitor -config config/queries.yaml
   ```

## Configuration

Edit `config/queries.yaml` to define your search criteria:

```yaml
queries:
  - name: "DARPA AI Opportunities"
    enabled: true
    parameters:
      title: "artificial intelligence"
      organizationName: "DEFENSE ADVANCED RESEARCH PROJECTS AGENCY"
      ptype: ["s", "p", "o"]
    notification:
      priority: high
      recipients: ["team@company.com"]
      channels: ["email", "slack"]
    advanced:
      include: ["AI", "machine learning"]
      exclude: ["canceled", "withdrawn"]
      maxDaysOld: 30
```

### Query Parameters

Common SAM.gov API parameters:
- `title`: Keywords in opportunity title (e.g., "artificial intelligence", "surveillance")
- `organizationName`: Agency name (e.g., "DEFENSE ADVANCED RESEARCH PROJECTS AGENCY")
- `ptype`: Posting type array (can include multiple):
  - `s` = Solicitation
  - `p` = Pre-solicitation  
  - `o` = Special Notice
  - `k` = Combined Synopsis/Solicitation
  - `r` = Sources Sought
- `typeOfSetAside`: Set-aside types (`SBA`, `8A`, `WOSB`, etc.)
- `naicsCode`: NAICS classification codes
- `state`: Two-letter state code

### Advanced Filtering

The monitor applies these filters AFTER retrieving results from SAM.gov:

- `include`: Keywords that must be present (uses intelligent matching)
- `exclude`: Keywords that must not be present  
- `minValue`/`maxValue`: Contract value range
- `maxDaysOld`: Maximum age of opportunities
- `setAsideTypes`: Required set-aside types
- `naicsCodes`: Required NAICS codes

**Important Notes on Filtering:**

1. **API Limits**: SAM.gov returns up to 1000 results per query, so use specific title searches
2. **Smart Matching**: Short keywords like "AI" use word-boundary detection to avoid false matches
3. **Generic Terms**: Terms like "monitoring system" require additional context keywords to match
4. **Exclude Filters**: Applied first to quickly eliminate irrelevant results

## Effective Query Strategies

### Choosing the Right Title Search

The `title` parameter is crucial because SAM.gov limits API responses to 1000 results:

**Too Broad** ❌
```yaml
title: "system"  # Returns thousands of unrelated results
```

**Too Narrow** ❌  
```yaml
title: "brain-inspired adaptive AI"  # May miss relevant opportunities
```

**Just Right** ✅
```yaml
title: "artificial intelligence"  # Specific but inclusive
# or
title: "machine learning"
# or  
title: "surveillance"
```

### Example Configurations

**For AI/ML Defense Opportunities:**
```yaml
queries:
  - name: "Defense AI Systems"
    enabled: true
    parameters:
      title: "artificial intelligence"
      ptype: ["o", "k", "p", "r", "s"]  # All relevant types
    advanced:
      include: ["edge AI", "autonomous", "defense", "military", "surveillance"]
      exclude: ["medical", "healthcare", "education"]
```

**For Surveillance/Security Systems:**
```yaml
queries:
  - name: "Surveillance Systems"
    enabled: true
    parameters:
      title: "surveillance"
      ptype: ["k", "s", "p"]  # Focus on solicitations
    advanced:
      include: ["video", "camera", "AI", "analytics", "detection"]
      exclude: ["maintenance only", "repair only"]
```

### Debugging Your Filters

1. **Start with dry-run mode** to see what matches:
   ```bash
   ./bin/monitor -dry-run -v -lookback 7
   ```

2. **Check the logs** for API response details:
   - Total records available
   - Number returned (limited to 1000)
   - Number passing your filters

3. **Adjust incrementally**:
   - If too few results: broaden title search or reduce include filters
   - If too many results: add more specific include keywords or exclude filters

## Usage

### Command Line Options

```bash
./bin/monitor [options]

Options:
  -config string     Path to config file (default "config/queries.yaml")
  -state string      Path to state file (default "state/monitor.json")
  -dry-run          Run without sending notifications
  -v                Verbose output
  -validate-env     Validate environment and exit
  -lookback int     Days to look back (default 3)
  -help             Show help
```

### Examples

```bash
# Test configuration without sending notifications
./bin/monitor -dry-run -v

# Run with custom lookback period
./bin/monitor -lookback 7 -v

# Validate environment setup
./bin/monitor -validate-env

# Use custom config file
./bin/monitor -config my-queries.yaml
```

## Automated Monitoring with GitHub Actions

The repository includes a GitHub Actions workflow that runs automatically twice daily. See the setup guide above for configuring secrets.

## Manual Workflow Execution

**Important**: The workflow does NOT run automatically when you push code changes. It only runs on the scheduled times above OR when manually triggered.

### How to Manually Trigger the Workflow

1. Go to your GitHub repository
2. Click the **"Actions"** tab
3. Select **"SAM.gov Opportunity Monitor"** workflow
4. Click the **"Run workflow"** button (blue button on the right)
5. Configure the run parameters:

**Available Parameters:**
- **`dry_run`** (checkbox): 
  - ✅ **Checked**: Test run without sending notifications (safe for testing)
  - ❌ **Unchecked**: Send real emails/notifications (use for production)
- **`verbose`** (checkbox): Enable detailed logging output
- **`lookback_days`** (number): Days to search backwards
  - **Default**: 3 days
  - **Recommended for first run**: 7-14 days to catch recent opportunities
  - **For testing**: 1-2 days to limit results

### Common Manual Run Scenarios

**First-time setup / Testing:**
```
dry_run: ✅ (checked)
verbose: ✅ (checked) 
lookback_days: 7
```

**Production run after config changes:**
```
dry_run: ❌ (unchecked)
verbose: ❌ (unchecked)
lookback_days: 3
```

**Catch up after downtime:**
```
dry_run: ❌ (unchecked)
verbose: ❌ (unchecked)
lookback_days: 14
```

6. Click **"Run workflow"** to start the execution

### Monitoring Manual Runs

- Check the **Actions** tab to see run status
- Click on the running workflow to see live logs
- Failed runs will show error details in the logs
- State is automatically saved between runs to prevent duplicates

## Development

### Build Commands

```bash
make build          # Build binary
make test           # Run tests
make test-integration # Run integration tests (needs API key)
make run-dry        # Run in dry-run mode
make lint           # Run linter
make coverage       # Generate coverage report
```

### Testing

The project uses BDD testing with Gherkin scenarios:

```bash
# Unit tests
go test ./...

# Integration tests (requires SAM_API_KEY)
go test -tags=integration ./test/...

# Specific feature tests
go test ./test/features/
```

### Project Structure

```
sam-gov-monitor/
├── cmd/monitor/           # Main application
├── internal/
│   ├── config/           # Configuration loading and validation
│   ├── samgov/           # SAM.gov API client
│   ├── monitor/          # Core monitoring logic
│   ├── notify/           # Notification system
│   └── cache/            # Caching layer
├── config/               # Configuration files
├── test/
│   ├── features/         # Gherkin BDD tests
│   └── integration/      # Integration tests
└── .github/workflows/    # GitHub Actions
```

## Troubleshooting

### Common Issues

1. **API Key Not Working**
   - Verify key is active (can take 1-2 business days)
   - Check for typos in environment variable
   - Run `./bin/monitor -validate-env`

2. **No Opportunities Found**
   - Try broader search terms
   - Increase lookback period: `-lookback 7`
   - Check with dry-run: `-dry-run -v`

3. **Network Timeouts**
   - SAM.gov API can be slow during peak hours
   - The system automatically retries with backoff
   - Consider running during off-peak hours

4. **Too Many/Few Results After Filtering**
   - **Getting too many results**: Your `title` search is too broad (e.g., "system")
   - **Missing relevant results**: Your `include` filters may be too restrictive
   - **False positives**: Add more specific `exclude` keywords
   - Use `-v` flag to see filtering details:
     ```
     API Response: TotalRecords=450, Returned=450, Limit=1000
     Query 'Defense AI': 450 total, 12 new, 3 updated
     ```

5. **Lookback Period Not Working**
   - SAM.gov may have fewer results than expected for your search
   - The API returns results sorted by date (newest first)
   - Even with `-lookback 90`, you'll only get up to 1000 total results

### Debug Mode

```bash
# Verbose output with all details
./bin/monitor -v -dry-run

# Test single query manually
go run ./cmd/monitor -config config/single-query.yaml -v
```

## Monitoring and Maintenance

- **State File**: `state/monitor.json` tracks seen opportunities
- **Logs**: All runs are logged with timestamps and query results
- **Metrics**: Built-in performance tracking and error reporting
- **Cleanup**: Old state entries are automatically pruned

## Security

- API keys stored as GitHub Secrets only
- No credentials in code or logs
- TLS verification for all API calls
- Input validation on all parameters
- Sanitized error messages

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass: `make test`
5. Submit a pull request

## License

MIT License - see LICENSE file for details.

## Support

- 📧 Issues: [GitHub Issues](https://github.com/yourusername/sam-gov-monitor/issues)
- 📖 Documentation: See `docs/` directory
- 💬 Discussions: [GitHub Discussions](https://github.com/yourusername/sam-gov-monitor/discussions)