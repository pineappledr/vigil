# Build Stage - Use debian for reliable CGO compilation
FROM golang:1.25 AS builder
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

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

# Create non-root user
RUN useradd -m -u 1000 vigil

# Copy the binary
COPY --from=builder /app/vigil-server .

# Copy the web folder
COPY --from=builder /app/web ./web

# Change ownership
RUN chown -R vigil:vigil /app
USER vigil

EXPOSE 8090

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8090/health || exit 1

CMD ["./vigil-server"]