# ============================================================================
# Job Crawler - Justfile
# ============================================================================

set shell := ["powershell", "-Command"]

default: help

# Help
help:
    @Write-Host "📦 Job Crawler - Justfile"
    @Write-Host ""
    @Write-Host "🔨 Build:"
    @Write-Host "  just build              - Build all services"
    @Write-Host "  just build-crawler      - Build crawler only"
    @Write-Host "  just build-enricher     - Build enricher only"
    @Write-Host "  just build-worker       - Build worker only"
    @Write-Host "  just rebuild            - Rebuild without cache"
    @Write-Host "  just rebuild-clean      - Clean rebuild (down -v + build + up)"
    @Write-Host ""
    @Write-Host "🚀 Services:"
    @Write-Host "  just up                 - Start all services"
    @Write-Host "  just down               - Stop services"
    @Write-Host "  just down-volumes       - Stop and remove volumes"
    @Write-Host "  just restart            - Restart services"
    @Write-Host "  just ps                 - Show containers"
    @Write-Host ""
    @Write-Host "📋 Logs:"
    @Write-Host "  just logs               - All logs (follow)"
    @Write-Host "  just logs-worker        - Worker logs"
    @Write-Host "  just logs-crawler       - Crawler logs"
    @Write-Host "  just logs-enricher      - Enricher logs"
    @Write-Host ""
    @Write-Host "🔍 Debug:"
    @Write-Host "  just stats              - Queue & ES stats"
    @Write-Host "  just redis              - Redis CLI"
    @Write-Host "  just shell              - Shell into worker"
    @Write-Host ""
    @Write-Host "🧹 Cleanup:"
    @Write-Host "  just clean              - Stop and remove volumes"
    @Write-Host "  just clean-all          - Deep clean (images, cache)"

# ============================================================================
# Build
# ============================================================================

# Build all services
build:
    $env:DOCKER_BUILDKIT=1; docker compose build

# Build crawler only
build-crawler:
    $env:DOCKER_BUILDKIT=1; docker compose build vl24h-crawler

# Build enricher only
build-enricher:
    $env:DOCKER_BUILDKIT=1; docker compose build vl24h-enricher

# Build worker only
build-worker:
    $env:DOCKER_BUILDKIT=1; docker compose build vl24h-worker

# Rebuild without cache
rebuild:
    $env:DOCKER_BUILDKIT=1; docker compose build --no-cache

# Clean rebuild (down -v + build + up)
rebuild-clean:
    docker compose down -v
    $env:DOCKER_BUILDKIT=1; docker compose build --no-cache
    docker compose up -d
    Start-Sleep -Seconds 2
    just stats

# ============================================================================
# Services
# ============================================================================

# Start all services
up:
    docker compose up -d
    Start-Sleep -Seconds 2
    just stats

# Stop services
down:
    docker compose down

# Stop and remove volumes
down-volumes:
    docker compose down -v

# Restart services
restart:
    docker compose restart

# Show containers
ps:
    docker compose ps

# ============================================================================
# Logs
# ============================================================================

# All logs (follow)
logs:
    docker compose logs -f

# Worker logs
logs-worker:
    docker compose logs -f vl24h-worker

# Crawler logs
logs-crawler:
    docker compose logs -f vl24h-crawler

# Enricher logs
logs-enricher:
    docker compose logs -f vl24h-enricher

# ============================================================================
# Debug
# ============================================================================

# Queue & ES stats
stats:
    @Write-Host "📊 Stats:"
    @Write-Host ""
    @Write-Host "Queues:"
    @try { $p = docker exec redis-crawler redis-cli LLEN jobs:pending:vieclam24h; Write-Host "  Pending: $p" } catch { Write-Host "  Redis not ready" }
    @try { $r = docker exec redis-crawler redis-cli LLEN jobs:raw:vieclam24h; Write-Host "  Raw:     $r" } catch { }
    @Write-Host ""
    @Write-Host "Dedup keys:"
    @try { $d = docker exec redis-crawler redis-cli DBSIZE; Write-Host "  Total:   $d" } catch { }
    @Write-Host ""
    @Write-Host "Elasticsearch:"
    @try { $c = (Invoke-RestMethod "http://localhost:9200/jobs_vieclam24h/_count" -ErrorAction Stop).count; Write-Host "  Jobs:    $c" } catch { Write-Host "  Not ready" }
    @Write-Host ""
    @docker compose ps

# Redis CLI
redis:
    docker exec -it redis-crawler redis-cli

# Shell into worker
shell:
    docker exec -it vl24h-worker sh

# ============================================================================
# Cleanup
# ============================================================================

# Stop and remove volumes
clean:
    docker compose down -v

# Deep clean (images, cache)
clean-all:
    docker compose down -v --rmi all
    docker system prune -af --volumes
