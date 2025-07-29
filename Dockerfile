# Build stage
FROM golang:1.21-alpine AS builder

# Install git and ca-certificates (needed for go modules and HTTPS)
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build the binary with version info
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-w -s -X main.Version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev') -X main.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o monitor ./cmd/monitor

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN adduser -D -s /bin/sh sammonitor

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/monitor .

# Copy config directory (can be overridden with volume mounts)
COPY --from=builder /app/config ./config

# Create state directory
RUN mkdir -p state && chown sammonitor:sammonitor state

# Switch to non-root user
USER sammonitor

# Set default command
ENTRYPOINT ["./monitor"]
CMD ["-config", "config/queries.yaml"]

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ./monitor -validate-env || exit 1

# Labels for metadata
LABEL maintainer="SAM.gov Monitor <noreply@example.com>"
LABEL version="1.0.0"
LABEL description="SAM.gov Opportunity Monitor - Automated government contract opportunity monitoring"
LABEL org.opencontainers.image.source="https://github.com/yourusername/sam-gov-monitor"
LABEL org.opencontainers.image.documentation="https://github.com/yourusername/sam-gov-monitor/blob/main/README.md"