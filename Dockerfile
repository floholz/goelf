# Build stage
FROM golang:1.21 AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y git ca-certificates gcc libc6-dev libsqlite3-dev && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with CGO enabled
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o goelf .

# Final stage
FROM debian:bookworm-slim

# Add labels for GitHub Container Registry
LABEL org.opencontainers.image.source="https://github.com/floholz/goelf"
LABEL org.opencontainers.image.description="European League of Football App - A Golang backend application for an HTMX webapp that displays European League Football data"
LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.vendor="floholz"
LABEL org.opencontainers.image.title="GOELF"
LABEL org.opencontainers.image.version="1.0.0"

# Install ca-certificates and sqlite runtime
RUN apt-get update && apt-get install -y ca-certificates sqlite3 wget && rm -rf /var/lib/apt/lists/*

# Create app user
RUN groupadd -g 1001 appgroup && \
    useradd -u 1001 -g appgroup -s /bin/bash appuser

# Create necessary directories and set proper permissions
RUN mkdir -p /app/database /app/assets /app/templates && \
    chown -R appuser:appgroup /app && \
    chmod 755 /app && \
    chmod 755 /app/database && \
    chmod 755 /app/assets && \
    chmod 755 /app/templates

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/goelf .

# Copy static files
COPY --from=builder /app/assets ./assets
COPY --from=builder /app/templates ./templates

# Make the binary executable and change ownership
RUN chmod +x /app/goelf && \
    chown appuser:appgroup /app/goelf && \
    chown -R appuser:appgroup /app/database

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 7788

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:7788/ || exit 1

# Run the application
CMD ["./goelf"] 