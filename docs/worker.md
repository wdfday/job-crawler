# Vieclam24h Worker

**Stage 3** trong pipeline - Normalize dữ liệu và index vào Elasticsearch.

---

## 1. Tổng quan

### 1.1 Chức năng chính

- **Queue Consumption**: Batch consume từ queue `jobs:raw:vieclam24h`
- **HTML Cleaning**: Strip tags, normalize Unicode
- **Normalization**: Chuyển đổi về format `domain.Job`
- **Bulk Indexing**: Index vào Elasticsearch

### 1.2 Vị trí trong Pipeline

```mermaid
flowchart LR
    subgraph Stage2["🔧 Stage 2"]
        Enricher["Enricher"]
    end
    
    subgraph Redis["💾 Redis"]
        Queue[("jobs:raw:vieclam24h")]
    end
    
    subgraph Stage3["📊 Stage 3: Worker"]
        Worker["Worker"]
        Cleaner["HTML Cleaner"]
        Normalizer["Normalizer"]
    end
    
    subgraph ES["🔍 Elasticsearch"]
        Index[("jobs_vieclam24h")]
    end
    
    Enricher -->|LPUSH| Queue
    Queue -->|Batch RPOP| Worker
    Worker --> Cleaner --> Normalizer
    Normalizer -->|Bulk Index| Index
    
    style Stage3 fill:#e8f5e9,stroke:#43a047
```

---

## 2. Kiến trúc

```mermaid
flowchart TB
    subgraph Worker["Worker Service"]
        Main["main.go"]
        WorkerMod["worker.Worker"]
        Consumer["queue.Consumer"]
        Cleaner["cleaner.Cleaner"]
        Normalizer["normalizer.Normalizer"]
        Indexer["indexer.ElasticsearchIndexer"]
    end
    
    subgraph External["External"]
        Redis[("Redis")]
        ES[("Elasticsearch")]
    end
    
    Main --> WorkerMod
    WorkerMod --> Consumer
    WorkerMod --> Cleaner
    WorkerMod --> Normalizer
    WorkerMod --> Indexer
    
    Consumer -->|Batch RPOP| Redis
    Indexer -->|Bulk API| ES
    
    style Worker fill:#e8f5e9
```

### 2.1 Class Structure

```mermaid
classDiagram
    class Worker {
        -consumer *Consumer
        -normalizer *Normalizer
        -cleaner *Cleaner
        -indexer Indexer
        -batchSize int
        -concurrency int
        +Run(ctx) error
        -runSingle(ctx, workerID) error
        -processJobs(rawJobs) []*Job
    }
    
    class Config {
        +Concurrency int
        +BatchSize int
    }
    
    class Normalizer {
        +Normalize(raw) *Job, error
    }
    
    class Cleaner {
        +CleanToText(html) string
        +CleanMap(data) map
    }
    
    class Indexer {
        +BulkIndex(ctx, jobs) error
        +EnsureIndex(ctx) error
    }
    
    Worker --> Config
    Worker --> Normalizer
    Worker --> Cleaner
    Worker --> Indexer
```

---

## 3. Input

### 3.1 Queue Source

| Property | Value |
|----------|-------|
| Queue Name | `jobs:raw:vieclam24h` |
| Consume Method | Batch RPOP (Lua script) |
| Batch Size | 100 (configurable) |

### 3.2 Enriched RawJob

```json
{
  "id": "200734388",
  "url": "https://vieclam24h.vn/...",
  "source": "vieclam24h",
  "raw_data": {
    "jobId": 200734388,
    "jobTitle": "Kỹ Thuật Viên Lắp Đặt",
    "companyName": "Công Ty ABC",
    "salaryFrom": 8000000,
    "salaryTo": 15000000,
    "salaryMinJsonLd": 8000000,
    "locationCity": ["Hà Nội", "TP.HCM"],
    "experienceText": "1 năm",
    "jobDescription": "Mô tả công việc...",
    "skills": "PLC, SCADA, AutoCAD",
    "industry": ["Điện - Điện tử", "Cơ khí"]
  }
}
```

---

## 4. Luồng xử lý

```mermaid
flowchart TD
    Start([Start]) --> InitPool["Start worker pool\n(N goroutines)"]
    
    InitPool --> Consume["ConsumeBatch\n(Lua script RPOP)"]
    
    Consume --> HasJobs{Jobs found?}
    HasJobs -->|No| Consume
    HasJobs -->|Yes| ProcessBatch["Process batch"]
    
    subgraph ProcessJob["For each RawJob (parallel)"]
        direction TB
        P1["Clean RawData map"]
        P2["Normalize → domain.Job"]
        P3["Clean text fields"]
        P1 --> P2 --> P3
    end
    
    ProcessBatch --> ProcessJob
    ProcessJob --> Collect["Collect normalized jobs"]
    
    Collect --> BulkIndex["Bulk Index to ES"]
    
    BulkIndex --> IndexOK{Success?}
    IndexOK -->|Yes| LogSuccess["Log: indexed N jobs"]
    IndexOK -->|No| LogError["Log error"]
    
    LogSuccess --> Consume
    LogError --> Consume
    
    style ProcessJob fill:#c8e6c9
    style BulkIndex fill:#bbdefb
```

