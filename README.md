# Job Crawler

Hệ thống thu thập và xử lý dữ liệu việc làm từ các trang tuyển dụng Việt Nam.

## Tính năng

- **Deduplication**: Tự động phát hiện và bỏ qua jobs không thay đổi (dựa trên `updated_at`)
- **Normalization**: Chuẩn hóa dữ liệu từ nhiều nguồn về format thống nhất
- **Vietnamese Search**: Full-text search với Vietnamese analyzer
- **Rate Limiting**: Tự động delay giữa requests để tránh bị block

## Kiến trúc

```
┌─────────────┐      ┌─────────────┐      ┌─────────────┐      ┌──────────────┐
│   Crawler   │ ──▶  │   Enricher  │ ──▶  │    Worker   │ ──▶  │Elasticsearch │
│  (API Fetch)│      │(HTML Scrape)│      │ (Normalize) │      │   (Index)    │
└─────────────┘      └─────────────┘      └─────────────┘      └──────────────┘
       │                    │                    │
       └────────────────────┴────────────────────┘
                           │
                    ┌──────▼──────┐
                    │    Redis    │
                    │ (Queue+Dedup)│
                    └─────────────┘
```

## Quick Start

```bash
# Build và chạy
make build
make up

# Xem logs
make logs           # Tất cả services
make logs-crawler   # Crawler only
make logs-worker    # Worker only

# Kiểm tra trạng thái
make stats          # Queue length, ES docs count
make ps             # Container status

# Dừng
make down           # Stop containers
make clean          # Stop + xóa data
```

## Cấu hình

| Biến môi trường | Mặc định | Mô tả |
|-----------------|----------|-------|
| `REDIS_ADDR` | `redis:6379` | Redis connection |
| `ELASTICSEARCH_URL` | `http://elasticsearch:9200` | Elasticsearch URL |
| `ELASTICSEARCH_INDEX` | `jobs_vieclam24h` | Tên index |
| `CRAWLER_DELAY_MS` | `2000` | Delay giữa requests (ms) |
| `WORKER_CONCURRENCY` | `5` | Số goroutines xử lý đồng thời |
| `WORKER_BATCH_SIZE` | `100` | Số jobs mỗi batch |

## Cấu trúc thư mục

```
cmd/
├── vieclam24h/
│   ├── crawler/     # Stage 1: Fetch từ API
│   └── enricher/    # Stage 2: Scrape HTML detail
├── vietnamworks/    # VietnamWorks crawler
└── worker/          # Stage 3: Normalize + Index

internal/
├── module/          # Crawler implementations
│   ├── vieclam24h/  # Vieclam24h crawler + scraper
│   ├── vietnamworks/
│   ├── topcv/
│   ├── careerviet/
│   ├── topdev/
│   └── worker/      # Worker implementation
├── common/
│   ├── dedup/       # Redis deduplication
│   ├── queue/       # Publisher/Consumer
│   ├── indexer/     # Elasticsearch indexer
│   ├── normalizer/  # Data normalization
│   └── cleaner/     # HTML cleaning
├── domain/          # Data models (Job, RawJob)
├── queue/           # Redis queue
└── config/          # Environment config
```

## Data Models

### RawJob (trong queue)

```go
{
  "id": "200734388",
  "url": "https://vieclam24h.vn/...",
  "source": "vieclam24h",
  "raw_data": {...},           // Dữ liệu thô từ API/HTML
  "last_updated_on": "...",    // Cho dedup check
  "expired_on": "2024-12-31"
}
```

### Job (trong Elasticsearch)

- `title`, `company`, `description`, `requirements`, `benefits`
- `location_city[]`, `location_district[]`
- `salary_min`, `salary_max` (triệu VND), `is_negotiable`
- `experience_tags[]` (A/B/C/D/E/F)
- `skills[]`, `industry[]`

## Debug

```bash
# Xem queue
make redis
LLEN jobs:pending:vieclam24h
LLEN jobs:raw:vieclam24h

# Xem dedup keys
KEYS "job:seen:*"

# Test Elasticsearch
curl localhost:9200/jobs_vieclam24h/_count
curl localhost:9200/jobs_vieclam24h/_search?q=developer
```

## Tài liệu chi tiết

- [vieclam24h.md](./vieclam24h.md) - Chi tiết pipeline Vieclam24h
- [CLAUDE.md](./CLAUDE.md) - Hướng dẫn cho AI assistant

## License

MIT License - Copyright (c) 2026 Project TKTT
