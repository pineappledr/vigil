# Build Stage - Alpine with CGO support for SQLite
FROM golang:1.25-alpine AS builder
WORKDIR /app

# Version build arg (set by CI or defaults to dev)
ARG VERSION=dev

# Install build dependencies (gcc, musl-dev needed for CGO/SQLite)
RUN apk add --no-cache gcc musl-dev

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the server binary with version
# CGO_ENABLED=1 is required for modernc.org/sqlite
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o vigil-server ./cmd/server

# Final Stage - Minimal Alpine runtime
FROM alpine:3.19
WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates wget tzdata

# Create non-root user and data directory
RUN adduser -D -u 1000 vigil && \
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