# Go Crawler - Makefile
# =====================

.PHONY: build run test clean docker-build docker-up docker-down help

# Variables
DOCKER_COMPOSE = docker compose
GO = go
DOCKER = docker
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME = $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT = $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# ==================== BUILD ====================

## Build all binaries
build:
	@echo "🔨 Building all services..."
	$(GO) build ./...

## Build specific service
build-crawler:
	$(GO) build -o bin/vl24h-crawler ./cmd/vl24h-crawler

build-enricher:
	$(GO) build -o bin/vl24h-enricher ./cmd/vl24h-enricher

build-worker:
	$(GO) build -o bin/worker ./cmd/worker

# ==================== DOCKER ====================

## Build optimized Docker image
docker-build:
	@echo "🐳 Building optimized Docker image..."
	@echo "Version: $(VERSION)"
	@echo "Commit: $(GIT_COMMIT)"
	DOCKER_BUILDKIT=1 $(DOCKER) build \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--tag go-crawler:$(VERSION) \
		--tag go-crawler:latest \
		.

## Build with cache busting (slower, ensures fresh build)
docker-build-nocache:
	@echo "🐳 Building Docker image (no cache)..."
	DOCKER_BUILDKIT=1 $(DOCKER) build \
		--no-cache \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--tag go-crawler:$(VERSION) \
		--tag go-crawler:latest \
		.

## Start all services with docker-compose
up:
	@echo "🚀 Starting services..."
	$(DOCKER_COMPOSE) up -d --build

## Stop all services
down:
	@echo "🛑 Stopping services..."
	$(DOCKER_COMPOSE) down

## Stop and remove volumes (clean reset)
reset:
	@echo "🧹 Resetting all data..."
	$(DOCKER_COMPOSE) down -v
	@echo "🚀 Starting fresh..."
	$(DOCKER_COMPOSE) up -d --build

## View logs
logs:
	$(DOCKER_COMPOSE) logs -f

logs-crawler:
	$(DOCKER_COMPOSE) logs -f vl24h-crawler

logs-enricher:
	$(DOCKER_COMPOSE) logs -f vl24h-enricher

logs-worker:
	$(DOCKER_COMPOSE) logs -f vl24h-worker

## Check service status
status:
	$(DOCKER_COMPOSE) ps


# ==================== ELASTICSEARCH ====================

## Show job count in Elasticsearch
count:
	@curl -s 'http://localhost:9200/jobs_vieclam24h/_count' | jq '.count'

## Sample data from Elasticsearch
sample:
	@curl -s 'http://localhost:9200/jobs_vieclam24h/_search?size=5&pretty' | jq '.hits.hits[]._source | {title, company, salary, location_city}'

## Check Elasticsearch health
es-health:
	@curl -s 'http://localhost:9200/_cluster/health?pretty'

## View index mapping
es-mapping:
	@curl -s 'http://localhost:9200/jobs_vieclam24h/_mapping?pretty'

# ==================== REDIS ====================

## Connect to Redis CLI
redis:
	docker exec -it redis-crawler redis-cli

## Check queue sizes
queues:
	@echo "📋 Queue Status:"
	@docker exec redis-crawler redis-cli LLEN jobs:pending:vieclam24h
	@docker exec redis-crawler redis-cli LLEN jobs:raw:vieclam24h

# ==================== TEST ====================

## Run tests
test:
	$(GO) test ./...

## Run tests with coverage
test-cover:
	$(GO) test -cover ./...

# ==================== CLEAN ====================

## Clean build artifacts
clean:
	@echo "🧹 Cleaning..."
	rm -rf bin/
	$(GO) clean

# ==================== HELP ====================

## Show this help
help:
	@echo "Go Crawler - Available Commands"
	@echo "================================"
	@echo ""
	@echo "Build:"
	@echo "  make build              - Build all services"
	@echo "  make build-crawler      - Build crawler only"
	@echo "  make build-enricher     - Build enricher only"
	@echo "  make build-worker       - Build worker only"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-build       - Build optimized Docker image"
	@echo "  make docker-build-nocache - Build without cache"
	@echo "  make up                 - Start all services"
	@echo "  make down               - Stop all services"
	@echo "  make reset              - Stop, clean volumes, restart"
	@echo "  make logs               - View all logs"
	@echo "  make logs-crawler       - View crawler logs"
	@echo "  make status             - Check service status"
	@echo ""
	@echo "Elasticsearch:"
	@echo "  make count              - Show job count"
	@echo "  make sample             - Show sample data"
	@echo "  make es-health          - Check ES cluster health"
	@echo "  make es-mapping         - View index mapping"
	@echo ""
	@echo "Redis:"
	@echo "  make redis              - Connect to Redis CLI"
	@echo "  make queues             - Check queue sizes"
	@echo ""
	@echo "Test:"
	@echo "  make test               - Run tests"
	@echo "  make test-cover         - Run tests with coverage"
	@echo ""
	@echo "Environment:"
	@echo "  VERSION: $(VERSION)"
	@echo "  GIT_COMMIT: $(GIT_COMMIT)"
