# Base stage for building
FROM golang:1.25-alpine AS builder
WORKDIR /app
# Install build dependencies
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build both binaries
RUN go build -o /app/bin/api ./cmd/api/main.go
RUN go build -o /app/bin/worker ./cmd/worker/main.go

# Final stage
FROM alpine:latest
WORKDIR /app

# IMPORTANT: Install FFmpeg in the runtime container
RUN apk add --no-cache ffmpeg ca-certificates

# Copy binaries from builder
COPY --from=builder /app/bin/api /app/api
COPY --from=builder /app/bin/worker /app/worker

# We don't set an ENTRYPOINT here because we will define it in docker-compose