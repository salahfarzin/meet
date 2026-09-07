# Build stage
FROM golang:1.27.1-alpine AS builder

ARG TARGETARCH

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application (TARGETARCH matches the build host natively — no
# emulation needed on arm64 like a hardcoded amd64 would force)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')" \
    -o /app/meet-service ./main.go

# Final stage
FROM alpine:3.19

ARG TARGETARCH

WORKDIR /app

# Install ca-certificates for HTTPS, tzdata for timezones, golang-migrate for database migrations, and create non-root user for security
RUN apk --no-cache add ca-certificates tzdata && \
    wget -O- https://github.com/golang-migrate/migrate/releases/download/v4.16.2/migrate.linux-${TARGETARCH}.tar.gz | tar xvz && \
    mv migrate /usr/local/bin/migrate && \
    chmod +x /usr/local/bin/migrate && \
    addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup && \
    mkdir -p /app/storage/logs && \
    chown -R appuser:appgroup /app

# Copy binary from builder (--chown here, not a separate RUN chown -R after —
# that would duplicate the binary's storage cost via copy-on-write)
COPY --chown=appuser:appgroup --from=builder /app/meet-service .
COPY --chown=appuser:appgroup --from=builder /app/migrations ./migrations
COPY --chown=appuser:appgroup scripts/entrypoint.sh /app/entrypoint.sh

RUN chmod +x /app/entrypoint.sh

USER appuser

EXPOSE 8083 9093

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8083/api/v1/health || exit 1

# Run the application
ENTRYPOINT ["/app/entrypoint.sh"]