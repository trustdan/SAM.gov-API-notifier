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

## Quick Start

### Prerequisites

1. Get a SAM.gov API key:
   - Register at [sam.gov](https://sam.gov)
   - Go to Account Details → Request Public API Key
   - Note: API keys can take 1-2 business days to activate

2. Set up environment variables:
```bash
export SAM_API_KEY="your-api-key-here"

# Email notifications (optional)
export SMTP_HOST="smtp.gmail.com"
export SMTP_PORT="587"
export SMTP_USERNAME="your-email@gmail.com"
export SMTP_PASSWORD="your-app-password"
export EMAIL_FROM="your-email@gmail.com"
export EMAIL_TO="recipient@company.com"

# Slack notifications (optional)
export SLACK_WEBHOOK="https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK"
export SLACK_CHANNEL="#opportunities"

# GitHub notifications (optional - auto-set in Actions)
export GITHUB_TOKEN="your-github-token"
export GITHUB_OWNER="yourusername"
export GITHUB_REPOSITORY="sam-gov-monitor"
```

### Installation

1. Clone and build:
```bash
git clone https://github.com/yourusername/sam-gov-monitor
cd sam-gov-monitor
make build
```

2. Test your setup:
```bash
./bin/monitor -validate-env
./bin/monitor -dry-run -v
```

3. Run with your configuration:
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
- `title`: Keywords in opportunity title
- `organizationName`: Agency name (e.g., "DARPA", "DOD")
- `ptype`: Posting type (`s`=Solicitation, `p`=Pre-solicitation, `o`=Special Notice)
- `typeOfSetAside`: Set-aside types (`SBA`, `8A`, `WOSB`, etc.)
- `naicsCode`: NAICS classification codes
- `state`: Two-letter state code

### Advanced Filtering

- `include`: Keywords that must be present
- `exclude`: Keywords that must not be present  
- `minValue`/`maxValue`: Contract value range
- `maxDaysOld`: Maximum age of opportunities
- `setAsideTypes`: Required set-aside types
- `naicsCodes`: Required NAICS codes

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

## GitHub Actions Setup

### Prerequisites

1. Set up your local environment variables in `.env` file (for local development):
```bash
# Required
SAM_API_KEY=your-api-key-here

# Email notifications (required for GitHub Actions)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your-email@gmail.com
SMTP_PASSWORD=your-app-password
EMAIL_FROM=your-email@gmail.com
EMAIL_TO=recipient@company.com

# Slack notifications (optional)
SLACK_WEBHOOK=https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK
SLACK_CHANNEL=#opportunities
SLACK_USERNAME=SAM-Monitor
```

### GitHub Repository Setup

1. Fork this repository

2. **Add Repository Secrets**: Go to Settings → Secrets and variables → Actions and add:

   **Required Secrets:**
   - `SAM_API_KEY`: Your SAM.gov API key
   - `SMTP_HOST`: SMTP server hostname (e.g., `smtp.gmail.com`)
   - `SMTP_PORT`: SMTP server port (e.g., `587`)
   - `SMTP_USERNAME`: Email username
   - `SMTP_PASSWORD`: Email password or app password
   - `EMAIL_FROM`: Sender email address
   - `EMAIL_TO`: Recipient email address(es)

   **Optional Secrets (for Slack notifications):**
   - `SLACK_WEBHOOK`: Slack webhook URL
   - `SLACK_CHANNEL`: Slack channel name (e.g., `#opportunities`)
   - `SLACK_USERNAME`: Bot display name (e.g., `SAM-Monitor`)

   **Auto-provided (no action needed):**
   - `GITHUB_TOKEN`: Automatically provided by GitHub

3. Enable GitHub Actions in the Actions tab

4. The workflow runs automatically twice daily:
   - 8:00 AM ET (12:00 UTC)
   - 6:00 PM ET (22:00 UTC)

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