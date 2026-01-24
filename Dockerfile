# Stage 1: Build
FROM golang:1.25-alpine AS builder
WORKDIR /src

# Install build tools
RUN apk add --no-cache gcc musl-dev

# --- FIX START ---
# Only copy go.mod first (ignore missing go.sum)
COPY go.mod ./

# Generate go.sum automatically inside the cloud builder
RUN go mod tidy
RUN go mod download
# --- FIX END ---

# Copy the rest of the code
COPY . .

# Build the Server
RUN CGO_ENABLED=0 GOOS=linux go build -o vigil-server ./cmd/server

# Stage 2: Runtime
FROM alpine:latest
WORKDIR /app

# Copy binary
COPY --from=builder /src/vigil-server .

# Setup Data Directory
RUN mkdir /data
VOLUME ["/data"]

# Configure App
ENV PORT=8090
ENV DB_PATH=/data/vigil.db

EXPOSE 8090
CMD ["./vigil-server"]