# Build Stage
FROM golang:1.25-alpine AS builder
WORKDIR /app

# Install git for any dependencies that need it
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the server binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o vigil-server ./cmd/server

# Final Stage
FROM alpine:3.19
WORKDIR /app

# Add ca-certificates for HTTPS and tzdata for timezone support
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN adduser -D -u 1000 vigil
USER vigil

# Copy the binary
COPY --from=builder /app/vigil-server .

# Copy the web folder
COPY --from=builder /app/web ./web

EXPOSE 8090

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8090/health || exit 1

CMD ["./vigil-server"]
