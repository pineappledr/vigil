# Build Stage - Use debian for reliable CGO compilation
FROM golang:1.25 AS builder
WORKDIR /app

# Copy go mod files first
COPY go.mod ./
COPY go.sum* ./

# Download dependencies and tidy
RUN go mod download || true
RUN go mod tidy

# Copy source code
COPY . .

# Build the server binary
RUN go build -ldflags="-s -w" -o vigil-server ./cmd/server

# Final Stage
FROM debian:bookworm-slim
WORKDIR /app

# Add ca-certificates for HTTPS
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user and data directory
RUN useradd -m -u 1000 vigil && \
    mkdir -p /data && \
    chown -R vigil:vigil /data

# Copy the binary
COPY --from=builder /app/vigil-server .

# Copy the web folder
COPY --from=builder /app/web ./web

# Change ownership
RUN chown -R vigil:vigil /app
USER vigil

# Set default database path to /data
ENV DB_PATH=/data/vigil.db

EXPOSE 9080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:9080/health || exit 1

CMD ["./vigil-server"]