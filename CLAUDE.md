# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a **SAM.gov Opportunity Monitor** project designed to be built in Go. The system monitors SAM.gov opportunities based on configurable search queries, runs twice daily via GitHub Actions, and sends email alerts for matches. Currently, this repository contains only planning documentation - the actual Go implementation has not yet been created.

## Current Repository State

**Status**: Day 3 Notification System Complete
- ✅ **Foundation Complete**: Go project structure, configuration, API client
- ✅ **Core Monitoring**: Concurrent query execution with state management
- ✅ **Deduplication**: Hash-based change detection and opportunity tracking
- ✅ **Retry Logic**: Exponential backoff for API reliability
- ✅ **Multi-Channel Notifications**: Email (HTML), Slack webhooks, GitHub issues
- ✅ **Calendar Integration**: Automatic .ics files for deadlines
- ✅ **Digest Mode**: Priority-based batching and immediate delivery
- ✅ **GitHub Actions**: Production-ready automated workflow
- 🚧 **Next**: Testing with real API and deployment

## Project Architecture (Planned)

Based on the documentation in `overview.md`, the system will have:

### Core Components
- **Go Binary**: Single-binary deployment with concurrent query execution
- **SAM.gov API Client**: RESTful client with retry logic and error handling
- **State Management**: JSON-based persistence to prevent duplicate notifications  
- **Notification System**: Email, Slack webhooks, and GitHub issue creation
- **GitHub Actions**: Automated twice-daily execution (8 AM and 6 PM ET)

### Planned Directory Structure
```
sam-gov-monitor/
├── .github/workflows/monitor.yml
├── cmd/monitor/main.go
├── internal/
│   ├── config/
│   ├── samgov/
│   ├── monitor/
│   ├── notify/
│   └── cache/
├── config/queries.yaml
├── test/features/
├── go.mod
├── Makefile
└── README.md
```

## Development Process

The project follows structured development practices outlined in `rules.md`:

### Session Management
- Document progress with timestamps and specific stopping points
- Use issue tracking for blockers and decisions
- Maintain detailed progress logs with time estimates vs. actuals

### Code Standards  
- Use Gherkin BDD for feature specifications
- Implement comprehensive error handling with retry logic
- Follow Go conventions for project structure and naming
- Write tests before implementing features

### Git Workflow
```
[TYPE][SCOPE]: Brief description (max 50 chars)

- What: [What changed]
- Why: [Why it changed]  
- Impact: [What this affects]

Roadmap: Step X.Y completed/in-progress
Issues: Closes #N, References #M
```

## Implementation Roadmap

The 6-day implementation plan from `roadmap.md` includes:

1. **Day 1**: Foundation & Go Setup
2. **Day 2**: Core Search Implementation  
3. **Day 3**: Notification System
4. **Day 4**: GitHub Action Integration
5. **Day 5**: Advanced Features & Error Handling
6. **Day 6**: Production Deployment & Polish

## Key Requirements

### Environment Variables (Required)
```
SAM_API_KEY=<your-sam-gov-api-key>
SMTP_HOST=<smtp-server>
SMTP_PORT=<smtp-port>
SMTP_USERNAME=<email-user>
SMTP_PASSWORD=<email-password>
EMAIL_FROM=<sender-email>
EMAIL_TO=<recipient-emails>
SLACK_WEBHOOK=<optional-slack-webhook>
```

### Build Commands
```bash
# Development
make build                    # Build binary
make run-dry                  # Run in dry-run mode
./bin/monitor -validate-env   # Check environment setup

# Testing  
go test ./...                               # Unit tests
go test -tags=integration ./test/...        # Integration tests (needs SAM_API_KEY)

# Production
./bin/monitor -config config/queries.yaml  # Run with real API calls

# Docker
make docker                   # Build Docker image
```

## Security Considerations

- API keys stored as GitHub Secrets only
- No credentials in code or logs
- Input validation on all query parameters
- TLS verification for all API calls
- Sanitized error messages in logs

## Testing Strategy

- Unit tests for all core functionality
- Integration tests with SAM.gov API
- BDD scenarios using Gherkin for business logic
- Performance benchmarks for query execution
- Dry-run mode for safe testing

## Next Steps for Implementation

When ready to begin coding:
1. Initialize Go module: `go mod init github.com/username/sam-gov-monitor`
2. Create basic project structure from roadmap
3. Implement SAM.gov API client with types
4. Add configuration loading and validation
5. Follow the 6-day roadmap for systematic development

## Notes

- The project emphasizes reliability and performance with Go's concurrency
- Designed for zero false negatives - missing an opportunity is not acceptable  
- Single binary deployment for maximum reliability in GitHub Actions
- Built for government contracting use cases (DARPA, defense contractors, small businesses)