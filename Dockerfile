# =============================================================================
# Build Stage - Compile Go binaries with optimization
# =============================================================================
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    make \
    upx

WORKDIR /build

# Copy dependency files first (better layer caching)
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download && go mod verify

# Copy source code
COPY . .

# Build arguments for optimization
ARG VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT=unknown

# Build flags for smaller, faster binaries
ARG LDFLAGS="-s -w \
    -X main.Version=${VERSION} \
    -X main.BuildTime=${BUILD_TIME} \
    -X main.GitCommit=${GIT_COMMIT}"

# Build all binaries with optimization
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="${LDFLAGS}" -trimpath -a -o /bin/vl24h-crawler ./cmd/vieclam24h/crawler && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="${LDFLAGS}" -trimpath -a -o /bin/vl24h-enricher ./cmd/vieclam24h/enricher && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="${LDFLAGS}" -trimpath -a -o /bin/crawler-vietnamworks ./cmd/vietnamworks && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="${LDFLAGS}" -trimpath -a -o /bin/worker ./cmd/worker

# Compress binaries with UPX (optional, comment out if issues)
RUN upx --best --lzma /bin/vl24h-crawler /bin/vl24h-enricher /bin/crawler-vietnamworks /bin/worker || true

# Verify binaries
RUN /bin/vl24h-crawler --version 2>/dev/null || echo "Crawler built" && \
    /bin/worker --version 2>/dev/null || echo "Worker built"

# =============================================================================
# Runtime Stage - Minimal production image
# =============================================================================
FROM alpine:3.19

# Metadata
LABEL maintainer="Project TKTT"
LABEL description="Job crawler system for Vietnamese job sites"
LABEL version="${VERSION}"

# Install runtime dependencies and create non-root user
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    dumb-init && \
    addgroup -g 1000 -S crawler && \
    adduser -u 1000 -S crawler -G crawler && \
    mkdir -p /app /tmp/crawler && \
    chown -R crawler:crawler /app /tmp/crawler

WORKDIR /app

# Copy binaries from builder
COPY --from=builder --chown=crawler:crawler /bin/vl24h-crawler ./vl24h-crawler
COPY --from=builder --chown=crawler:crawler /bin/vl24h-enricher ./vl24h-enricher
COPY --from=builder --chown=crawler:crawler /bin/crawler-vietnamworks ./crawler-vietnamworks
COPY --from=builder --chown=crawler:crawler /bin/worker ./worker

# Environment variables
ENV TZ=Asia/Ho_Chi_Minh \
    PATH="/app:${PATH}" \
    TMPDIR=/tmp/crawler

# Switch to non-root user
USER crawler

# Health check (override in docker-compose for specific services)
# Uncomment and customize based on service needs
# HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
#     CMD ["/app/worker", "--health"]

# Default command (overridden in docker-compose)
ENTRYPOINT ["/usr/bin/dumb-init", "--"]
CMD ["/app/worker"]