# ==========================================
# Kalyx Server: Multi-Stage Containerfile 🌸
# Zero-Dependency Distroless Runtime
# ==========================================

# Stage 1: Build static standalone binary
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Install basic root CA certificates and timezone data
RUN apk --no-cache add ca-certificates tzdata

# Fetch dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy application source code
COPY . .

# Prepare data directory owned by nonroot user (UID 65532)
RUN mkdir -p /build/data && chown -R 65532:65532 /build/data

# Build static binary without CGO
ARG VERSION=0.1.0
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /build/bin/kalyx ./cmd/kalyx

# Stage 2: Minimal Distroless runtime
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /

# Copy certificates and compiled binary
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /build/bin/kalyx /kalyx
COPY --from=builder --chown=65532:65532 /build/data /data

# Default environment configuration
ENV PORT=8080 \
    KALYX_SQLITE_PATH=/data/kalyx.db \
    GIN_MODE=release

# Expose HTTP port
EXPOSE 8080

# Run as nonroot user (UID 65532)
USER 65532:65532

VOLUME ["/data"]

ENTRYPOINT ["/kalyx"]
