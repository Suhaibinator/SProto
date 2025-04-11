# --- Stage 1: Build ---
# Use a Go version compatible with go.mod requirements
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /app

# Copy Go module files
COPY go.mod go.sum ./
# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the server binary statically linked
# Using CGO_ENABLED=0 ensures static linking without C libraries
# -ldflags="-w -s" strips debug information and symbols for a smaller binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /sproto-server ./cmd/server

# --- Stage 2: Runtime ---
# Use a minimal base image
FROM alpine:latest

# Install ca-certificates for HTTPS communication if needed (e.g., external APIs)
# Also create a non-root user for security
RUN apk --no-cache add ca-certificates && \
    adduser -D -u 1001 -g 1001 sproto-user

# Set working directory
WORKDIR /app

# Copy the built binary from the builder stage
COPY --from=builder /sproto-server /app/sproto-server

# Change ownership to the non-root user
RUN chown sproto-user:sproto-user /app/sproto-server

# Switch to the non-root user
USER sproto-user

# Expose the default server port (can be overridden by environment variable)
EXPOSE 8080

# Command to run the server
ENTRYPOINT ["/app/sproto-server"]
