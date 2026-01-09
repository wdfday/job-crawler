# ============================================================================
# Job Crawler - Makefile
# ============================================================================

.PHONY: help build build-crawler build-enricher build-worker rebuild rebuild-clean \
        up down down-volumes restart ps \
        logs logs-worker logs-crawler logs-enricher \
        stats redis shell clean clean-all

.DEFAULT_GOAL := help

# ============================================================================
# Help
# ============================================================================
help:
	@echo "📦 Job Crawler - Makefile"
	@echo ""
	@echo "🔨 Build:"
	@echo "  make build              - Build all services"
	@echo "  make build-crawler      - Build crawler only"
	@echo "  make build-enricher     - Build enricher only"
	@echo "  make build-worker       - Build worker only"
	@echo "  make rebuild            - Rebuild without cache"
	@echo "  make rebuild-clean      - Clean rebuild (down -v + build + up)"
	@echo ""
	@echo "🚀 Services:"
	@echo "  make up                 - Start all services"
	@echo "  make down               - Stop services"
	@echo "  make down-volumes       - Stop and remove volumes"
	@echo "  make restart            - Restart services"
	@echo "  make ps                 - Show containers"
	@echo ""
	@echo "📋 Logs:"
	@echo "  make logs               - All logs (follow)"
	@echo "  make logs-worker        - Worker logs"
	@echo "  make logs-crawler       - Crawler logs"
	@echo "  make logs-enricher      - Enricher logs"
	@echo ""
	@echo "🔍 Debug:"
	@echo "  make stats              - Queue & ES stats"
	@echo "  make redis              - Redis CLI"
	@echo "  make shell              - Shell into worker"
	@echo ""
	@echo "🧹 Cleanup:"
	@echo "  make clean              - Stop and remove volumes"
	@echo "  make clean-all          - Deep clean (images, cache)"

# ============================================================================
# Build
# ============================================================================
build:
	@DOCKER_BUILDKIT=1 docker compose build

build-crawler:
	@DOCKER_BUILDKIT=1 docker compose build vl24h-crawler

build-enricher:
	@DOCKER_BUILDKIT=1 docker compose build vl24h-enricher

build-worker:
	@DOCKER_BUILDKIT=1 docker compose build vl24h-worker

rebuild:
	@DOCKER_BUILDKIT=1 docker compose build --no-cache

rebuild-clean:
	@docker compose down -v
	@DOCKER_BUILDKIT=1 docker compose build --no-cache
	@docker compose up -d
	@sleep 2
	@make stats

# ============================================================================
# Services
# ============================================================================
up:
	@docker compose up -d
	@sleep 2
	@make stats

down:
	@docker compose down

down-volumes:
	@docker compose down -v

restart:
	@docker compose restart

ps:
	@docker compose ps

# ============================================================================
# Logs
# ============================================================================
logs:
	@docker compose logs -f

logs-worker:
	@docker compose logs -f vl24h-worker

logs-crawler:
	@docker compose logs -f vl24h-crawler

logs-enricher:
	@docker compose logs -f vl24h-enricher

# ============================================================================
# Debug
# ============================================================================
stats:
	@echo "📊 Stats:"
	@echo ""
	@echo "Queues:"
	@docker exec redis-crawler redis-cli LLEN jobs:pending:vieclam24h 2>/dev/null | awk '{print "  Pending: " $$1}' || echo "  Redis not ready"
	@docker exec redis-crawler redis-cli LLEN jobs:raw:vieclam24h 2>/dev/null | awk '{print "  Raw:     " $$1}' || echo ""
	@echo ""
	@echo "Dedup keys:"
	@docker exec redis-crawler redis-cli DBSIZE 2>/dev/null | awk '{print "  Total:   " $$2}' || echo ""
	@echo ""
	@echo "Elasticsearch:"
	@curl -s http://localhost:9200/jobs_vieclam24h/_count 2>/dev/null | grep -o '"count":[0-9]*' | cut -d: -f2 | awk '{print "  Jobs:    " $$1}' || echo "  Not ready"
	@echo ""
	@docker compose ps

redis:
	@docker exec -it redis-crawler redis-cli

shell:
	@docker exec -it vl24h-worker sh

# ============================================================================
# Cleanup
# ============================================================================
clean:
	@docker compose down -v

clean-all:
	@docker compose down -v --rmi all
	@docker system prune -af --volumes