### 4.1 Parallel Processing

```mermaid
flowchart LR
    subgraph Batch["Batch (100 jobs)"]
        J1[Job 1]
        J2[Job 2]
        J3[Job 3]
        JN[Job N]
    end
    
    subgraph Workers["Goroutines"]
        W1[Worker 1]
        W2[Worker 2]
        W3[Worker 3]
        WN[Worker N]
    end
    
    J1 --> W1
    J2 --> W2
    J3 --> W3
    JN --> WN
    
    subgraph Results["Normalized"]
        R1[Job 1]
        R2[Job 2]
        R3[Job 3]
        RN[Job N]
    end
    
    W1 --> R1
    W2 --> R2
    W3 --> R3
    WN --> RN
    
    Results --> BulkIndex["Bulk Index"]
```

---

## 5. Normalization

### 5.1 Field Mapping

```mermaid
flowchart LR
    subgraph RawData["RawData (Enriched)"]
        R1["jobTitle"]
        R2["companyName"]
        R3["salaryMinJsonLd / salaryFrom"]
        R4["locationCity[]"]
        R5["experienceText / experienceRange"]
        R6["skills (string)"]
        R7["industry[]"]
    end
    
    subgraph Job["domain.Job"]
        J1["Title"]
        J2["Company"]
        J3["SalaryMin / SalaryMax"]
        J4["LocationCity[]"]
        J5["Experience / ExpTags[]"]
        J6["Skills[]"]
        J7["Industry[]"]
    end
    
    R1 --> J1
    R2 --> J2
    R3 --> J3
    R4 --> J4
    R5 --> J5
    R6 -->|split| J6
    R7 --> J7
```

### 5.2 Salary Conversion

```mermaid
flowchart TD
    subgraph Input["Input (VND)"]
        I1["salaryMinJsonLd: 8000000"]
        I2["salaryMaxJsonLd: 15000000"]
        I3["salaryText: 'Thỏa thuận'"]
    end
    
    I1 --> Convert["÷ 1,000,000"]
    I2 --> Convert
    
    Convert --> Output1["salary_min: 8\nsalary_max: 15"]
    
    I3 --> CheckNeg{"== 'Thỏa thuận'?"}
    CheckNeg -->|Yes| Output2["is_negotiable: true\nsalary_min/max: 0"]
    CheckNeg -->|No| Output1
    
    style Output1 fill:#c8e6c9
    style Output2 fill:#fff9c4
```

**Priority:** `salaryMinJsonLd` > `salaryFrom`

### 5.3 Experience Tags

```mermaid
flowchart LR
    subgraph Input["experienceText"]
        E1["0 năm"]
        E2["1 năm"]
        E3["2 năm"]
        E4["3-5 năm"]
        E5["> 5 năm"]
        E6["> 10 năm"]
    end
    
    subgraph Output["experience_tags[]"]
        T1["A, B, C, D, E, F"]
        T2["B, C, D, E, F"]
        T3["C, D, E, F"]
        T4["D, E, F"]
        T5["E, F"]
        T6["F"]
    end
    
    E1 --> T1
    E2 --> T2
    E3 --> T3
    E4 --> T4
    E5 --> T5
    E6 --> T6
```

| Tag | Meaning |
|-----|---------|
| A | Phù hợp 0-1 năm |
| B | Phù hợp 1-2 năm |
| C | Phù hợp 2-3 năm |
| D | Phù hợp 3-5 năm |
| E | Phù hợp 5-10 năm |
| F | Phù hợp 10+ năm |

### 5.4 Skills Parsing

```
Input:  "PLC, SCADA, AutoCAD"
Output: ["PLC", "SCADA", "AutoCAD"]
```

### 5.5 HTML Cleaning

```go
job.Description = cleaner.CleanToText(job.Description)
job.Requirements = cleaner.CleanToText(job.Requirements)
job.Benefits = cleaner.CleanToText(job.Benefits)
```

- Strip HTML tags
- Normalize Unicode
- Trim whitespace
- Remove empty lines

---

## 6. Output

### 6.1 Elasticsearch Index

| Property | Value |
|----------|-------|
| Index Name | `jobs_vieclam24h` |
| Method | Bulk API |
| Analyzer | Vietnamese (lowercase + asciifolding) |

### 6.2 Document Format

