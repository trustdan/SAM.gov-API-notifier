# Running SAM.gov Monitor Locally

When encountering rate limits from GitHub Actions, you can run the monitor directly from your local machine with enhanced rate limit handling.

## Quick Start

1. **Copy the example environment file:**
   ```bash
   cp .env.example .env
   ```

2. **Edit `.env` with your credentials:**
   - Add your SAM.gov API key
   - Configure email/Slack/GitHub settings as needed
   - Adjust rate limit settings if needed

3. **Run the monitor:**
   ```bash
   ./scripts/run-local.sh
   ```

## Features

The local runner provides:

- **Custom User-Agent**: Identifies as a local client instead of GitHub Actions
- **Enhanced Rate Limiting**: Configurable delays and retry attempts
- **Exponential Backoff with Jitter**: Prevents thundering herd on retries
- **Environment File Support**: Easy configuration management
- **Debug Logging**: Detailed output for troubleshooting

## Configuration Options

### Rate Limit Settings

- `SAM_RATE_LIMIT_DELAY`: Initial delay between requests (default: 10s)
- `SAM_MAX_RETRIES`: Maximum retry attempts (default: 5)
- `SAM_USER_AGENT`: Custom user agent string

### Example for Aggressive Rate Limits

If you're experiencing persistent rate limits, try these settings in your `.env`:

```bash
# Conservative settings for heavy rate limiting
SAM_RATE_LIMIT_DELAY=30s
SAM_MAX_RETRIES=10
SAM_USER_AGENT=SAM.gov-Monitor-Local/1.0 (personal-research)
```

## Troubleshooting

### Still Getting Rate Limited?

1. **Increase delays**: Set `SAM_RATE_LIMIT_DELAY=60s` or higher
2. **Run during off-peak hours**: Early morning or late evening
3. **Reduce query frequency**: Modify `config/queries.yaml` to fewer terms
4. **Contact SAM.gov**: Your IP might be flagged - request whitelisting

### Debugging

The local runner enables debug mode by default. Check the output for:
- Exact API responses
- Retry attempts and delays
- User-Agent being sent
- Rate limit headers from the API

## Manual Build and Run

If the script doesn't work, you can run manually:

```bash
# Build
go build -o bin/monitor cmd/monitor/main.go

# Set environment
export SAM_API_KEY="your_key"
export SAM_USER_AGENT="SAM.gov-Monitor-Local/1.0"
export SAM_RATE_LIMIT_DELAY="15s"
export SAM_MAX_RETRIES="5"

# Run
./bin/monitor -config config/queries.yaml
```

## Cron Job Setup (Optional)

To run automatically on your local machine:

```bash
# Edit crontab
crontab -e

# Add entry (runs at 8 AM and 6 PM)
0 8,18 * * * cd /path/to/SAM.gov-API-notifier && ./scripts/run-local.sh >> logs/monitor.log 2>&1
```

## Security Notes

- Never commit your `.env` file (it's in `.gitignore`)
- Keep your API keys secure
- Use app-specific passwords for email
- Rotate credentials regularly