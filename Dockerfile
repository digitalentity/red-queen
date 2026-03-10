# Stage 1: Build
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache make git

WORKDIR /app

# Copy dependency files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the binary
RUN make build

# Stage 2: Runtime
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/red-queen .

# Create directories for uploads and storage
RUN mkdir -p /data/uploads /data/storage

# Expose FTP (2121) and REST API (8080) ports
EXPOSE 2121 8080

# Default environment variables
ENV RED_QUEEN_CONFIG=/config/config.yaml

# Run the application
ENTRYPOINT ["./red-queen"]
