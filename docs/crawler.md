# Vieclam24h Crawler

**Stage 1** trong pipeline thu thập dữ liệu - Fetch danh sách jobs từ API Vieclam24h.

---

## Mục lục

1. [Tổng quan](#1-tổng-quan)
2. [Kiến trúc](#2-kiến-trúc)
3. [Input - API Vieclam24h](#3-input---api-vieclam24h)
4. [Luồng xử lý chi tiết](#4-luồng-xử-lý-chi-tiết)
5. [Output](#5-output)
6. [Deduplication System](#6-deduplication-system)
7. [Cấu hình](#7-cấu-hình)
8. [Code Reference](#8-code-reference)
9. [Troubleshooting](#9-troubleshooting)

---

## 1. Tổng quan

### 1.1 Chức năng chính

Crawler thực hiện các nhiệm vụ sau:

- **API Fetching**: Gọi API Vieclam24h để lấy danh sách jobs theo phân trang
- **Deduplication**: Kiểm tra với Redis để skip jobs không thay đổi
- **Data Extraction**: Chuyển đổi API response thành `RawJob` format
- **Queue Publishing**: Push jobs mới/cập nhật vào Redis queue
- **Scheduling**: Tự động chạy mỗi 6 giờ

### 1.2 Vị trí trong Pipeline

```mermaid
flowchart LR
    subgraph Stage1["📦 Stage 1: Crawler"]
        API[("Vieclam24h API")]
        Crawler["Crawler Service"]
    end
    
    subgraph Storage["💾 Redis"]
        Queue1[("jobs:pending:vieclam24h")]
        Dedup[("job:seen:*")]
    end
    
    subgraph Stage2["🔧 Stage 2"]
        Enricher["Enricher"]
    end
    
    API -->|"GET /get-job-list"| Crawler
    Crawler -->|"Check"| Dedup
    Crawler -->|"LPUSH"| Queue1
    Queue1 -->|"BRPOP"| Enricher
    
    style Stage1 fill:#e3f2fd,stroke:#1976d2
    style Crawler fill:#bbdefb
```

---

## 2. Kiến trúc

### 2.1 Component Diagram

```mermaid
flowchart TB
    subgraph Crawler["Crawler Service"]
        Main["main.go"]
        CrawlerMod["vieclam24h.Crawler"]
        Dedup["dedup.Deduplicator"]
        Publisher["queue.Publisher"]
        HTTP["http.Client"]
    end
    
    subgraph External["External Services"]
        API[("Vieclam24h API\nhttps://apiv2.vieclam24h.vn")]
        Redis[("Redis\nredis:6379")]
    end
    
    Main --> CrawlerMod
    CrawlerMod --> HTTP
    CrawlerMod --> Dedup
    CrawlerMod --> Publisher
    
    HTTP -->|"HTTPS"| API
    Dedup -->|"GET/SET"| Redis
    Publisher -->|"LPUSH"| Redis
    
    style Crawler fill:#e8f5e9,stroke:#43a047
    style API fill:#fff3e0,stroke:#ff9800
    style Redis fill:#ffebee,stroke:#e53935
```

### 2.2 Class Structure

```mermaid
classDiagram
    class Crawler {
        -client *http.Client
        -config Config
        -dedup *Deduplicator
        -pendingQueue *Publisher
        +Crawl(ctx) []*RawJob, error
        +CrawlWithCallback(ctx, handler) error
        +Source() JobSource
        -fetchPage(ctx, page) *APIResponse, error
        -itemToRawJob(item) *RawJob
    }
    
    class Config {
        +MaxPages int
        +PerPage int
        +RequestDelay Duration
        +BearerToken string
        +Branch string
    }
    
    class Deduplicator {
        +CheckJob(ctx, source, id, lastUpdated) Result
        +MarkSeenWithTTL(ctx, source, id, lastUpdated, expiredOn) error
    }
    
    class Publisher {
        +Publish(ctx, job) error
        +PublishBatch(ctx, jobs) error
    }
    
    Crawler --> Config
    Crawler --> Deduplicator
    Crawler --> Publisher
```

---

## 3. Input - API Vieclam24h

### 3.1 Endpoint

```
GET https://apiv2.vieclam24h.vn/employer/fe/job/get-job-list
```

### 3.2 Request Headers

| Header | Value | Mô tả |
|--------|-------|-------|
| `Authorization` | `Bearer <token>` | JWT token xác thực |
| `X-Branch` | `vl24h.north` | Region: north/south |
| `Accept` | `application/json` | Response format |
| `User-Agent` | `Mozilla/5.0...` | Browser simulation |

### 3.3 Query Parameters

| Param | Type | Required | Default | Mô tả |
|-------|------|----------|---------|-------|
| `page` | int | Yes | 1 | Số trang (1-indexed) |
| `per_page` | int | No | 30 | Jobs/trang (max 100) |
| `request_from` | string | No | `search_result_web` | Source identifier |

### 3.4 Sample Request

```bash
curl -X GET "https://apiv2.vieclam24h.vn/employer/fe/job/get-job-list?page=1&per_page=30&request_from=search_result_web" \
  -H "Authorization: Bearer eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9..." \
  -H "X-Branch: vl24h.north" \
  -H "Accept: application/json"
```

### 3.5 Response Structure

```mermaid
classDiagram
    class APIResponse {
        +code int
        +msg string
        +data Data
    }
    
    class Data {
        +items []JobItem
        +pagination Pagination
    }
    
    class Pagination {
        +current_page int
        +last_page int
        +per_page int
        +total int
    }
    
    class JobItem {
        +id int
        +title string
        +title_slug string
        +employer_info EmployerInfo
        +salary_from int
        +salary_to int
        +province_ids []int
        +experience_range int
        +updated_at int64
        +resume_apply_expired int64
    }
    
    class EmployerInfo {
        +id int
        +name string
        +logo string
        +rate_response int
    }
    
    APIResponse --> Data
    Data --> Pagination
    Data --> JobItem
    JobItem --> EmployerInfo
```

### 3.6 Sample Response

```json
{
  "code": 200,
  "msg": "success",
  "data": {
    "items": [
      {
        "id": 200734388,
        "title": "Kỹ Thuật Viên Lắp Đặt Hệ Thống",
        "title_slug": "ky-thuat-vien-lap-dat-he-thong",
        "employer_id": 12345,
        "employer_info": {
          "id": 12345,
          "name": "Công Ty CP Công Nghệ ABC",
          "slug": "cong-ty-cp-cong-nghe-abc",
          "logo": "https://cdn.vieclam24h.vn/upload/employer/12345.png",
          "rate_response": 95
        },
        "salary_from": 8000000,
        "salary_to": 15000000,
        "salary_text": "8 - 15 triệu",
        "salary_unit": 1,
        "province_ids": [1, 8],
        "district_ids": [14, 760],
        "contact_address": "Số 38, Ngách 49, Ngõ 63 Phúc Đồng, Long Biên, Hà Nội",
        "experience_range": 3,
        "working_method": 1,
        "level_requirement": 1,
        "degree_requirement": 3,
        "gender": 0,
        "vacancy_quantity": 5,
        "occupation_ids_main": [103, 104],
        "field_ids_main": 15,
        "total_views": 150,
        "total_resume_applied": 20,
        "job_requirement_html": "<p>Tốt nghiệp Cao đẳng...</p>",
        "other_requirement_html": "<p>Ưu tiên có kinh nghiệm...</p>",
        "created_at": 1735600000,
        "updated_at": 1735689600,
        "approved_at": 1735601000,
        "resume_apply_expired": 1738627199
      }
    ],
    "pagination": {
      "current_page": 1,
      "last_page": 450,
      "per_page": 30,
      "total": 13500
    }
  }
}
```

### 3.7 JobItem Field Reference

| Field | Type | Mô tả |
|-------|------|-------|
| `id` | int | Job ID (primary key) |
| `title` | string | Tiêu đề job |
| `title_slug` | string | URL-safe slug |
| `employer_id` | int | ID công ty |
| `employer_info.name` | string | Tên công ty |
| `employer_info.logo` | string | URL logo |
| `employer_info.rate_response` | int | Tỷ lệ phản hồi (0-100%) |
| `salary_from` | int | Lương tối thiểu (VND) |
| `salary_to` | int | Lương tối đa (VND) |
| `salary_text` | string | Text hiển thị ("8-15 triệu", "Thỏa thuận") |
| `salary_unit` | int | 1=VND, 2=USD |
| `province_ids` | []int | Mảng ID tỉnh/thành phố |
| `district_ids` | []int | Mảng ID quận/huyện |
| `contact_address` | string | Địa chỉ liên hệ đầy đủ |
| `experience_range` | int | Enum kinh nghiệm (1-5) |
| `working_method` | int | 1=Full-time, 2=Part-time, 3=Intern |
| `level_requirement` | int | Cấp bậc (1=Nhân viên, 2=Trưởng nhóm...) |
| `degree_requirement` | int | Bằng cấp (1=Không yêu cầu, 2=THPT...) |
| `gender` | int | 0=Không yêu cầu, 1=Nam, 2=Nữ |
| `vacancy_quantity` | int | Số lượng cần tuyển |
| `total_views` | int | Tổng lượt xem |
| `total_resume_applied` | int | Số CV đã ứng tuyển |
| `updated_at` | int64 | Unix timestamp cập nhật |
| `resume_apply_expired` | int64 | Unix timestamp hết hạn |

### 3.8 Enum Mappings

#### experience_range

| Value | Meaning |
|-------|---------|
| 1 | Không yêu cầu |
| 2 | Dưới 1 năm |
| 3 | 1 năm |
| 4 | 2 năm |
| 5 | 3-5 năm |
| 6 | Trên 5 năm |

#### working_method

| Value | Meaning |
|-------|---------|
| 1 | Toàn thời gian (Full-time) |
| 2 | Bán thời gian (Part-time) |
| 3 | Thực tập sinh (Intern) |

#### level_requirement

| Value | Meaning |
|-------|---------|
| 1 | Nhân viên |
| 2 | Trưởng nhóm |
| 3 | Quản lý |
| 4 | Giám đốc |
| 5 | C-Level |

---

## 4. Luồng xử lý chi tiết

### 4.1 Main Flow

```mermaid
flowchart TD
    Start([Start]) --> Init["Initialize Components"]
    Init --> Schedule["Start Scheduler\n(every 6 hours)"]
    
    Schedule --> RunCrawl["Run Crawler Cycle"]
    RunCrawl --> FetchPage["Fetch API Page N"]
    
    FetchPage --> CheckStatus{HTTP 200?}
    CheckStatus -->|No| LogError["Log Error"]
    LogError --> End([End Cycle])
    
    CheckStatus -->|Yes| ParseJSON["Parse JSON Response"]
    ParseJSON --> CheckItems{Items exist?}
    CheckItems -->|No| End
    
    CheckItems -->|Yes| ProcessLoop["For each JobItem"]
    
    subgraph ProcessJob["Process Single Job"]
        ProcessLoop --> BuildID["Build Job ID\nvieclam24h-{id}"]
        BuildID --> DedupCheck{"Dedup Check\n(Redis)"}
        
        DedupCheck -->|ResultUnchanged| SkipJob["Skip (unchanged)"]
        DedupCheck -->|ResultNew| CreateRawJob["Create RawJob"]
        DedupCheck -->|ResultUpdated| CreateRawJob
        
        CreateRawJob --> PublishQueue["LPUSH to\njobs:pending:vieclam24h"]
        PublishQueue --> MarkSeen["Mark Seen\n(Redis SET with TTL)"]
        MarkSeen --> NextJob["Next Job"]
        SkipJob --> NextJob
    end
    
    NextJob --> MoreJobs{More jobs?}
    MoreJobs -->|Yes| ProcessLoop
    MoreJobs -->|No| CheckPage{More pages?}
    
    CheckPage -->|Yes| Sleep["Sleep 2-5s\n(base + jitter)"]
    Sleep --> IncrPage["page++"]
    IncrPage --> FetchPage
    
    CheckPage -->|No| LogSummary["Log Summary\n(total crawled, new/updated)"]
    LogSummary --> Wait["Wait for next cycle\n(6 hours)"]
    Wait --> RunCrawl
    
    style DedupCheck fill:#fff9c4,stroke:#fbc02d
    style CreateRawJob fill:#c8e6c9,stroke:#43a047
    style SkipJob fill:#ffccbc,stroke:#ff5722
```

### 4.2 Initialization Sequence

```mermaid
sequenceDiagram
    participant Main as main()
    participant Config as config.Load()
    participant Redis as Redis Client
    participant Dedup as Deduplicator
    participant Pub as Publisher
    participant Crawler as Vieclam24h Crawler
    
    Main->>Config: Load()
    Config-->>Main: cfg
    
    Main->>Redis: NewClient(cfg.Redis)
    Main->>Redis: Ping()
    Redis-->>Main: PONG
    Note right of Main: Connected!
    
    Main->>Dedup: NewDeduplicator(redis, "job:seen", 30d)
    Dedup-->>Main: deduplicator
    
    Main->>Pub: NewPublisher(redis, "jobs:pending:vieclam24h")
    Pub-->>Main: publisher
    
    Main->>Crawler: NewCrawler(config, dedup, publisher)
    Crawler-->>Main: crawler
    
    Main->>Main: Start Scheduler (goroutine)
    Main->>Main: Wait for SIGINT/SIGTERM
```

### 4.3 API Fetch Sequence

```mermaid
sequenceDiagram
    participant Crawler
    participant HTTP as http.Client
    participant API as Vieclam24h API
    
    Crawler->>HTTP: NewRequest(GET, url)
    Crawler->>HTTP: SetHeader(Authorization, Bearer...)
    Crawler->>HTTP: SetHeader(X-Branch, vl24h.north)
    
    HTTP->>API: GET /job/get-job-list?page=1&per_page=30
    
    alt Success
        API-->>HTTP: 200 OK + JSON body
        HTTP-->>Crawler: response
        Crawler->>Crawler: json.Unmarshal(body, &APIResponse)
        
        alt API code == 200
            Crawler-->>Crawler: return &apiResp, nil
        else API code != 200
            Crawler-->>Crawler: return nil, error
        end
        
    else Error
        API-->>HTTP: 4xx/5xx or timeout
        HTTP-->>Crawler: error
        Crawler-->>Crawler: return nil, error
    end
```

### 4.4 Deduplication Flow

```mermaid
flowchart TD
    subgraph Input
        JobID["Job ID: 200734388"]
        Updated["updated_at: 1735689600"]
    end
    
    JobID --> BuildKey["Build Key:\njob:seen:vieclam24h:200734388"]
    Updated --> BuildKey
    
    BuildKey --> RedisGet["Redis GET key"]
    
    RedisGet --> Exists{Key exists?}
    Exists -->|No| ReturnNew["Return: ResultNew"]
    
    Exists -->|Yes| Compare{"Compare values\nstored vs new"}
    Compare -->|Different| ReturnUpdated["Return: ResultUpdated"]
    Compare -->|Same| ReturnUnchanged["Return: ResultUnchanged"]
    
    ReturnNew --> ProcessJob["Process Job ✓"]
    ReturnUpdated --> ProcessJob
    ReturnUnchanged --> SkipJob["Skip Job ✗"]
    
    style ReturnNew fill:#c8e6c9
    style ReturnUpdated fill:#fff9c4
    style ReturnUnchanged fill:#ffccbc
```

### 4.5 RawJob Creation

```mermaid
flowchart LR
    subgraph APIData["API JobItem"]
        A1["id: 200734388"]
        A2["title: 'Kỹ Thuật Viên'"]
        A3["employer_info.name: 'ABC'"]
        A4["salary_from: 8000000"]
        A5["province_ids: [1, 8]"]
        A6["updated_at: 1735689600"]
        A7["resume_apply_expired: 1738627199"]
    end
    
    subgraph Transform["itemToRawJob()"]
        T1["Build URL"]
        T2["Parse Dates"]
        T3["Map Fields"]
    end
    
    subgraph Output["RawJob"]
        O1["ID: '200734388'"]
        O2["URL: 'https://vieclam24h.vn/...'"]
        O3["Source: 'vieclam24h'"]
        O4["LastUpdatedOn: '1735689600'"]
        O5["ExpiredOn: 2025-02-04"]
        O6["RawData: {...}"]
    end
    
    A1 --> T1
    A2 --> T3
    A3 --> T3
    A4 --> T3
    A5 --> T1
    A6 --> T2
    A7 --> T2
    
    T1 --> O2
    T2 --> O4
    T2 --> O5
    T3 --> O6
    A1 --> O1
```

---

## 5. Output

### 5.1 Queue

| Property | Value |
|----------|-------|
| Queue Name | `jobs:pending:vieclam24h` |
| Redis Command | `LPUSH` |
| Format | JSON-encoded `RawJob` |

### 5.2 RawJob Structure

```json
{
  "id": "200734388",
  "url": "https://vieclam24h.vn/ky-thuat-vien-lap-dat-c15p1id200734388.html",
  "source": "vieclam24h",
  "last_updated_on": "1735689600",
  "expired_on": "2025-02-04T23:59:59+07:00",
  "extracted_at": "2025-01-09T19:00:00+07:00",
  "raw_data": {
    "jobId": 200734388,
    "jobTitle": "Kỹ Thuật Viên Lắp Đặt Hệ Thống",
    "jobUrl": "https://vieclam24h.vn/...",
    "companyId": 12345,
    "companyName": "Công Ty CP Công Nghệ ABC",
    "companyLogo": "https://cdn.vieclam24h.vn/upload/employer/12345.png",
    "provinceIds": [1, 8],
    "districtIds": [14, 760],
    "contactAddress": "Số 38, Ngách 49, Long Biên, Hà Nội",
    "salaryFrom": 8000000,
    "salaryTo": 15000000,
    "salaryText": "8 - 15 triệu",
    "salaryUnit": 1,
    "experienceRange": 3,
    "workingMethod": 1,
    "levelRequirement": 1,
    "degreeRequirement": 3,
    "gender": 0,
    "vacancyQuantity": 5,
    "occupationIds": [103, 104],
    "fieldIdMain": 15,
    "fieldIdsSub": null,
    "jobDescription": "",
    "jobRequirement": "<p>Tốt nghiệp Cao đẳng...</p>",
    "otherRequirement": "<p>Ưu tiên có kinh nghiệm...</p>",
    "totalViews": 150,
    "totalResumeApplied": 20,
    "rateResponse": 95,
    "createdAt": 1735600000,
    "updatedAt": 1735689600,
    "expiredAt": 1738627199
  }
}
```

### 5.3 RawData Field Mapping

| RawData Field | Source (API) | Type |
|---------------|--------------|------|
| `jobId` | `item.ID` | int |
| `jobTitle` | `item.Title` | string |
| `jobUrl` | (constructed) | string |
| `companyId` | `item.EmployerID` | int |
| `companyName` | `item.EmployerInfo.Name` | string |
| `companyLogo` | `item.EmployerInfo.Logo` | string |
| `provinceIds` | `item.ProvinceIDs` | []int |
| `districtIds` | `item.DistrictIDs` | []int |
| `contactAddress` | `item.ContactAddress` | string |
| `salaryFrom` | `item.SalaryFrom` | int |
| `salaryTo` | `item.SalaryTo` | int |
| `salaryText` | `item.SalaryText` | string |
| `salaryUnit` | `item.SalaryUnit` | int |
| `experienceRange` | `item.ExperienceRange` | int |
| `workingMethod` | `item.WorkingMethod` | int |
| `levelRequirement` | `item.LevelRequirement` | int |
| `degreeRequirement` | `item.DegreeRequirement` | int |
| `jobRequirement` | `item.JobRequirementHTML` | string (HTML) |
| `otherRequirement` | `item.OtherRequirementHTML` | string (HTML) |
| `totalViews` | `item.TotalViews` | int |
| `totalResumeApplied` | `item.TotalResumeApplied` | int |
| `rateResponse` | `item.EmployerInfo.RateResponse` | int |
| `createdAt` | `item.CreatedAt` | int64 |
| `updatedAt` | `item.UpdatedAt` | int64 |
| `expiredAt` | `item.ResumeApplyExpired` | int64 |

### 5.4 URL Construction

```go
// Pattern: https://vieclam24h.vn/{slug}-c{field}p{province}id{id}.html
jobURL := fmt.Sprintf("%s/%s-c%dp%did%d.html",
    BaseURL,           // https://vieclam24h.vn
    item.TitleSlug,    // ky-thuat-vien-lap-dat
    item.FieldIDMain,  // 15
    item.ProvinceIDs[0], // 1 (first province)
    item.ID,           // 200734388
)
// Result: https://vieclam24h.vn/ky-thuat-vien-lap-dat-c15p1id200734388.html
```

---

## 6. Deduplication System

### 6.1 Overview

```mermaid
flowchart TB
    subgraph Redis["Redis Storage"]
        Key1["job:seen:vieclam24h:200734388\n= '1735689600'\nTTL: 30 days"]
        Key2["job:seen:vieclam24h:200734389\n= '1735689500'\nTTL: 15 days"]
        Key3["job:seen:vieclam24h:200734390\n= '1735689400'\nTTL: 7 days"]
    end
    
    subgraph Operations
        Check["CheckJob(source, id, lastUpdated)"]
        Mark["MarkSeenWithTTL(source, id, lastUpdated, expiredOn)"]
    end
    
    Check -->|GET| Redis
    Mark -->|SET with TTL| Redis
```

### 6.2 Key Pattern

```
job:seen:{source}:{job_id}
```

**Example:**

```
job:seen:vieclam24h:200734388
```

### 6.3 Value

```
{updated_at} timestamp as string
```

**Example:**

```
1735689600
```

### 6.4 TTL Calculation

```go
// TTL = (expired_at - now) + 24h buffer
ttl := time.Until(expiredOn) + 24*time.Hour

// Minimum TTL: 24h
if ttl < 24*time.Hour {
    ttl = 24 * time.Hour
}

// Maximum TTL: 30 days (default)
if ttl > 30*24*time.Hour {
    ttl = 30 * 24 * time.Hour
}
```

### 6.5 Check Logic

```go
func (d *Deduplicator) CheckJob(ctx, source, id, lastUpdated) (Result, error) {
    key := fmt.Sprintf("%s:%s:%s", d.prefix, source, id)
    
    stored, err := d.redis.Get(ctx, key).Result()
    if err == redis.Nil {
        return ResultNew, nil  // Key doesn't exist
    }
    if err != nil {
        return ResultError, err
    }
    
    if stored != lastUpdated {
        return ResultUpdated, nil  // Value changed
    }
    
    return ResultUnchanged, nil  // Same value
}
```

### 6.6 Result Types

| Result | Meaning | Action |
|--------|---------|--------|
| `ResultNew` | Job ID chưa từng thấy | Process & index |
| `ResultUpdated` | Job ID đã thấy, nhưng `updated_at` khác | Re-process & update index |
| `ResultUnchanged` | Job ID đã thấy, `updated_at` giống | Skip (không làm gì) |

---

## 7. Cấu hình

### 7.1 Crawler Config

| Field | Type | Default | Mô tả |
|-------|------|---------|-------|
| `MaxPages` | int | 200 | Số trang tối đa (0 = unlimited) |
| `PerPage` | int | 30 | Số jobs mỗi trang (max 100) |
| `RequestDelay` | Duration | 3s | Delay cơ bản giữa requests |
| `BearerToken` | string | (hardcoded) | API authentication token |
| `Branch` | string | `vl24h.north` | Region filter |

### 7.2 Default Config

```go
func DefaultConfig() Config {
    return Config{
        MaxPages:     200,
        PerPage:      30,
        RequestDelay: 3 * time.Second,
        BearerToken:  "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9...",
        Branch:       "vl24h.north",
    }
}
```

### 7.3 Environment Variables

| Variable | Default | Mô tả |
|----------|---------|-------|
| `REDIS_ADDR` | `redis:6379` | Redis connection string |
| `REDIS_PASSWORD` | (empty) | Redis password |
| `REDIS_DB` | `0` | Redis database number |
| `CRAWLER_DELAY_MS` | `2000` | Override RequestDelay (ms) |

### 7.4 Rate Limiting

```mermaid
flowchart LR
    Request1["Request Page 1"] --> Delay1["Sleep 2-5s"]
    Delay1 --> Request2["Request Page 2"]
    Request2 --> Delay2["Sleep 2-5s"]
    Delay2 --> Request3["Request Page 3"]
    
    subgraph DelayCalc["Delay Calculation"]
        Base["Base: 3s (config)"]
        Jitter["Jitter: rand(0-3s)"]
        Total["Total: 3-6s"]
    end
```

```go
// Random delay (Base + 0-3s jitter)
randomDelay := c.config.RequestDelay + time.Duration(rand.Intn(3000))*time.Millisecond
time.Sleep(randomDelay)
```

---

## 8. Code Reference

### 8.1 File Locations

| Component | Path |
|-----------|------|
| Entry Point | `cmd/vieclam24h/crawler/main.go` |
| Crawler Logic | `internal/module/vieclam24h/crawler.go` |
| Types | `internal/module/vieclam24h/types.go` |
| Config | `internal/module/vieclam24h/config.go` |
| Deduplicator | `internal/common/dedup/dedup.go` |
| Publisher | `internal/queue/publisher.go` |

### 8.2 Key Functions

| Function | Description |
|----------|-------------|
| `NewCrawler(cfg, dedup, queue)` | Khởi tạo crawler |
| `Crawl(ctx)` | Crawl tất cả pages, return slice |
| `CrawlWithCallback(ctx, handler)` | Crawl với callback per page |
| `fetchPage(ctx, page)` | Fetch single API page |
| `itemToRawJob(item)` | Convert API item to RawJob |

---

## 9. Troubleshooting

### 9.1 Không fetch được data

```mermaid
flowchart TD
    Problem["Crawler không fetch được data"]
    
    Problem --> Check1["1. Check logs"]
    Check1 --> Cmd1["docker logs vl24h-crawler"]
    
    Problem --> Check2["2. Test API manually"]
    Check2 --> Cmd2["curl với Bearer token"]
    
    Problem --> Check3["3. Check Redis connection"]
    Check3 --> Cmd3["redis-cli PING"]
    
    subgraph Causes["Possible Causes"]
        C1["Token expired"]
        C2["API rate limited"]
        C3["Network/firewall"]
        C4["Redis down"]
    end
```

**Commands:**

```bash
# Check logs
docker logs vl24h-crawler

# Test API directly
curl -H "Authorization: Bearer eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9..." \
  "https://apiv2.vieclam24h.vn/employer/fe/job/get-job-list?page=1"

# Check Redis
redis-cli PING
```

### 9.2 Dedup không hoạt động

```bash
# Check dedup keys
redis-cli KEYS "job:seen:vieclam24h:*" | head -10

# Check specific key
redis-cli GET "job:seen:vieclam24h:200734388"

# Check TTL
redis-cli TTL "job:seen:vieclam24h:200734388"

# Count keys
redis-cli KEYS "job:seen:vieclam24h:*" | wc -l
```

### 9.3 Queue không có data

```bash
# Check queue length
redis-cli LLEN jobs:pending:vieclam24h

# View queue items (first 5)
redis-cli LRANGE jobs:pending:vieclam24h 0 4

# Check if crawler is running
docker ps | grep crawler
```

### 9.4 Clear và chạy lại

```bash
# Clear dedup keys (để crawl lại tất cả)
redis-cli KEYS "job:seen:vieclam24h:*" | xargs redis-cli DEL

# Clear pending queue
redis-cli DEL jobs:pending:vieclam24h

# Restart crawler
docker restart vl24h-crawler

# Watch logs
docker logs -f vl24h-crawler
```

### 9.5 Stats

```bash
# Số jobs trong queue
redis-cli LLEN jobs:pending:vieclam24h

# Số dedup keys
redis-cli KEYS "job:seen:vieclam24h:*" | wc -l

# Memory usage
redis-cli INFO memory | grep used_memory_human
```
