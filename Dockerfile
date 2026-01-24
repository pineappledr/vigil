# Stage 1: Build
FROM golang:1.25-alpine AS builder
WORKDIR /src

# Install build tools
RUN apk add --no-cache gcc musl-dev

# Copy dependency locks
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the Server (Static binary)
RUN CGO_ENABLED=0 GOOS=linux go build -o vigil-server ./cmd/server

# Stage 2: Runtime
FROM alpine:latest
WORKDIR /app

# Copy binary from builder
COPY --from=builder /src/vigil-server .

# Setup Data Directory
RUN mkdir /data
VOLUME ["/data"]

# Configure App
ENV PORT=8090
ENV DB_PATH=/data/vigil.db

EXPOSE 8090
CMD ["./vigil-server"]