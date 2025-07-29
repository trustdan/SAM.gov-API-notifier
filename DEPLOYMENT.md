# SAM.gov Monitor - Production Deployment Guide

This guide provides step-by-step instructions for deploying the SAM.gov Opportunity Monitor in production environments.

## 📋 Prerequisites

### Required Accounts and Access
- [x] GitHub account with repository access
- [x] SAM.gov account with API key
- [x] Email service (Gmail, Outlook, or SMTP server)
- [x] Slack workspace (optional)

### Technical Requirements
- [x] GitHub Actions enabled on repository
- [x] Go 1.21+ (for local development/testing)
- [x] Docker (optional, for containerized deployment)

## 🔑 Step 1: Obtain Required Credentials

### 1.1 SAM.gov API Key
1. Register at [sam.gov](https://sam.gov)
2. Navigate to Account Details → Request Public API Key
3. Wait 1-2 business days for key activation
4. Test key with: `curl "https://api.sam.gov/opportunities/v2/search?api_key=YOUR_KEY&limit=1"`

### 1.2 Email Credentials
**For Gmail:**
```bash
# Use App Passwords (not your regular password)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your-email@gmail.com
SMTP_PASSWORD=your-app-password
EMAIL_FROM=your-email@gmail.com
EMAIL_TO=recipient1@company.com,recipient2@company.com
```

**For Other SMTP Providers:**
```bash
SMTP_HOST=your-smtp-server.com
SMTP_PORT=587  # or 465 for SSL
SMTP_USERNAME=your-username
SMTP_PASSWORD=your-password
EMAIL_FROM=sender@company.com
EMAIL_TO=recipient@company.com
```

### 1.3 Slack Integration (Optional)
1. Create a Slack App in your workspace
2. Enable Incoming Webhooks
3. Create webhook URL for your channel
4. Note webhook URL: `https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX`

## 🚀 Step 2: GitHub Actions Deployment

### 2.1 Fork Repository
1. Fork this repository to your GitHub account
2. Clone your fork locally:
```bash
git clone https://github.com/YOUR_USERNAME/sam-gov-monitor
cd sam-gov-monitor
```

### 2.2 Configure GitHub Secrets
Go to Settings → Secrets and variables → Actions and add:

**Required Secrets:**
```
SAM_API_KEY=your-sam-gov-api-key-here
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your-email@gmail.com
SMTP_PASSWORD=your-app-password
EMAIL_FROM=your-email@gmail.com
EMAIL_TO=recipient@company.com
```

**Optional Secrets:**
```
SLACK_WEBHOOK=https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK
SLACK_CHANNEL=#opportunities
SLACK_USERNAME=SAM.gov Monitor
GITHUB_TOKEN=auto-generated-by-github
```

### 2.3 Customize Configuration
1. Edit `config/queries.yaml` for your specific needs
2. Test configuration locally:
```bash
# Validate configuration
make validate-env

# Test with dry run
make run-dry
```

### 2.4 Enable GitHub Actions
1. Go to Actions tab in your repository
2. Enable workflows if prompted
3. The monitor will run automatically at 8 AM and 6 PM ET

### 2.5 Manual Test Run
1. Go to Actions → "SAM.gov Opportunity Monitor"
2. Click "Run workflow"
3. Set parameters:
   - `dry_run`: ✅ (checked) for testing
   - `verbose`: ✅ (checked) for detailed logs
   - `lookback_days`: 7 (for testing)
4. Click "Run workflow"
5. Monitor logs for any issues

## 🐳 Step 3: Docker Deployment (Alternative)

### 3.1 Create Environment File
```bash
# Create .env file (DO NOT commit to git)
cat > .env << 'EOF'
SAM_API_KEY=your-sam-gov-api-key-here
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your-email@gmail.com
SMTP_PASSWORD=your-app-password
EMAIL_FROM=your-email@gmail.com
EMAIL_TO=recipient@company.com
SLACK_WEBHOOK=https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK
EOF
```

### 3.2 Build and Run Container
```bash
# Build image
make docker

# Run with docker-compose (production)
make docker-up

# Run development version
make docker-dev

# View logs
make docker-logs

# Stop services
make docker-down
```

### 3.3 Production Docker Deployment
```bash
# For production, use specific scheduling
docker run -d \
  --name sam-monitor \
  --env-file .env \
  -v $(pwd)/config:/app/config:ro \
  -v $(pwd)/state:/app/state \
  --restart unless-stopped \
  sam-gov-monitor:latest \
  -config config/queries.yaml -state state/monitor.json
```

## 🔧 Step 4: Local Development Setup

### 4.1 Install Dependencies
```bash
# Install Go dependencies
make deps

# Install development tools (optional)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### 4.2 Set Environment Variables
```bash
# Add to your shell profile (.bashrc, .zshrc, etc.)
export SAM_API_KEY="your-api-key"
export SMTP_HOST="smtp.gmail.com"
export SMTP_PORT="587"
export SMTP_USERNAME="your-email@gmail.com"
export SMTP_PASSWORD="your-app-password"
export EMAIL_FROM="your-email@gmail.com"
export EMAIL_TO="recipient@company.com"
```

### 4.3 Build and Test
```bash
# Build application
make build

# Validate environment
make validate-env

# Run tests
make test

# Run integration tests (requires SAM_API_KEY)
make test-integration

# Run in dry-run mode
make run-dry

# Run normally (sends real notifications)
make run
```

## 🛡️ Step 5: Security Hardening

### 5.1 Run Security Audit
```bash
# Build and run security validation
make build
./bin/monitor -validate-env -v

# Check configuration security
./bin/monitor -config config/queries.yaml -dry-run -v
```

### 5.2 Security Checklist
- [x] API keys stored only in GitHub Secrets
- [x] No credentials in code or configuration files
- [x] SMTP credentials use app passwords, not account passwords
- [x] Email notifications go to appropriate recipients
- [x] File permissions are restrictive (600 for sensitive files)
- [x] Container runs as non-root user
- [x] All HTTP connections use HTTPS/TLS

### 5.3 Environment Validation
The application includes built-in security validation:
```bash
# Validate all security settings
./bin/monitor -validate-env

# Run full security audit
go run ./cmd/monitor -config config/queries.yaml -dry-run
```

## 📊 Step 6: Monitoring and Maintenance

### 6.1 Monitor GitHub Actions
- Check Actions tab regularly for failed runs
- Review execution logs for errors or warnings
- Monitor artifact storage (state files are automatically managed)

### 6.2 Performance Monitoring
```bash
# Generate performance report
./bin/monitor -report

# View cache statistics (if using local deployment)
./bin/monitor -report | grep -A 10 "Cache Statistics"

# Check metrics file
cat state/metrics.json | jq .
```

### 6.3 Log Management
- GitHub Actions logs are retained for 90 days
- State files are preserved between runs via artifacts
- Failed runs automatically upload debug logs

### 6.4 Maintenance Tasks

**Weekly:**
- Review notification effectiveness
- Check for new query opportunities
- Verify no failed runs

**Monthly:**
- Update query configurations if needed
- Review and clean up old state data
- Check for application updates

**Quarterly:**
- Rotate API keys and passwords
- Security audit review
- Performance optimization review

## 🔄 Step 7: Configuration Management

### 7.1 Updating Queries
1. Edit `config/queries.yaml`
2. Validate locally: `make run-dry`
3. Commit and push changes
4. Test with manual GitHub Actions run

### 7.2 Environment Updates
1. Update GitHub Secrets
2. Test with dry-run workflow
3. Monitor next scheduled run

### 7.3 Scaling Considerations
- **Query Limits**: Recommend max 20 queries per run
- **API Rate Limits**: Built-in retry logic handles SAM.gov limits
- **Notification Limits**: Monitor email/Slack rate limits
- **Storage**: State files grow slowly, cleanup is automatic

## 🚨 Step 8: Troubleshooting

### 8.1 Common Issues

**API Key Problems:**
```bash
# Test API key manually
curl "https://api.sam.gov/opportunities/v2/search?api_key=YOUR_KEY&limit=1"

# Check key format and expiration
./bin/monitor -validate-env
```

**Email Issues:**
```bash
# Test SMTP connectivity
telnet smtp.gmail.com 587

# Verify app password setup (Gmail)
# Use App Passwords, not account password
```

**GitHub Actions Failures:**
- Check secrets are properly set
- Verify YAML syntax in configuration
- Review action logs for specific errors

### 8.2 Debug Mode
```bash
# Enable verbose logging
./bin/monitor -config config/queries.yaml -dry-run -v

# Generate debug report
./bin/monitor -report > debug-report.txt
```

### 8.3 Getting Help
1. Check GitHub Issues for similar problems
2. Review logs in GitHub Actions
3. Use dry-run mode to test changes safely
4. Validate configuration with built-in tools

## ✅ Step 9: Production Readiness Checklist

**Before Going Live:**
- [x] All credentials properly configured in GitHub Secrets
- [x] Configuration validated with dry-run
- [x] Test notification received successfully
- [x] Security audit passed
- [x] Backup/recovery plan documented
- [x] Monitoring and alerting configured
- [x] Team trained on maintenance procedures

**Post-Deployment:**
- [x] Monitor first few runs closely
- [x] Verify notifications are being received
- [x] Check no false positives or missed opportunities
- [x] Document any custom configurations
- [x] Schedule regular maintenance tasks

## 📈 Step 10: Optimization and Scaling

### 10.1 Performance Optimization
```bash
# Monitor query performance
./bin/monitor -report | grep -A 5 "Query Performance"

# Optimize slow queries
# Add more specific search criteria
# Use include/exclude filters
# Reduce lookback periods for broad queries
```

### 10.2 Advanced Configuration
```yaml
# Example optimized query
- name: "High-Value AI Contracts"
  enabled: true
  parameters:
    title: "artificial intelligence"
    organizationName: "DARPA"
    ptype: ["s", "p"]
  notification:
    priority: high
    recipients: ["ai-team@company.com"]
    channels: ["email", "slack"]
  advanced:
    include: ["machine learning", "AI", "neural network"]
    exclude: ["training", "educational", "simulation"]
    minValue: 100000
    maxDaysOld: 14
```

### 10.3 Multi-Environment Setup
```bash
# Development environment
cp config/queries.yaml config/queries-dev.yaml
# Edit for development-specific queries

# Staging environment  
cp config/queries.yaml config/queries-staging.yaml
# Edit for staging-specific queries

# Use environment-specific configs
./bin/monitor -config config/queries-dev.yaml -dry-run
```

---

## 🎯 Quick Start Summary

For experienced users, here's the minimal setup:

```bash
# 1. Fork repo and clone
git clone https://github.com/YOUR_USERNAME/sam-gov-monitor

# 2. Set GitHub Secrets: SAM_API_KEY, SMTP_*, EMAIL_*

# 3. Test locally
make build && make validate-env && make run-dry

# 4. Enable GitHub Actions and run manual test

# 5. Monitor scheduled runs at 8 AM and 6 PM ET
```

**That's it!** Your SAM.gov monitor is now running in production. 🚀