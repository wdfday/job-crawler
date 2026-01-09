# =============================================================================
# Builder Stage - Build all binaries sequentially
# =============================================================================
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Layer 1: Dependencies (cached unless go.mod/go.sum changes)
COPY go.mod go.sum ./
RUN go mod download

# Layer 2: Source code
COPY . .

# Layer 3: Build ALL binaries SEQUENTIALLY (no parallel, no mount cache)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -trimpath -o /bin/vl24h-crawler ./cmd/vieclam24h/crawler && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -trimpath -o /bin/vl24h-enricher ./cmd/vieclam24h/enricher && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -trimpath -o /bin/worker ./cmd/worker && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -trimpath -o /bin/vietnamworks ./cmd/vietnamworks

# =============================================================================
# Runtime Targets - One per service
# =============================================================================

# Vieclam24h Crawler Runtime
FROM alpine:3.20 AS vl24h-crawler

RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 1000 appuser

WORKDIR /app

COPY --from=builder --chown=appuser:appuser /bin/vl24h-crawler ./vl24h-crawler

ENV TZ=Asia/Ho_Chi_Minh

USER appuser

CMD ["/app/vl24h-crawler"]

# Vieclam24h Enricher Runtime
FROM alpine:3.20 AS vl24h-enricher

RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 1000 appuser

WORKDIR /app

COPY --from=builder --chown=appuser:appuser /bin/vl24h-enricher ./vl24h-enricher

ENV TZ=Asia/Ho_Chi_Minh

USER appuser

CMD ["/app/vl24h-enricher"]

# Worker Runtime
FROM alpine:3.20 AS worker

RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 1000 appuser

WORKDIR /app

COPY --from=builder --chown=appuser:appuser /bin/worker ./worker

ENV TZ=Asia/Ho_Chi_Minh

USER appuser

CMD ["/app/worker"]

## VietnamWorks Crawler Runtime
#FROM alpine:3.20 AS vietnamworks
#
#RUN apk add --no-cache ca-certificates tzdata && \
#    adduser -D -u 1000 appuser
#
#WORKDIR /app
#
#COPY --from=builder --chown=appuser:appuser /bin/vietnamworks ./vietnamworks
#
#ENV TZ=Asia/Ho_Chi_Minh
#
#USER appuser
#
#CMD ["/app/vietnamworks"]
