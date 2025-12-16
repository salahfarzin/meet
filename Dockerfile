# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')" \
    -o /app/meet-service ./main.go

# Final stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS, tzdata for timezones, golang-migrate for database migrations, and create non-root user for security
RUN apk --no-cache add ca-certificates tzdata && \
    wget -O- https://github.com/golang-migrate/migrate/releases/download/v4.16.2/migrate.linux-amd64.tar.gz | tar xvz && \
    mv migrate /usr/local/bin/migrate && \
    chmod +x /usr/local/bin/migrate && \
    addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Copy binary from builder
COPY --from=builder /app/meet-service .
COPY --from=builder /app/migrations ./migrations
COPY scripts/entrypoint.sh /app/entrypoint.sh

# Set ownership and permissions
RUN mkdir -p /app/storage/logs && \
    chown -R appuser:appgroup /app && \
    chmod +x /app/entrypoint.sh

USER appuser

EXPOSE 8080 50051

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

    # Run the application
ENTRYPOINT ["/app/entrypoint.sh"]