```json
{
  "id": "200734388",
  "source": "vieclam24h",
  "source_url": "https://vieclam24h.vn/...",
  "title": "Kỹ Thuật Viên Lắp Đặt",
  "company": "Công Ty ABC",
  "location": "Nam Từ Liêm, Hà Nội",
  "location_city": ["Hà Nội", "TP.HCM"],
  "location_district": ["Nam Từ Liêm", "Quận 1"],
  "salary": "8 - 15 triệu",
  "salary_min": 8,
  "salary_max": 15,
  "is_negotiable": false,
  "experience": "1 năm",
  "experience_tags": ["B", "C", "D", "E", "F"],
  "industry": ["Điện - Điện tử", "Cơ khí"],
  "skills": ["PLC", "SCADA", "AutoCAD"],
  "description": "Mô tả công việc (plain text)...",
  "requirements": "Yêu cầu (plain text)...",
  "benefits": "Quyền lợi (plain text)...",
  "total_views": 150,
  "total_resume_applied": 20,
  "rate_response": 95,
  "expired_at": "2025-02-04T23:59:59Z",
  "crawled_at": "2025-01-09T19:00:00Z"
}
```

### 6.3 Elasticsearch Mapping

```json
{
  "mappings": {
    "properties": {
      "id": {"type": "keyword"},
      "source": {"type": "keyword"},
      "title": {"type": "text", "analyzer": "vietnamese"},
      "company": {"type": "text", "analyzer": "vietnamese"},
      "description": {"type": "text", "analyzer": "vietnamese"},
      "location_city": {"type": "keyword"},
      "location_district": {"type": "keyword"},
      "salary_min": {"type": "integer"},
      "salary_max": {"type": "integer"},
      "is_negotiable": {"type": "boolean"},
      "experience_tags": {"type": "keyword"},
      "skills": {"type": "keyword"},
      "industry": {"type": "keyword"},
      "expired_at": {"type": "date"},
      "crawled_at": {"type": "date"}
    }
  }
}
```

---

## 7. Cấu hình

| Config | Default | Mô tả |
|--------|---------|-------|
| `Concurrency` | 5 | Số worker goroutines |
| `BatchSize` | 100 | Số jobs mỗi batch |

### Environment Variables

| Variable | Default |
|----------|---------|
| `REDIS_ADDR` | `redis:6379` |
| `REDIS_JOB_QUEUE` | `jobs:raw:vieclam24h` |
| `ELASTICSEARCH_URL` | `http://elasticsearch:9200` |
| `ELASTICSEARCH_INDEX` | `jobs_vieclam24h` |
| `WORKER_CONCURRENCY` | `5` |
| `WORKER_BATCH_SIZE` | `100` |

---

## 8. Sample Queries

### Count documents

```bash
curl localhost:9200/jobs_vieclam24h/_count
```

### Search by keyword

```bash
curl localhost:9200/jobs_vieclam24h/_search?q=developer
```

### Filter by city

```bash
curl -X POST localhost:9200/jobs_vieclam24h/_search \
  -H 'Content-Type: application/json' -d '
{
  "query": {"term": {"location_city": "Hà Nội"}}
}'
```

### Filter by salary

```bash
curl -X POST localhost:9200/jobs_vieclam24h/_search \
  -H 'Content-Type: application/json' -d '
{
  "query": {"range": {"salary_min": {"gte": 10, "lte": 20}}}
}'
```

### Filter by experience

```bash
curl -X POST localhost:9200/jobs_vieclam24h/_search \
  -H 'Content-Type: application/json' -d '
{
  "query": {"term": {"experience_tags": "C"}}
}'
```

---

## 9. Code Reference

| Component | Path |
|-----------|------|
| Entry Point | `cmd/worker/main.go` |
| Worker | `internal/module/worker/worker.go` |
| Normalizer | `internal/common/normalizer/normalizer.go` |
| Cleaner | `internal/common/cleaner/cleaner.go` |
| Indexer | `internal/common/indexer/elasticsearch.go` |

---

## 10. Troubleshooting

### Check queue

```bash
redis-cli LLEN jobs:raw:vieclam24h
```

### Check ES health

```bash
curl localhost:9200/_cluster/health
curl localhost:9200/jobs_vieclam24h/_count
```

### Check logs

```bash
docker logs worker
```

### Clear queue

```bash
redis-cli DEL jobs:raw:vieclam24h
```

### Reindex

```bash
curl -X DELETE localhost:9200/jobs_vieclam24h
docker restart worker
```

### Common Issues

| Issue | Solution |
|-------|----------|
| ES connection failed | Check ES health, restart |
| Mapping conflict | Delete index, restart worker |
| Normalization error | Check logs for field issues |
| Queue empty | Check enricher is running |
