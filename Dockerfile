# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o codementor ./cmd/codementor

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /app/codementor .
COPY --from=builder /app/configs ./configs

# Create data directory
RUN mkdir -p /app/data /app/.codementor

# Environment variables
ENV CODEMENTOR_OLLAMA_HOST=http://host.docker.internal:11434
ENV CODEMENTOR_SERVER_HOST=0.0.0.0
ENV CODEMENTOR_SERVER_PORT=8080

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Default command
ENTRYPOINT ["./codementor"]
CMD ["serve"]

