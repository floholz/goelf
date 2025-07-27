# Build stage
FROM golang:1.21-alpine AS builder

# Install git and ca-certificates (needed for go mod download)
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o main .

# Final stage
FROM alpine:latest

# Add labels for GitHub Container Registry
LABEL org.opencontainers.image.source="https://github.com/floholz/goelf"
LABEL org.opencontainers.image.description="European League of Football App - A Golang backend application for an HTMX webapp that displays European League Football data"
LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.vendor="floholz"
LABEL org.opencontainers.image.title="GOELF"
LABEL org.opencontainers.image.version="1.0.0"

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates sqlite

# Create app user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Create necessary directories
RUN mkdir -p /app/database /app/assets /app/templates && \
    chown -R appuser:appgroup /app

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/main .

# Copy static files
COPY --from=builder /app/assets ./assets
COPY --from=builder /app/templates ./templates

# Change ownership of the binary
RUN chown appuser:appgroup /app/main

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/ || exit 1

# Run the application
CMD ["./main"] 