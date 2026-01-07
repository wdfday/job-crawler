# Vieclam24h Data Pipeline - Complete Documentation

Tài liệu chi tiết về hệ thống thu thập và xử lý dữ liệu việc làm từ Vieclam24h.vn.

**Phiên bản:** 2.0
**Cập nhật:** 2026-01-08

---

## Mục lục

1. [Tổng quan kiến trúc](#1-tổng-quan-kiến-trúc)
2. [API Endpoints & Authentication](#2-api-endpoints--authentication)
3. [Type Definitions](#3-type-definitions)
4. [Luồng xử lý chi tiết](#4-luồng-xử-lý-chi-tiết)
5. [Deduplication System](#5-deduplication-system)
6. [Experience Tags System](#6-experience-tags-system)
7. [Field Mapping Tables](#7-field-mapping-tables)
8. [Enum Value Mappings](#8-enum-value-mappings)
9. [Elasticsearch Index](#9-elasticsearch-index)
10. [Queue Architecture](#10-queue-architecture)
11. [Error Handling & Retry](#11-error-handling--retry)
12. [Performance & Scalability](#12-performance--scalability)
13. [Monitoring & Metrics](#13-monitoring--metrics)
14. [Troubleshooting Guide](#14-troubleshooting-guide)
15. [Configuration Reference](#15-configuration-reference)
16. [Sample Queries](#16-sample-queries)
17. [Commands](#17-commands)

---

## 1. Tổng quan kiến trúc

### 1.1 System Architecture

```mermaid
flowchart TB
    subgraph Sources["🌐 Data Sources"]
        API["Vieclam24h API<br/>Bearer Token Auth<br/>JobItem (40+ fields)"]
        HTML["Detail Page HTML<br/>JSON-LD Schema.org<br/>Additional metadata"]
    end

    subgraph Pipeline["⚙️ Processing Pipeline (3 Stages)"]
        direction LR
        C["📦 Crawler<br/>API Fetch<br/>Dedup Check"]
        E["🔧 Enricher<br/>HTML Scrape<br/>JSON-LD Parse"]
        W["📊 Worker<br/>Normalize<br/>Transform"]
    end

    subgraph Storage["💾 Storage Layer"]
        Redis[("🔴 Redis<br/>Queue + Dedup<br/>TTL-based")]
        ES[("🟢 Elasticsearch<br/>Full-text Index<br/>jobs_vieclam24h")]
    end

    API --> C
    C -->|"RawJob (API data)"| Redis
    Redis -->|"jobs:pending:vieclam24h"| E
    HTML --> E
    E -->|"Enriched RawJob"| Redis
    Redis -->|"jobs:raw:vieclam24h"| W
    W -->|"domain.Job"| ES
    C -.->|"Dedup Check (updated_at)"| Redis

    style C fill:#e3f2fd
    style E fill:#f3e5f5
    style W fill:#e8f5e9
```

### 1.2 Data Flow Summary

| Stage | Input | Processing | Output | Storage |
|-------|-------|-----------|--------|---------|
| **Crawler** | API Response (JSON) | Parse, Dedup, Extract | `RawJob` (40+ fields) | Redis Pending Queue |
| **Enricher** | RawJob + HTML | HTML Parse, JSON-LD Extract | Enriched `RawJob` | Redis Raw Queue |
| **Worker** | Enriched RawJob | Normalize, Transform, Clean | `domain.Job` | Elasticsearch |

### 1.3 Key Features

- **Deduplication**: Redis-based change detection using `updated_at` timestamp
- **Two-Stage Queue**: Separation of concerns (API vs HTML)
- **Schema.org Compliance**: JSON-LD extraction for structured data
- **Experience Aggregation**: Tag-based system for flexible search (A-F tags)
- **Multi-location Support**: Array-based location storage
- **Salary Normalization**: VND to millions conversion, negotiable detection
- **TTL Management**: Automatic job expiration based on `expired_at`

---

## 2. API Endpoints & Authentication

### 2.1 Base Configuration

```go
Base URL: https://api-job.vieclam24h.vn/api
Bearer Token: eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9...
Channel Code: vl24h
Branch: vl24h.north (Miền Bắc) | vl24h.south (Miền Nam)
```

### 2.2 Main Endpoint

```
GET /job/get-job-list
```

**Headers:**
```http
Authorization: Bearer eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9...
Content-Type: application/json
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)
```

**Query Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `page` | int | Yes | 1 | Page number (1-indexed) |
| `per_page` | int | No | 30 | Items per page (max: 100) |
| `channel_code` | string | Yes | - | "vl24h" |
| `branch` | string | No | - | "vl24h.north" or "vl24h.south" |

**Sample Request:**
```bash
curl -X GET "https://api-job.vieclam24h.vn/api/job/get-job-list?page=1&per_page=30&channel_code=vl24h&branch=vl24h.north" \
  -H "Authorization: Bearer eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9..."
```

### 2.3 API Response Structure

```json
{
  "code": 200,
  "msg": "success",
  "data": {
    "items": [
      {
        "id": 200734388,
        "channel_code": "vl24h",
        "title": "Kỹ Thuật Viên Lắp Đặt",
        "employer_info": {
          "id": 12345,
          "name": "Công Ty CP Công Nghệ Môi Trường",
          "logo": "https://cdn.vieclam24h.vn/...",
          "rate_response": 95
        },
        "salary_from": 8000000,
        "salary_to": 15000000,
        "province_ids": [1],
        "district_ids": [14],
        "experience_range": 3,
        "working_method": 1,
        "updated_at": 1735689600,
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

### 2.4 Detail Page URL Pattern

```
https://vieclam24h.vn/viec-lam/{title-slug}-i{job_id}.html
```

**Example:**
```
https://vieclam24h.vn/viec-lam/ky-thuat-vien-lap-dat-he-thong-quan-trac-tu-dong-i200734388.html
```

**Construction:**
```go
jobURL := fmt.Sprintf("https://vieclam24h.vn/viec-lam/%s-i%d.html",
    item.TitleSlug, item.ID)
```

---

## 3. Type Definitions

### 3.1 API Response Types (Crawler)

#### 3.1.1 Main Response Structure

```go
// APIResponse - Root response from API
type APIResponse struct {
    Code int    `json:"code"`    // 200 = success
    Msg  string `json:"msg"`     // "success"
    Data struct {
        Items      []JobItem  `json:"items"`
        Pagination Pagination `json:"pagination"`
    } `json:"data"`
}

// Pagination - Pagination metadata
type Pagination struct {
    CurrentPage int `json:"current_page"` // 1-indexed
    LastPage    int `json:"last_page"`    // Total pages
    PerPage     int `json:"per_page"`     // Items per page
    Total       int `json:"total"`        // Total jobs
}
```

#### 3.1.2 JobItem (40+ Fields)

```go
// JobItem - Complete job data from API
type JobItem struct {
    // === IDENTIFICATION ===
    ID          int    `json:"id"`            // Primary key
    ChannelCode string `json:"channel_code"`  // "vl24h"
    Title       string `json:"title"`
    TitleSlug   string `json:"title_slug"`    // URL-safe slug

    // === EMPLOYER ===
    EmployerID   int          `json:"employer_id"`
    EmployerInfo EmployerInfo `json:"employer_info"`

    // === LOCATION (ID-based) ===
    ProvinceIDs []int `json:"province_ids"`  // [1, 8] = Hà Nội, TP.HCM
    DistrictIDs []int `json:"district_ids"`  // [14, 760]
    ContactAddress string `json:"contact_address"` // Full address text

    // === CLASSIFICATION (Enum IDs) ===
    LevelRequirement  int   `json:"level_requirement"`  // 1-5 (Nhân viên, Trưởng nhóm, etc.)
    DegreeRequirement int   `json:"degree_requirement"` // 1-5 (Không yêu cầu, Cao đẳng, etc.)
    ExperienceRange   int   `json:"experience_range"`   // 1-5 (0 năm, 1 năm, 2 năm, etc.)
    WorkingMethod     int   `json:"working_method"`     // 1-3 (Full-time, Part-time, Intern)
    Gender            int   `json:"gender"`             // 0=Không yêu cầu, 1=Nam, 2=Nữ
    OccupationIDs     []int `json:"occupation_ids_main"` // Job category IDs
    FieldIDMain       int   `json:"field_ids_main"`     // Main field/industry ID
    FieldIDsSub       any   `json:"field_ids_sub"`      // Sub-fields (can be int or []int)

    // === SALARY ===
    SalaryFrom int    `json:"salary_from"` // VND (e.g., 8000000)
    SalaryTo   int    `json:"salary_to"`   // VND (e.g., 15000000)
    SalaryText string `json:"salary_text"` // "8 - 15 triệu" or "Thỏa thuận"
    SalaryUnit int    `json:"salary_unit"` // 1=VND, 2=USD

    // === JOB DETAILS ===
    VacancyQuantity int    `json:"vacancy_quantity"`      // Number of positions
    ContactName     string `json:"contact_name"`          // Contact person
    Status          int    `json:"status"`                // 1=Active, 0=Inactive

    // === CONTENT (HTML) ===
    JobRequirement       string   `json:"job_requirement"`        // Plain text
    JobRequirementHTML   string   `json:"job_requirement_html"`   // Rich HTML
    OtherRequirement     string   `json:"other_requirement"`
    OtherRequirementHTML string   `json:"other_requirement_html"`
    JobBox               []JobBox `json:"job_box"`                // Description blocks

    // === STATISTICS ===
    TotalViews         int `json:"total_views"`
    TotalResumeApplied int `json:"total_resume_applied"`

    // === TIMESTAMPS (Unix seconds) ===
    CreatedAt          int64 `json:"created_at"`
    UpdatedAt          int64 `json:"updated_at"`            // For dedup
    ApprovedAt         int64 `json:"approved_at"`
    ResumeApplyExpired int64 `json:"resume_apply_expired"`  // Expiration date
}

// EmployerInfo - Company details
type EmployerInfo struct {
    ID           int    `json:"id"`
    Name         string `json:"name"`
    Slug         string `json:"slug"`
    Status       int    `json:"status"`      // 1=Active
    Logo         string `json:"logo"`        // CDN URL
    RateResponse int    `json:"rate_response"` // 0-100% response rate
}

// JobBox - Description content blocks
type JobBox struct {
    ID          int    `json:"id"`
    ServiceCode string `json:"service_code"`  // "description", "benefit"
    Content     string `json:"content"`       // Plain text
    ContentHTML string `json:"content_html"`  // Rich HTML
}
```

### 3.2 JSON-LD Types (Enricher)

#### 3.2.1 Schema.org JobPosting

```go
// JobPosting - Structured data from detail page
type JobPosting struct {
    Context  string `json:"@context"`  // "https://schema.org"
    Type     string `json:"@type"`     // "JobPosting"

    // === BASIC INFO ===
    Title       string `json:"title"`
    Description string `json:"description"`      // Full job description
    JobBenefits string `json:"jobBenefits"`      // Benefit details

    // === REQUIREMENTS ===
    Skills         string `json:"skills"`         // Comma-separated
    Qualifications string `json:"qualifications"` // Education requirement

    // === CLASSIFICATION ===
    Industry             string `json:"industry"`              // "Điện - Điện tử, Cơ khí"
    OccupationalCategory string `json:"occupationalCategory"`  // Position level
    EmploymentType       string `json:"employmentType"`        // "FULL_TIME"

    // === METADATA ===
    TotalJobOpenings int    `json:"totalJobOpenings"`  // Vacancy count
    DatePosted       string `json:"datePosted"`        // ISO 8601
    ValidThrough     string `json:"validThrough"`      // Expiration (ISO 8601)

    // === NESTED OBJECTS ===
    Identifier         Identifier   `json:"identifier"`
    HiringOrganization Organization `json:"hiringOrganization"`
    JobLocation        []Location   `json:"jobLocation"`  // Array for multi-location
    BaseSalary         BaseSalary   `json:"baseSalary"`
}

// Identifier - Job ID info
type Identifier struct {
    Type  string `json:"@type"`  // "PropertyValue"
    Name  string `json:"name"`   // "Job ID"
    Value string `json:"value"`  // "200734388"
}

// Organization - Company info from Schema.org
type Organization struct {
    Type   string `json:"@type"`  // "Organization"
    Name   string `json:"name"`
    SameAs string `json:"sameAs"` // Company website
    Logo   string `json:"logo"`
}

// Location - Structured location
type Location struct {
    Type    string  `json:"@type"`  // "Place"
    Address Address `json:"address"`
}

// Address - Detailed address breakdown
type Address struct {
    Type            string `json:"@type"`           // "PostalAddress"
    StreetAddress   string `json:"streetAddress"`   // "Số 38, Ngách 49, Ngõ 63"
    AddressLocality string `json:"addressLocality"` // "Nam Từ Liêm" (District)
    AddressRegion   string `json:"addressRegion"`   // "Hà Nội" (Province)
    PostalCode      string `json:"postalCode"`
    AddressCountry  string `json:"addressCountry"`  // "VN"
}

// BaseSalary - Salary structure
type BaseSalary struct {
    Type     string      `json:"@type"`   // "MonetaryAmount"
    Currency string      `json:"currency"` // "VND"
    Value    SalaryValue `json:"value"`
}

// SalaryValue - Salary range or negotiable
type SalaryValue struct {
    Type     string `json:"@type"`    // "QuantitativeValue"
    UnitText string `json:"unitText"` // "MONTH", "YEAR"
    MinValue int    `json:"minValue"` // 8000000 (VND)
    MaxValue int    `json:"maxValue"` // 15000000 (VND)
    Value    string `json:"value"`    // "Thỏa thuận" (if negotiable)
}
```

### 3.3 Domain Types (Worker Output)

#### 3.3.1 RawJob (Intermediate)

```go
// RawJob - Pre-normalization data structure
type RawJob struct {
    ID            string         `json:"id"`              // "200734388"
    URL           string         `json:"url"`             // Detail page URL
    Source        string         `json:"source"`          // "vieclam24h"
    RawData       map[string]any `json:"raw_data"`        // Dynamic fields
    HTMLContent   string         `json:"html_content,omitempty"` // Temporary
    ExtractedAt   time.Time      `json:"extracted_at"`
    LastUpdatedOn string         `json:"last_updated_on,omitempty"` // For dedup
    ExpiredOn     time.Time      `json:"expired_on,omitempty"`      // For TTL
}
```

**RawData Map Contents (After Enrichment):**

```go
RawData: map[string]any{
    // === FROM API (Crawler) ===
    "jobId":              200734388,
    "jobTitle":           "Kỹ Thuật Viên...",
    "jobUrl":             "https://vieclam24h.vn/...",
    "companyId":          12345,
    "companyName":        "Công Ty CP...",
    "companyLogo":        "https://cdn.vieclam24h.vn/...",
    "provinceIds":        []int{1, 8},
    "districtIds":        []int{14, 760},
    "contactAddress":     "Số 38, Ngách 49...",
    "salaryFrom":         8000000,
    "salaryTo":           15000000,
    "salaryText":         "8 - 15 triệu",
    "salaryUnit":         1,
    "experienceRange":    3,
    "workingMethod":      1,
    "levelRequirement":   1,
    "degreeRequirement":  3,
    "jobRequirement":     "<p>HTML content</p>",
    "otherRequirement":   "<p>HTML content</p>",
    "totalViews":         150,
    "totalResumeApplied": 20,
    "rateResponse":       95,
    "createdAt":          1735689600,
    "updatedAt":          1735776000,
    "expiredAt":          1738627199,

    // === FROM JSON-LD (Enricher) ===
    "canonicalUrl":         "https://vieclam24h.vn/...",
    "jobDescription":       "Full description text",
    "jobBenefits":          "BHXH, BHYT...",
    "skills":               "PLC, SCADA, AutoCAD",
    "qualifications":       "Cao đẳng",
    "industry":             []string{"Điện - Điện tử", "Cơ khí"},
    "occupationalCategory": "Nhân viên",
    "employmentType":       "FULL_TIME",
    "companyWebsite":       "https://example.com",
    "locationCity":         []string{"Hà Nội", "TP.HCM"},
    "locationDistrict":     []string{"Nam Từ Liêm", "Quận 1"},
    "salaryMinJsonLd":      8000000,
    "salaryMaxJsonLd":      15000000,
    "salaryCurrency":       "VND",
    "salaryTextJsonLd":     "Thỏa thuận",
    "isNegotiable":         true,

    // === FROM HTML PARSING (Enricher) ===
    "experienceText":       "1 năm",
}
```

#### 3.3.2 domain.Job (Final)

```go
// Job - Normalized, indexed document
type Job struct {
    // === IDENTIFICATION ===
    ID        string `json:"id"`         // "vieclam24h-200734388"
    Source    string `json:"source"`     // "vieclam24h"
    SourceURL string `json:"source_url"` // Detail page URL

    // === BASIC INFO ===
    Title    string `json:"title"`
    Company  string `json:"company"`
    Location string `json:"location"`  // Full address text
    Position string `json:"position"`  // Level (Nhân viên, Trưởng nhóm)
    WorkType string `json:"work_type"` // "Toàn thời gian", "Bán thời gian"

    // === LOCATION (Arrays for multi-location) ===
    LocationCity     []string `json:"location_city"`     // ["Hà Nội", "TP.HCM"]
    LocationDistrict []string `json:"location_district"` // ["Nam Từ Liêm", "Quận 1"]

    // === SALARY ===
    Salary       string `json:"salary"`        // "8 - 15 triệu" or "Thỏa thuận"
    SalaryMin    int    `json:"salary_min"`    // 8 (in triệu VND)
    SalaryMax    int    `json:"salary_max"`    // 15 (in triệu VND)
    IsNegotiable bool   `json:"is_negotiable"` // true if "Thỏa thuận"

    // === EXPERIENCE ===
    Experience string   `json:"experience"`      // "1 năm", "2 năm"
    ExpTags    []string `json:"experience_tags"` // ["C", "D", "E", "F"]

    // === CLASSIFICATION ===
    Industry       []string `json:"industry"`       // ["Điện - Điện tử", "Cơ khí"]
    Field          string   `json:"field"`          // Main industry
    Qualifications string   `json:"qualifications"` // "Cao đẳng"
    Skills         []string `json:"skills"`         // ["PLC", "SCADA"]

    // === CONTENT ===
    Description  string `json:"description"`  // HTML cleaned
    Requirements string `json:"requirements"` // HTML cleaned
    Benefits     string `json:"benefits"`     // HTML cleaned

    // === METADATA ===
    TotalViews           int       `json:"total_views"`
    TotalResumeApplied   int       `json:"total_resume_applied"`
    RateResponse         float64   `json:"rate_response"`
    CompanyWebsite       string    `json:"company_website"`
    OccupationalCategory string    `json:"occupational_category"`
    EmploymentType       string    `json:"employment_type"`

    // === TIMESTAMPS ===
    ExpiredAt time.Time `json:"expired_at"`
    CrawledAt time.Time `json:"crawled_at"`
}
```

---

## 4. Luồng xử lý chi tiết

### 4.1 Stage 1: Crawler (API Fetch)

```mermaid
flowchart TD
    subgraph Init["🚀 Initialization"]
        A[Load Config] --> B[Connect Redis]
        B --> C[Initialize Deduplicator]
        C --> D[Setup HTTP Client]
    end

    subgraph Loop["🔄 Main Crawl Loop"]
        D --> E[Fetch API Page N]
        E --> F{Status 200?}
        F -->|No| G[Log Error]
        F -->|Yes| H[Parse JSON Response]
        G --> Z[End]

        H --> I{Items exist?}
        I -->|No| Z
        I -->|Yes| J[For each JobItem]

        J --> K{Dedup Check}
        K -->|ResultNew| L[Create RawJob]
        K -->|ResultUpdated| L
        K -->|ResultUnchanged| M[Skip]

        L --> N[Build RawData Map]
        N --> O[Push to Pending Queue]
        O --> P[Mark Seen in Redis]
        P --> Q{More items?}
        M --> Q

        Q -->|Yes| J
        Q -->|No| R{More pages?}
        R -->|Yes| S[Sleep 2s + jitter]
        S --> E
        R -->|No| Z
    end

    style K fill:#fff9c4
    style L fill:#c8e6c9
    style M fill:#ffccbc
```

**Key Operations:**

1. **API Fetch**
   ```go
   url := fmt.Sprintf("%s/job/get-job-list?page=%d&per_page=%d&channel_code=%s&branch=%s",
       baseURL, page, perPage, channelCode, branch)

   req.Header.Set("Authorization", "Bearer "+bearerToken)
   req.Header.Set("User-Agent", userAgent)
   ```

2. **Dedup Check**
   ```go
   result := dedup.CheckJob(ctx, "vieclam24h", strconv.Itoa(item.ID), item.UpdatedAt)

   switch result {
   case ResultNew:       // Never seen before
   case ResultUpdated:   // Job was modified
   case ResultUnchanged: // Skip processing
   }
   ```

3. **RawJob Creation**
   ```go
   rawJob := &domain.RawJob{
       ID:            fmt.Sprintf("vieclam24h-%d", item.ID),
       URL:           jobURL,
       Source:        "vieclam24h",
       LastUpdatedOn: strconv.FormatInt(item.UpdatedAt, 10),
       ExpiredOn:     time.Unix(item.ResumeApplyExpired, 0),
       ExtractedAt:   time.Now(),
       RawData:       buildRawDataMap(item),
   }
   ```

4. **Queue Push**
   ```go
   err := publisher.Publish(ctx, rawJob)
   // Publishes to "jobs:pending:vieclam24h"
   ```

5. **Dedup Mark**
   ```go
   err := dedup.MarkSeenWithTTL(ctx, "vieclam24h", jobID, updatedAt, expiredOn)
   // Sets key with TTL = (expiredOn - now) + 24h
   ```

**Performance Considerations:**

- **Request Delay:** 2s base + 0-3s random jitter
- **Pagination:** Automatic stop at `LastPage`
- **Error Handling:** Log and continue (don't crash entire crawl)
- **Concurrency:** Single-threaded to avoid rate limiting

### 4.2 Stage 2: Enricher (HTML Scrape)

```mermaid
flowchart TD
    subgraph Consumer["📥 Queue Consumer"]
        A[BRPOP Pending Queue<br/>Timeout: 5s] --> B{Job found?}
        B -->|No| A
        B -->|Yes| C[Deserialize RawJob]
    end

    subgraph Fetch["🌐 HTML Fetch"]
        C --> D[HTTP GET job.URL]
        D --> E{Status 200?}
        E -->|No| F[Log Error]
        E -->|Yes| G[Read HTML Body]
        F --> H[Use Partial Data]
    end

    subgraph Parse["🔍 HTML Parsing"]
        G --> I[goquery Parse]
        I --> J[Extract JSON-LD]
        J --> K[Parse JobPosting]
        K --> L[Extract Experience<br/>from HTML div]
        L --> M[Extract Canonical URL]
    end

    subgraph Enrich["✨ Data Enrichment"]
        M --> N[Merge JSON-LD into RawData]
        H --> N
        N --> O[Process Industry Array]
        O --> P[Deduplicate Locations]
        P --> Q[Detect Negotiable Salary]
        Q --> R[Clear HTMLContent]
    end

    subgraph Publish["📤 Queue Publish"]
        R --> S[Set ExtractedAt]
        S --> T[Push to Raw Queue]
        T --> U[Sleep delay + jitter]
        U --> A
    end

    style J fill:#e1bee7
    style N fill:#c5e1a5
    style T fill:#ffccbc
```

**Key Operations:**

1. **Queue Consumer**
   ```go
   consumer.Run(ctx, func(job *domain.RawJob) error {
       // Process each job from pending queue
   })
   // Uses BRPOP with 5s timeout
   ```

2. **HTML Fetch**
   ```go
   func fetchHTML(ctx context.Context, url string) (string, error) {
       req.Header.Set("User-Agent", "Mozilla/5.0...")
       req.Header.Set("Accept", "text/html,application/xhtml+xml...")
       req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9")

       // 30s timeout per request
   }
   ```

3. **JSON-LD Extraction**
   ```go
   doc.Find("script[type='application/ld+json']").Each(func(i int, s *goquery.Selection) {
       jsonContent := s.Text()

       var jobPosting JobPosting
       json.Unmarshal([]byte(jsonContent), &jobPosting)

       if jobPosting.Type == "JobPosting" {
           // Extract fields
       }
   })
   ```

4. **Experience Parsing**
   ```go
   // HTML structure:
   //   <div class="flex flex-col">
   //     <div>Kinh nghiệm</div>
   //     <div>1 năm</div>
   //   </div>

   doc.Find("div.flex.flex-col").Each(func(i int, parent *goquery.Selection) {
       children := parent.Children().Filter("div")
       if children.Length() >= 2 {
           label := strings.TrimSpace(children.First().Text())
           if label == "Kinh nghiệm" {
               value := strings.TrimSpace(children.Eq(1).Text())
               job.RawData["experienceText"] = value
           }
       }
   })
   ```

5. **Location Deduplication**
   ```go
   citySet := make(map[string]bool)
   districtSet := make(map[string]bool)
   var cities, districts []string

   for _, loc := range jobPosting.JobLocation {
       if !citySet[loc.Address.AddressRegion] {
           cities = append(cities, loc.Address.AddressRegion)
           citySet[loc.Address.AddressRegion] = true
       }
   }

   job.RawData["locationCity"] = cities
   job.RawData["locationDistrict"] = districts
   ```

6. **Salary Priority Logic**
   ```go
   // JSON-LD salary takes priority over API salary
   if jobPosting.BaseSalary.Value.MinValue > 0 {
       job.RawData["salaryMinJsonLd"] = jobPosting.BaseSalary.Value.MinValue
       job.RawData["salaryMaxJsonLd"] = jobPosting.BaseSalary.Value.MaxValue
   }

   // Detect negotiable
   if jobPosting.BaseSalary.Value.Value == "Thỏa thuận" {
       job.RawData["isNegotiable"] = true
   }
   ```

**Error Handling:**

- **Fetch Failure:** Log error, proceed with API data only
- **Parse Error:** Skip JSON-LD, use API fallback
- **Missing Fields:** Use default values (empty strings, empty arrays)

**Performance:**

- **Request Delay:** 5s base + 0-3s random jitter
- **Timeout:** 30s per HTTP request
- **Concurrency:** Can run multiple enricher instances

### 4.3 Stage 3: Worker (Normalization)

```mermaid
flowchart TD
    subgraph Consumer["📥 Queue Consumer"]
        A[BRPOP Raw Queue<br/>Timeout: 5s] --> B{Job found?}
        B -->|No| A
        B -->|Yes| C[Deserialize RawJob]
    end

    subgraph Detect["🔍 Source Detection"]
        C --> D{Check job.Source}
        D -->|vieclam24h| E[normalizeVieclam24h]
        D -->|topcv| F[normalizeTopCV]
        D -->|other| G[Skip]
    end

    subgraph Normalize["🔧 Vieclam24h Normalization"]
        E --> H[Extract Basic Fields]
        H --> I[Process Salary<br/>Priority: JSON-LD > API]
        I --> J[Convert VND to triệu]
        J --> K[Build Salary Text]
        K --> L[Map Experience → Tags]
        L --> M[Clean HTML Content]
        M --> N[Set Default Values]
        N --> O[Populate Timestamps]
    end

    subgraph Batch["📦 Batch Processing"]
        O --> P[Add to Batch Buffer]
        P --> Q{Batch size >= 50?}
        Q -->|Yes| R[Bulk Index to ES]
        Q -->|No| A
        R --> S{Success?}
        S -->|Yes| A
        S -->|No| T[Log Error, Retry]
        T --> A
    end

    style E fill:#bbdefb
    style I fill:#fff9c4
    style R fill:#c8e6c9
```

**Normalization Steps:**

1. **Basic Field Extraction**
   ```go
   job := &domain.Job{
       ID:       rawJob.ID,
       Source:   "vieclam24h",
       SourceURL: rawJob.URL,
       Title:    getString(data, "jobTitle", "title"),
       Company:  getString(data, "companyName"),
       Location: getString(data, "contactAddress"),
       Position: getString(data, "occupationalCategory"),
   }
   ```

2. **Salary Processing (Priority Logic)**
   ```go
   // Priority: JSON-LD > API
   salaryMin := getInt(data, "salaryMinJsonLd", "salaryFrom")
   salaryMax := getInt(data, "salaryMaxJsonLd", "salaryTo")

   // Convert VND to triệu
   if salaryMin > 1000 {
       salaryMin = salaryMin / 1_000_000
   }
   if salaryMax > 1000 {
       salaryMax = salaryMax / 1_000_000
   }

   job.SalaryMin = salaryMin
   job.SalaryMax = salaryMax

   // Build text representation
   if getBool(data, "isNegotiable") {
       job.IsNegotiable = true
       job.Salary = "Thỏa thuận"
   } else if salaryMin > 0 && salaryMax > 0 {
       job.Salary = fmt.Sprintf("%d - %d triệu", salaryMin, salaryMax)
   } else if salaryMin > 0 {
       job.Salary = fmt.Sprintf("Từ %d triệu", salaryMin)
   } else {
       job.Salary = "Không xác định"
   }
   ```

3. **Experience Tag Mapping**
   ```go
   experienceText := getString(data, "experienceText")
   job.Experience = experienceText
   job.ExpTags = mapExperienceToTags(experienceText)

   // See section 6 for full mapping logic
   ```

4. **Location Processing**
   ```go
   // Already deduplicated by Enricher
   job.LocationCity = getStringArray(data, "locationCity")
   job.LocationDistrict = getStringArray(data, "locationDistrict")
   ```

5. **Content Cleaning**
   ```go
   job.Description = cleanHTML(getString(data, "jobDescription", "jobRequirement"))
   job.Requirements = cleanHTML(getString(data, "jobRequirement"))
   job.Benefits = cleanHTML(getString(data, "jobBenefits"))

   func cleanHTML(html string) string {
       // Strip HTML tags but preserve structure
       // Convert <br>, <p> to newlines
       // Remove inline styles
       // Decode HTML entities
   }
   ```

6. **Default Values**
   ```go
   job.Qualifications = getString(data, "qualifications")
   if job.Qualifications == "" {
       job.Qualifications = "Không yêu cầu"
   }

   job.WorkType = mapWorkingMethod(getInt(data, "workingMethod"))
   // 1 → "Toàn thời gian"
   // 2 → "Bán thời gian"
   // 3 → "Thực tập"
   ```

7. **Timestamps**
   ```go
   job.ExpiredAt = rawJob.ExpiredOn
   job.CrawledAt = time.Now()
   ```

**Bulk Indexing:**

```go
// Batch processing (50 jobs per bulk request)
var batch []*domain.Job

for job := range jobs {
    batch = append(batch, normalizeJob(job))

    if len(batch) >= 50 {
        bulkIndexer.Add(ctx, batch)
        batch = batch[:0]
    }
}

// Flush remaining
if len(batch) > 0 {
    bulkIndexer.Add(ctx, batch)
}
```

**Error Handling:**

- **Parse Failure:** Log and skip job
- **Index Failure:** Retry up to 3 times with exponential backoff
- **Malformed Data:** Use defaults, log warning

---

## 5. Deduplication System

### 5.1 Architecture

```mermaid
flowchart LR
    subgraph API["🌐 API Response"]
        A["JobItem<br/>id: 200734388<br/>updated_at: 1735776000"]
    end

    subgraph Dedup["🔍 Deduplicator"]
        B[Generate Key]
        C[Redis GET]
        D{Compare}
    end

    subgraph Redis["🔴 Redis"]
        E[("Key: job:seen:vieclam24h:200734388<br/>Value: 1735776000<br/>TTL: 720h")]
    end

    subgraph Result["✅ Decision"]
        F["ResultNew<br/>→ Process"]
        G["ResultUpdated<br/>→ Re-process"]
        H["ResultUnchanged<br/>→ Skip"]
    end

    A --> B
    B --> C
    C --> E
    E --> D
    D -->|"Key not found"| F
    D -->|"Value changed"| G
    D -->|"Value same"| H

    F --> I[Mark Seen]
    G --> I
    I --> E

    style F fill:#c8e6c9
    style G fill:#fff9c4
    style H fill:#ffccbc
```

### 5.2 Redis Key Structure

**Pattern:**
```
job:seen:{source}:{job_id}
```

**Examples:**
```
job:seen:vieclam24h:200734388
job:seen:vieclam24h:200734389
job:seen:vieclam24h:200734390
```

**Value:**
```
{updated_at_timestamp}
```

**Example:**
```
1735776000
```

**TTL Calculation:**
```go
ttl := (expiredAt - now) + 24 * time.Hour

// Example:
// expiredAt: 2026-02-03 23:59:59
// now:       2026-01-08 03:00:00
// ttl:       26 days + 20 hours + 59 minutes + 59 seconds + 24 hours
//          = 27 days + 20 hours + 59 minutes + 59 seconds
```

### 5.3 Implementation

```go
type Deduplicator struct {
    redis  *redis.Client
    prefix string // "job:seen"
}

type CheckResult int

const (
    ResultNew       CheckResult = iota // 0: Never seen before
    ResultUpdated                      // 1: Job has updates
    ResultUnchanged                    // 2: No changes detected
)

// CheckJob checks if a job should be processed
func (d *Deduplicator) CheckJob(ctx context.Context, source, jobID string, lastUpdatedOn int64) CheckResult {
    key := fmt.Sprintf("%s:%s:%s", d.prefix, source, jobID)

    // GET stored timestamp
    storedValue, err := d.redis.Get(ctx, key).Result()
    if err == redis.Nil {
        // Key doesn't exist → new job
        return ResultNew
    }
    if err != nil {
        // Redis error → assume new (fail-open)
        log.Printf("[Dedup] Redis error for %s: %v", key, err)
        return ResultNew
    }

    // Parse stored timestamp
    storedTimestamp, err := strconv.ParseInt(storedValue, 10, 64)
    if err != nil {
        // Corrupted value → assume new
        return ResultNew
    }

    // Compare timestamps
    if storedTimestamp != lastUpdatedOn {
        return ResultUpdated
    }

    return ResultUnchanged
}

// MarkSeenWithTTL marks a job as seen with expiration-based TTL
func (d *Deduplicator) MarkSeenWithTTL(ctx context.Context, source, jobID string, lastUpdatedOn, expiredOn int64) error {
    key := fmt.Sprintf("%s:%s:%s", d.prefix, source, jobID)
    value := strconv.FormatInt(lastUpdatedOn, 10)

    // Calculate TTL
    expireTime := time.Unix(expiredOn, 0)
    ttl := time.Until(expireTime) + 24*time.Hour

    // Ensure minimum TTL of 1 hour
    if ttl < time.Hour {
        ttl = time.Hour
    }

    return d.redis.Set(ctx, key, value, ttl).Err()
}
```

### 5.4 Decision Flow

```mermaid
graph TD
    A[Job from API] --> B{Redis Key Exists?}

    B -->|No| C[ResultNew]
    B -->|Yes| D{Stored updated_at<br/>== Current updated_at?}

    D -->|No| E[ResultUpdated]
    D -->|Yes| F[ResultUnchanged]

    C --> G[Process Job]
    E --> G
    F --> H[Skip Job]

    G --> I[Push to Queue]
    G --> J[Mark Seen with TTL]

    style C fill:#a5d6a7
    style E fill:#fff59d
    style F fill:#ef9a9a
```

### 5.5 Edge Cases

| Scenario | Behavior | Reason |
|----------|----------|--------|
| Redis down | Process all jobs (ResultNew) | Fail-open to avoid data loss |
| Corrupted value | Process job (ResultNew) | Assume corrupted = needs update |
| TTL expired | Process job (ResultNew) | Job re-appeared after expiration |
| Same `updated_at` | Skip (ResultUnchanged) | No changes detected |
| Different `updated_at` | Re-process (ResultUpdated) | Job was modified |
| Negative TTL | Set minimum 1 hour | Prevent immediate expiration |

### 5.6 Monitoring Metrics

**Key Metrics:**
```go
// Counters
dedup_new_total         // Jobs never seen before
dedup_updated_total     // Jobs with updates
dedup_unchanged_total   // Jobs skipped
dedup_redis_errors      // Redis connection errors

// Gauges
dedup_redis_keys_total  // Total keys in Redis
dedup_ttl_average       // Average TTL in hours
```

**Sample Query (Prometheus):**
```promql
# Dedup hit rate
100 * rate(dedup_unchanged_total[5m]) /
  rate((dedup_new_total + dedup_updated_total + dedup_unchanged_total)[5m])

# Should be 70-90% in steady state
```

---

## 6. Experience Tags System

### 6.1 Tag Definitions

| Tag | Experience Range | Ý nghĩa | Use Case |
|-----|------------------|---------|----------|
| **A** | 0 năm | Fresher, chưa có kinh nghiệm | Sinh viên mới ra trường |
| **B** | 0-1 năm | Entry level | Đã thực tập hoặc làm <1 năm |
| **C** | 1-2 năm | Junior | Đã có kinh nghiệm cơ bản |
| **D** | 2-3 năm | Junior+ | Thành thạo công việc |
| **E** | 3-5 năm | Mid-level | Có thể dẫn dắt nhóm nhỏ |
| **F** | 5+ năm | Senior | Chuyên gia, kinh nghiệm cao |

### 6.2 Aggregation Principle

**Core Idea:** Người có kinh nghiệm cao hơn có thể ứng tuyển vị trí yêu cầu thấp hơn.

**Example:**
- Job requires "1 năm kinh nghiệm"
- Tags generated: `["C", "D", "E", "F"]`
- People with 1-2 năm (C), 2-3 năm (D), 3-5 năm (E), 5+ năm (F) can all apply

### 6.3 Mapping Table

| Job Requirement Text | Tags Generated | Who Can Apply |
|----------------------|----------------|---------------|
| `""` (empty) | `["A","B","C","D","E","F"]` | Tất cả |
| `"Không yêu cầu"` | `["A","B","C","D","E","F"]` | Tất cả |
| `"Chưa có kinh nghiệm"` | `["A","B","C","D","E","F"]` | Tất cả |
| `"Dưới 1 năm"` | `["B","C","D","E","F"]` | 0-1 năm trở lên |
| `"1 năm"` | `["C","D","E","F"]` | 1-2 năm trở lên |
| `"2 năm"` | `["D","E","F"]` | 2-3 năm trở lên |
| `"3 năm"` | `["E","F"]` | 3-5 năm trở lên |
| `"4 năm"` | `["E","F"]` | 3-5 năm trở lên |
| `"5 năm"` | `["E","F"]` | 3-5 năm trở lên |
| `"Hơn 5 năm"` | `["F"]` | Chỉ senior 5+ |
| `"Trên 5 năm"` | `["F"]` | Chỉ senior 5+ |
| `"6 năm"` | `["F"]` | Chỉ senior 5+ |

### 6.4 Implementation

```go
func mapExperienceToTags(exp string) []string {
    exp = strings.TrimSpace(exp)

    // === SPECIAL CASES ===
    if exp == "" || strings.Contains(exp, "Không yêu cầu") {
        return []string{"A", "B", "C", "D", "E", "F"}
    }
    if strings.Contains(exp, "Chưa có kinh nghiệm") {
        return []string{"A", "B", "C", "D", "E", "F"}
    }
    if strings.Contains(exp, "Dưới 1 năm") {
        return []string{"B", "C", "D", "E", "F"}
    }
    if strings.Contains(exp, "Hơn 5 năm") || strings.Contains(exp, "Trên 5 năm") {
        return []string{"F"}
    }

    // === PARSE NUMBER ===
    var years int
    _, err := fmt.Sscanf(exp, "%d", &years)
    if err != nil {
        // Can't parse number → assume no requirement
        return []string{"A", "B", "C", "D", "E", "F"}
    }

    // === MAP YEARS TO TAGS ===
    switch {
    case years == 0:
        return []string{"A", "B", "C", "D", "E", "F"}
    case years <= 1:
        return []string{"C", "D", "E", "F"}
    case years <= 2:
        return []string{"D", "E", "F"}
    case years <= 5:
        return []string{"E", "F"}
    default: // years > 5
        return []string{"F"}
    }
}
```

### 6.5 Search Query Examples

**Use Case 1:** User has 2 years experience (Tag D)

```json
{
  "query": {
    "bool": {
      "must": [
        {
          "term": {
            "experience_tags": "D"
          }
        }
      ]
    }
  }
}
```

**Results:** Jobs requiring 0 năm, dưới 1 năm, 1 năm, 2 năm (all jobs with tag D)

**Use Case 2:** User is a fresher (Tag A)

```json
{
  "query": {
    "bool": {
      "must": [
        {
          "term": {
            "experience_tags": "A"
          }
        }
      ]
    }
  }
}
```

**Results:** Only jobs requiring "Không yêu cầu" or "Chưa có kinh nghiệm"

**Use Case 3:** User has 10 years experience (Tag F)

```json
{
  "query": {
    "bool": {
      "must": [
        {
          "term": {
            "experience_tags": "F"
          }
        }
      ]
    }
  }
}
```

**Results:** ALL jobs (since F is in all tags)

### 6.6 Validation Examples

| Input | Expected Tags | ✅/❌ |
|-------|--------------|------|
| `"0 năm"` | `["A","B","C","D","E","F"]` | ✅ |
| `"1 năm"` | `["C","D","E","F"]` | ✅ |
| `"2 năm"` | `["D","E","F"]` | ✅ |
| `"3 năm"` | `["E","F"]` | ✅ |
| `"5 năm"` | `["E","F"]` | ✅ |
| `"Hơn 5 năm"` | `["F"]` | ✅ |
| `"Không yêu cầu kinh nghiệm"` | `["A","B","C","D","E","F"]` | ✅ |
| `"abc"` | `["A","B","C","D","E","F"]` | ✅ (fallback) |

---

## 7. Field Mapping Tables

### 7.1 API → RawJob (Crawler Stage)

| API Field | RawData Key | Type | Example | Notes |
|-----------|-------------|------|---------|-------|
| `id` | `jobId` | int | `200734388` | Primary identifier |
| `title` | `jobTitle` | string | `"Kỹ Thuật Viên..."` | Job title |
| `title_slug` | `titleSlug` | string | `"ky-thuat-vien-..."` | URL slug |
| `employer_id` | `companyId` | int | `12345` | Company ID |
| `employer_info.name` | `companyName` | string | `"Công Ty CP..."` | Company name |
| `employer_info.logo` | `companyLogo` | string | `"https://cdn..."` | Logo URL |
| `employer_info.rate_response` | `rateResponse` | int | `95` | 0-100% |
| `province_ids` | `provinceIds` | []int | `[1, 8]` | Hà Nội=1, TP.HCM=8 |
| `district_ids` | `districtIds` | []int | `[14, 760]` | See mapping table |
| `contact_address` | `contactAddress` | string | `"Số 38, Ngách..."` | Full address |
| `salary_from` | `salaryFrom` | int | `8000000` | VND |
| `salary_to` | `salaryTo` | int | `15000000` | VND |
| `salary_text` | `salaryText` | string | `"8 - 15 triệu"` | Human-readable |
| `salary_unit` | `salaryUnit` | int | `1` | 1=VND, 2=USD |
| `experience_range` | `experienceRange` | int | `3` | 1-5 (see enum) |
| `working_method` | `workingMethod` | int | `1` | 1-3 (see enum) |
| `level_requirement` | `levelRequirement` | int | `1` | 1-5 (see enum) |
| `degree_requirement` | `degreeRequirement` | int | `3` | 1-5 (see enum) |
| `gender` | `gender` | int | `0` | 0=Không YC, 1=Nam, 2=Nữ |
| `job_requirement_html` | `jobRequirement` | string | `"<p>HTML</p>"` | Rich HTML |
| `other_requirement_html` | `otherRequirement` | string | `"<p>HTML</p>"` | Rich HTML |
| `total_views` | `totalViews` | int | `150` | View count |
| `total_resume_applied` | `totalResumeApplied` | int | `20` | Application count |
| `created_at` | `createdAt` | int64 | `1735689600` | Unix timestamp |
| `updated_at` | - | int64 | `1735776000` | → `LastUpdatedOn` |
| `approved_at` | `approvedAt` | int64 | `1735776000` | Unix timestamp |
| `resume_apply_expired` | `expiredAt` | int64 | `1738627199` | → `ExpiredOn` |

### 7.2 JSON-LD → RawData (Enricher Stage)

| JSON-LD Path | RawData Key | Type | Example | Priority |
|--------------|-------------|------|---------|----------|
| `description` | `jobDescription` | string | `"Lắp đặt hệ thống..."` | High |
| `jobBenefits` | `jobBenefits` | string | `"BHXH, BHYT..."` | High |
| `skills` | `skills` | string | `"PLC, SCADA"` | High |
| `qualifications` | `qualifications` | string | `"Cao đẳng"` | High |
| `industry` | `industry` | []string | `["Điện - Điện tử"]` | High (split by comma) |
| `occupationalCategory` | `occupationalCategory` | string | `"Nhân viên"` | Medium |
| `employmentType` | `employmentType` | string | `"FULL_TIME"` | Medium |
| `hiringOrganization.sameAs` | `companyWebsite` | string | `"https://example.com"` | Low |
| `jobLocation[].address.addressRegion` | `locationCity` | []string | `["Hà Nội"]` | High (deduplicated) |
| `jobLocation[].address.addressLocality` | `locationDistrict` | []string | `["Nam Từ Liêm"]` | High (deduplicated) |
| `baseSalary.value.minValue` | `salaryMinJsonLd` | int | `8000000` | **Override API** |
| `baseSalary.value.maxValue` | `salaryMaxJsonLd` | int | `15000000` | **Override API** |
| `baseSalary.value.value` | `salaryTextJsonLd` | string | `"Thỏa thuận"` | High |
| `baseSalary.currency` | `salaryCurrency` | string | `"VND"` | Low |
| `identifier.value` | - | string | `"200734388"` | Validation only |
| HTML `div` parsing | `experienceText` | string | `"1 năm"` | **Override API** |
| `<link rel="canonical">` | `canonicalUrl` | string | `"https://..."` | Low |

### 7.3 RawData → domain.Job (Worker Stage)

| RawData Key(s) | domain.Job Field | Transformation | Example |
|----------------|------------------|----------------|---------|
| `jobId` | `ID` | `"vieclam24h-{id}"` | `"vieclam24h-200734388"` |
| - | `Source` | Constant | `"vieclam24h"` |
| `jobUrl` or `canonicalUrl` | `SourceURL` | Direct | `"https://vieclam24h.vn/..."` |
| `jobTitle` | `Title` | Direct | `"Kỹ Thuật Viên..."` |
| `companyName` | `Company` | Direct | `"Công Ty CP..."` |
| `contactAddress` | `Location` | Direct | `"Số 38, Ngách 49..."` |
| `occupationalCategory` | `Position` | Direct | `"Nhân viên"` |
| `workingMethod` | `WorkType` | Enum map | `1 → "Toàn thời gian"` |
| `locationCity` | `LocationCity` | Direct array | `["Hà Nội", "TP.HCM"]` |
| `locationDistrict` | `LocationDistrict` | Direct array | `["Nam Từ Liêm"]` |
| `salaryMinJsonLd` OR `salaryFrom` | `SalaryMin` | Priority JSON-LD, ÷1M | `8000000 → 8` |
| `salaryMaxJsonLd` OR `salaryTo` | `SalaryMax` | Priority JSON-LD, ÷1M | `15000000 → 15` |
| `isNegotiable` | `IsNegotiable` | Boolean | `true` |
| Computed | `Salary` | Generated | `"8 - 15 triệu"` or `"Thỏa thuận"` |
| `experienceText` | `Experience` | Direct | `"1 năm"` |
| `experienceText` | `ExpTags` | `mapExperienceToTags()` | `["C","D","E","F"]` |
| `industry` | `Industry` | Direct array | `["Điện - Điện tử"]` |
| `qualifications` | `Qualifications` | Default: "Không yêu cầu" | `"Cao đẳng"` |
| `skills` | `Skills` | Split by comma | `["PLC", "SCADA"]` |
| `jobDescription` OR `jobRequirement` | `Description` | `cleanHTML()` | Plain text |
| `jobRequirement` | `Requirements` | `cleanHTML()` | Plain text |
| `jobBenefits` | `Benefits` | `cleanHTML()` | Plain text |
| `totalViews` | `TotalViews` | Direct | `150` |
| `totalResumeApplied` | `TotalResumeApplied` | Direct | `20` |
| `rateResponse` | `RateResponse` | Direct | `95.0` |
| `companyWebsite` | `CompanyWebsite` | Direct | `"https://..."` |
| `employmentType` | `EmploymentType` | Direct | `"FULL_TIME"` |
| `expiredAt` | `ExpiredAt` | Unix → `time.Time` | `time.Unix(1738627199, 0)` |
| - | `CrawledAt` | `time.Now()` | Current time |

---

## 8. Enum Value Mappings

### 8.1 Working Method (working_method)

| Value | Text (Vietnamese) | Text (English) |
|-------|-------------------|----------------|
| `1` | Toàn thời gian | Full-time |
| `2` | Bán thời gian | Part-time |
| `3` | Thực tập | Internship |

**Mapping Code:**
```go
func mapWorkingMethod(method int) string {
    switch method {
    case 1: return "Toàn thời gian"
    case 2: return "Bán thời gian"
    case 3: return "Thực tập"
    default: return "Không xác định"
    }
}
```

### 8.2 Level Requirement (level_requirement)

| Value | Vietnamese | English |
|-------|------------|---------|
| `1` | Nhân viên | Staff |
| `2` | Trưởng nhóm | Team Lead |
| `3` | Trưởng/Phó phòng | Manager/Deputy |
| `4` | Giám đốc và cấp cao hơn | Director+ |
| `5` | Khác | Other |

### 8.3 Degree Requirement (degree_requirement)

| Value | Vietnamese | English |
|-------|------------|---------|
| `1` | Không yêu cầu | No requirement |
| `2` | Trung cấp | Vocational |
| `3` | Cao đẳng | College |
| `4` | Đại học | University |
| `5` | Sau đại học | Postgraduate |

### 8.4 Experience Range (experience_range)

| Value | Vietnamese | Mapped Tags |
|-------|------------|-------------|
| `1` | Chưa có kinh nghiệm | A, B, C, D, E, F |
| `2` | Dưới 1 năm | B, C, D, E, F |
| `3` | 1 năm | C, D, E, F |
| `4` | 2 năm | D, E, F |
| `5` | 3 năm | E, F |
| `6` | 4 năm | E, F |
| `7` | 5 năm | E, F |
| `8` | Hơn 5 năm | F |

**Note:** HTML parsing (`experienceText`) takes priority over this enum.

### 8.5 Gender (gender)

| Value | Vietnamese | English |
|-------|------------|---------|
| `0` | Không yêu cầu | No requirement |
| `1` | Nam | Male |
| `2` | Nữ | Female |

### 8.6 Province IDs (Common)

| ID | Province/City |
|----|---------------|
| `1` | Hà Nội |
| `2` | Hồ Chí Minh |
| `24` | Đà Nẵng |
| `4` | Hải Phòng |
| `22` | Cần Thơ |
| `6` | Bắc Ninh |
| `8` | Bình Dương |
| `10` | Đồng Nai |

**Full Mapping:** See [provinces.json](#) (174 entries)

### 8.7 Salary Unit (salary_unit)

| Value | Currency |
|-------|----------|
| `1` | VND |
| `2` | USD |

---

## 9. Elasticsearch Index

### 9.1 Index Settings

```json
{
  "settings": {
    "number_of_shards": 3,
    "number_of_replicas": 1,
    "refresh_interval": "5s",
    "max_result_window": 100000,

    "analysis": {
      "analyzer": {
        "vietnamese_analyzer": {
          "type": "custom",
          "tokenizer": "standard",
          "filter": [
            "lowercase",
            "asciifolding",
            "vietnamese_stop",
            "vietnamese_stemmer"
          ]
        }
      },
      "filter": {
        "vietnamese_stop": {
          "type": "stop",
          "stopwords": ["và", "của", "có", "được", "cho", "các"]
        },
        "vietnamese_stemmer": {
          "type": "stemmer",
          "language": "light_english"
        }
      }
    }
  }
}
```

### 9.2 Index Mapping

```json
{
  "mappings": {
    "properties": {
      "id": {
        "type": "keyword"
      },
      "title": {
        "type": "text",
        "analyzer": "vietnamese_analyzer",
        "fields": {
          "keyword": {"type": "keyword"},
          "raw": {"type": "text", "analyzer": "standard"}
        }
      },
      "company": {
        "type": "text",
        "analyzer": "vietnamese_analyzer",
        "fields": {
          "keyword": {"type": "keyword"}
        }
      },
      "location": {
        "type": "text",
        "analyzer": "vietnamese_analyzer"
      },
      "location_city": {
        "type": "keyword"
      },
      "location_district": {
        "type": "keyword"
      },
      "position": {
        "type": "keyword"
      },
      "salary": {
        "type": "text"
      },
      "salary_min": {
        "type": "integer"
      },
      "salary_max": {
        "type": "integer"
      },
      "is_negotiable": {
        "type": "boolean"
      },
      "work_type": {
        "type": "keyword"
      },
      "industry": {
        "type": "keyword"
      },
      "field": {
        "type": "keyword"
      },
      "experience": {
        "type": "keyword"
      },
      "experience_tags": {
        "type": "keyword"
      },
      "qualifications": {
        "type": "keyword"
      },
      "description": {
        "type": "text",
        "analyzer": "vietnamese_analyzer"
      },
      "requirements": {
        "type": "text",
        "analyzer": "vietnamese_analyzer"
      },
      "benefits": {
        "type": "text",
        "analyzer": "vietnamese_analyzer"
      },
      "skills": {
        "type": "keyword"
      },
      "source": {
        "type": "keyword"
      },
      "source_url": {
        "type": "keyword",
        "index": false
      },
      "total_views": {
        "type": "integer"
      },
      "total_resume_applied": {
        "type": "integer"
      },
      "rate_response": {
        "type": "float"
      },
      "company_website": {
        "type": "keyword",
        "index": false
      },
      "occupational_category": {
        "type": "keyword"
      },
      "employment_type": {
        "type": "keyword"
      },
      "expired_at": {
        "type": "date"
      },
      "crawled_at": {
        "type": "date"
      }
    }
  }
}
```

### 9.3 Sample Document

```json
{
  "id": "vieclam24h-200734388",
  "title": "Kỹ Thuật Viên Lắp Đặt Hệ Thống Quan Trắc Tự Động",
  "company": "Công Ty CP Công Nghệ Môi Trường Vietstar",
  "location": "Số 38, Ngách 49, Ngõ 63, Đại Mỗ, Nam Từ Liêm, Hà Nội",
  "location_city": ["Hà Nội"],
  "location_district": ["Nam Từ Liêm"],
  "position": "Nhân viên",
  "salary": "8 - 15 triệu",
  "salary_min": 8,
  "salary_max": 15,
  "is_negotiable": false,
  "work_type": "Toàn thời gian",
  "industry": ["Vận hành - Bảo trì", "Điện - Điện tử"],
  "field": "Điện - Điện tử",
  "experience": "1 năm",
  "experience_tags": ["C", "D", "E", "F"],
  "qualifications": "Cao đẳng",
  "description": "Lắp đặt, vận hành và bảo trì hệ thống quan trắc môi trường tự động...",
  "requirements": "Tốt nghiệp Đại học hoặc Cao đẳng chuyên ngành Điện tử, Điện tự động hóa...",
  "benefits": "BHXH, BHYT, BHTN theo quy định nhà nước. Nghỉ phép năm 12 ngày...",
  "skills": ["PLC", "SCADA", "AutoCAD"],
  "source": "vieclam24h",
  "source_url": "https://vieclam24h.vn/viec-lam/ky-thuat-vien-lap-dat-he-thong-quan-trac-tu-dong-i200734388.html",
  "total_views": 150,
  "total_resume_applied": 20,
  "rate_response": 95.0,
  "company_website": "https://vietstar.com.vn",
  "occupational_category": "Nhân viên",
  "employment_type": "FULL_TIME",
  "expired_at": "2026-02-03T23:59:59Z",
  "crawled_at": "2026-01-08T03:00:00Z"
}
```

---

## 10. Queue Architecture

### 10.1 Two-Queue System

```mermaid
flowchart LR
    subgraph Crawler["📦 Crawler"]
        A["Parse API"]
        B["Dedup Check"]
        C["Build RawJob"]
    end

    subgraph Q1["📋 Queue 1: Pending"]
        D[("jobs:pending:vieclam24h<br/><br/>Type: List (LPUSH/BRPOP)<br/>Content: RawJob (API only)<br/>Size: ~13,500 jobs<br/>Payload: ~5KB/job")]
    end

    subgraph Enricher["🔧 Enricher"]
        E["BRPOP (5s timeout)"]
        F["Fetch HTML"]
        G["Parse JSON-LD"]
        H["Merge Data"]
    end

    subgraph Q2["📋 Queue 2: Raw"]
        I[("jobs:raw:vieclam24h<br/><br/>Type: List (LPUSH/BRPOP)<br/>Content: Enriched RawJob<br/>Size: ~13,500 jobs<br/>Payload: ~15KB/job")]
    end

    subgraph Worker["📊 Worker"]
        J["BRPOP (5s timeout)"]
        K["Normalize"]
        L["Batch (50 jobs)"]
        M["Bulk Index"]
    end

    A --> B --> C --> D
    D --> E --> F --> G --> H --> I
    I --> J --> K --> L --> M

    style D fill:#fff9c4
    style I fill:#c5e1a5
```

### 10.2 Queue Details

#### Queue 1: Pending

**Name:** `jobs:pending:vieclam24h`

**Purpose:** Store raw API data before HTML enrichment

**Operations:**
```redis
# Crawler pushes
LPUSH jobs:pending:vieclam24h '{"id":"vieclam24h-123","url":"...","raw_data":{...}}'

# Enricher pops (blocking, 5s timeout)
BRPOP jobs:pending:vieclam24h 5

# Check queue size
LLEN jobs:pending:vieclam24h
```

**Characteristics:**
- **Size:** ~13,500 jobs (full crawl)
- **Payload:** ~5KB per job (JSON)
- **TTL:** None (processed immediately)
- **Retention:** Deleted after BRPOP

#### Queue 2: Raw

**Name:** `jobs:raw:vieclam24h`

**Purpose:** Store enriched data before normalization

**Operations:**
```redis
# Enricher pushes
LPUSH jobs:raw:vieclam24h '{"id":"vieclam24h-123","url":"...","raw_data":{...enriched...}}'

# Worker pops (blocking, 5s timeout)
BRPOP jobs:raw:vieclam24h 5

# Check queue size
LLEN jobs:raw:vieclam24h
```

**Characteristics:**
- **Size:** ~13,500 jobs (full crawl)
- **Payload:** ~15KB per job (JSON + HTML metadata)
- **TTL:** None (processed immediately)
- **Retention:** Deleted after BRPOP

### 10.3 Queue Flow Timing

```mermaid
gantt
    title Full Crawl Timeline (13,500 jobs)
    dateFormat HH:mm
    axisFormat %H:%M

    section Crawler
    API Fetch (450 pages × 2s)   :00:00, 15m
    Dedup & Queue Push           :00:00, 20m

    section Enricher
    HTML Scrape (5s/job × 13,500) :00:05, 18h

    section Worker
    Normalize & Index             :00:10, 2h
```

**Bottleneck:** HTML scraping (5s delay × 13,500 = ~18 hours)

**Optimization:** Run multiple enricher instances in parallel

### 10.4 Parallel Enricher Setup

```bash
# Run 5 enricher instances
docker-compose up --scale enricher=5

# Each instance processes ~2,700 jobs
# Total time: 18h ÷ 5 = ~3.6 hours
```

---

## 11. Error Handling & Retry

### 11.1 Error Classification

| Error Type | Stage | Severity | Retry Strategy |
|------------|-------|----------|----------------|
| **API Timeout** | Crawler | Medium | Exponential backoff (3 retries) |
| **API 5xx** | Crawler | High | Exponential backoff (5 retries) |
| **API 4xx** | Crawler | Low | Skip page, log warning |
| **HTML Fetch 404** | Enricher | Low | Use API data only, continue |
| **HTML Parse Error** | Enricher | Low | Use API data only, continue |
| **JSON-LD Invalid** | Enricher | Low | Ignore JSON-LD, use API fallback |
| **Redis Connection** | All | Critical | Retry forever with backoff |
| **ES Connection** | Worker | Critical | Retry forever with backoff |
| **ES Bulk Failure** | Worker | High | Retry batch 3 times, then skip |
| **Dedup Redis Down** | Crawler | Medium | Fail-open (process all jobs) |

### 11.2 Retry Implementation

#### Exponential Backoff

```go
func retryWithBackoff(ctx context.Context, maxRetries int, fn func() error) error {
    var err error
    for i := 0; i < maxRetries; i++ {
        err = fn()
        if err == nil {
            return nil
        }

        // Exponential backoff: 1s, 2s, 4s, 8s, 16s
        backoff := time.Duration(1<<uint(i)) * time.Second

        log.Printf("Retry %d/%d after %v: %v", i+1, maxRetries, backoff, err)

        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(backoff):
        }
    }
    return fmt.Errorf("max retries exceeded: %w", err)
}
```

#### Example Usage (API Fetch)

```go
var resp *APIResponse

err := retryWithBackoff(ctx, 3, func() error {
    var fetchErr error
    resp, fetchErr = fetchAPIPage(ctx, page)
    return fetchErr
})

if err != nil {
    log.Printf("Failed to fetch page %d after retries: %v", page, err)
    return err
}
```

### 11.3 Circuit Breaker (HTML Fetch)

```go
type CircuitBreaker struct {
    maxFailures int
    failures    int
    state       string // "closed", "open", "half-open"
    resetTime   time.Time
}

func (cb *CircuitBreaker) Call(fn func() error) error {
    if cb.state == "open" {
        if time.Now().Before(cb.resetTime) {
            return errors.New("circuit breaker open")
        }
        cb.state = "half-open"
    }

    err := fn()

    if err != nil {
        cb.failures++
        if cb.failures >= cb.maxFailures {
            cb.state = "open"
            cb.resetTime = time.Now().Add(1 * time.Minute)
        }
        return err
    }

    cb.failures = 0
    cb.state = "closed"
    return nil
}
```

### 11.4 Dead Letter Queue

**Purpose:** Store jobs that fail after all retries

**Implementation:**
```go
const deadLetterQueue = "jobs:dead:vieclam24h"

func handleFatalError(ctx context.Context, job *domain.RawJob, err error) {
    // Add error metadata
    job.RawData["_error"] = err.Error()
    job.RawData["_failedAt"] = time.Now().Format(time.RFC3339)

    // Push to DLQ
    redis.LPush(ctx, deadLetterQueue, job)

    log.Printf("Job %s moved to DLQ: %v", job.ID, err)
}
```

**Monitoring DLQ:**
```bash
# Check DLQ size
redis-cli LLEN jobs:dead:vieclam24h

# Inspect failed jobs
redis-cli LRANGE jobs:dead:vieclam24h 0 10

# Reprocess DLQ
while true; do
  redis-cli RPOPLPUSH jobs:dead:vieclam24h jobs:pending:vieclam24h
done
```

---

## 12. Performance & Scalability

### 12.1 Current Performance

| Metric | Value | Notes |
|--------|-------|-------|
| **Crawler Throughput** | 450 pages/15min | ~30 pages/min |
| **Enricher Throughput** | 720 jobs/hour | 1 instance, 5s delay |
| **Worker Throughput** | 6,750 jobs/hour | Batch size 50 |
| **Full Crawl Time** | ~18 hours | Bottleneck: Enricher |
| **ES Index Size** | ~500MB | 13,500 jobs |
| **Redis Memory** | ~200MB | Queues + Dedup |

### 12.2 Horizontal Scaling

#### Crawler

```bash
# NOT RECOMMENDED: Multiple crawlers cause duplicate work
# Stick to 1 crawler instance
docker-compose up crawler --scale crawler=1
```

**Reason:** Crawler state (current page) is not shared

#### Enricher

```bash
# RECOMMENDED: Scale enricher for faster HTML fetch
docker-compose up enricher --scale enricher=10

# 10 instances × 720 jobs/hour = 7,200 jobs/hour
# Full crawl time: 13,500 ÷ 7,200 = ~2 hours
```

**Best Practice:**
- Run 5-10 enricher instances
- Monitor rate limits on vieclam24h.vn
- Use distributed rate limiting (Redis)

#### Worker

```bash
# Scale worker for faster indexing
docker-compose up worker --scale worker=5

# 5 instances × 6,750 jobs/hour = 33,750 jobs/hour
```

**Best Practice:**
- Scale based on ES cluster capacity
- Monitor ES bulk queue size
- Use bulk API for efficiency

### 12.3 Vertical Scaling

#### Redis

```yaml
# Increase maxmemory for large queues
redis:
  image: redis:7-alpine
  command: redis-server --maxmemory 2gb --maxmemory-policy allkeys-lru
```

#### Elasticsearch

```yaml
# Increase heap size for large indices
elasticsearch:
  environment:
    - "ES_JAVA_OPTS=-Xms2g -Xmx2g"
  deploy:
    resources:
      limits:
        memory: 4g
```

### 12.4 Optimization Techniques

#### Batch Processing

```go
// Worker bulk indexing
const batchSize = 50

var batch []*domain.Job
for job := range jobs {
    batch = append(batch, normalize(job))

    if len(batch) >= batchSize {
        bulkIndex(batch)
        batch = batch[:0]
    }
}
```

#### Connection Pooling

```go
// HTTP client with connection pooling
client := &http.Client{
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     30 * time.Second,
    },
    Timeout: 30 * time.Second,
}
```

#### Redis Pipeline

```go
// Batch Redis operations
pipe := redis.Pipeline()
for i := 0; i < 1000; i++ {
    pipe.Set(ctx, fmt.Sprintf("key:%d", i), value, ttl)
}
pipe.Exec(ctx)
```

---

## 13. Monitoring & Metrics

### 13.1 Key Metrics

#### Crawler

```prometheus
# Counters
crawler_pages_fetched_total{source="vieclam24h"}
crawler_jobs_discovered_total{source="vieclam24h"}
crawler_dedup_new_total{source="vieclam24h"}
crawler_dedup_updated_total{source="vieclam24h"}
crawler_dedup_unchanged_total{source="vieclam24h"}
crawler_errors_total{source="vieclam24h", type="api_timeout"}

# Gauges
crawler_current_page{source="vieclam24h"}
crawler_queue_size{queue="pending"}
```

#### Enricher

```prometheus
# Counters
enricher_jobs_processed_total{source="vieclam24h"}
enricher_html_fetch_success_total{source="vieclam24h"}
enricher_html_fetch_errors_total{source="vieclam24h"}
enricher_jsonld_extracted_total{source="vieclam24h"}

# Histograms
enricher_html_fetch_duration_seconds{source="vieclam24h"}
enricher_processing_duration_seconds{source="vieclam24h"}

# Gauges
enricher_queue_size{queue="pending"}
```

#### Worker

```prometheus
# Counters
worker_jobs_normalized_total{source="vieclam24h"}
worker_es_bulk_success_total{source="vieclam24h"}
worker_es_bulk_errors_total{source="vieclam24h"}

# Histograms
worker_normalization_duration_seconds{source="vieclam24h"}
worker_es_bulk_duration_seconds{source="vieclam24h"}

# Gauges
worker_queue_size{queue="raw"}
worker_batch_size{source="vieclam24h"}
```

### 13.2 Grafana Dashboard

```json
{
  "dashboard": {
    "title": "Vieclam24h Pipeline",
    "panels": [
      {
        "title": "Crawl Progress",
        "targets": [
          {
            "expr": "crawler_current_page{source='vieclam24h'}"
          }
        ]
      },
      {
        "title": "Dedup Distribution",
        "targets": [
          {
            "expr": "rate(crawler_dedup_new_total[5m])",
            "legendFormat": "New"
          },
          {
            "expr": "rate(crawler_dedup_updated_total[5m])",
            "legendFormat": "Updated"
          },
          {
            "expr": "rate(crawler_dedup_unchanged_total[5m])",
            "legendFormat": "Unchanged"
          }
        ]
      },
      {
        "title": "Queue Sizes",
        "targets": [
          {
            "expr": "crawler_queue_size{queue='pending'}",
            "legendFormat": "Pending"
          },
          {
            "expr": "worker_queue_size{queue='raw'}",
            "legendFormat": "Raw"
          }
        ]
      },
      {
        "title": "HTML Fetch Duration (p95)",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, enricher_html_fetch_duration_seconds_bucket)"
          }
        ]
      },
      {
        "title": "ES Indexing Rate",
        "targets": [
          {
            "expr": "rate(worker_es_bulk_success_total[1m]) * 50"
          }
        ]
      }
    ]
  }
}
```

### 13.3 Alerts

```yaml
groups:
  - name: vieclam24h
    rules:
      # Crawler stuck
      - alert: CrawlerStuck
        expr: rate(crawler_jobs_discovered_total[5m]) == 0
        for: 10m
        annotations:
          summary: "Crawler not discovering new jobs"

      # Queue backlog
      - alert: QueueBacklog
        expr: crawler_queue_size{queue="pending"} > 10000
        for: 30m
        annotations:
          summary: "Pending queue has large backlog"

      # High error rate
      - alert: HighErrorRate
        expr: rate(enricher_html_fetch_errors_total[5m]) > 0.1
        for: 5m
        annotations:
          summary: "Enricher HTML fetch error rate > 10%"

      # ES indexing slow
      - alert: SlowIndexing
        expr: rate(worker_es_bulk_success_total[5m]) < 10
        for: 10m
        annotations:
          summary: "ES indexing rate < 10 bulks/min"
```

---

## 14. Troubleshooting Guide

### 14.1 Common Issues

#### Issue 1: Crawler not finding jobs

**Symptoms:**
```bash
curl localhost:9200/jobs_vieclam24h/_count
# Returns: {"count": 0}
```

**Diagnosis:**
```bash
# Check crawler logs
docker logs go-crawler-crawler-1

# Check API response
curl -H "Authorization: Bearer ..." \
  "https://api-job.vieclam24h.vn/api/job/get-job-list?page=1&per_page=1&channel_code=vl24h"
```

**Solutions:**
1. Verify bearer token is valid
2. Check API endpoint is accessible
3. Verify `channel_code` and `branch` parameters
4. Check Redis connection

#### Issue 2: Enricher stuck

**Symptoms:**
```bash
redis-cli LLEN jobs:pending:vieclam24h
# Returns: 13500 (not decreasing)
```

**Diagnosis:**
```bash
# Check enricher logs
docker logs go-crawler-enricher-1

# Test HTML fetch manually
curl -I https://vieclam24h.vn/viec-lam/test-i200734388.html
```

**Solutions:**
1. Check rate limiting (429 responses)
2. Increase `CRAWLER_DELAY_MS` if rate limited
3. Scale enricher instances: `docker-compose up --scale enricher=5`
4. Verify HTML structure hasn't changed

#### Issue 3: ES indexing failures

**Symptoms:**
```bash
# Worker logs show bulk errors
2026-01-08 03:00:00 [ERROR] Bulk index failed: 413 Request Entity Too Large
```

**Diagnosis:**
```bash
# Check ES cluster health
curl localhost:9200/_cluster/health

# Check bulk queue
curl localhost:9200/_nodes/stats/thread_pool?filter_path=**.bulk
```

**Solutions:**
1. Reduce `WORKER_BATCH_SIZE` from 50 to 25
2. Increase ES `http.max_content_length`
3. Check ES disk space
4. Reduce payload size (strip HTML)

#### Issue 4: Dedup not working

**Symptoms:**
```bash
# Same jobs indexed multiple times
curl localhost:9200/jobs_vieclam24h/_search?q=id:vieclam24h-200734388
# Returns: 2 hits (duplicate)
```

**Diagnosis:**
```bash
# Check Redis dedup keys
redis-cli KEYS job:seen:vieclam24h:*

# Check specific job
redis-cli GET job:seen:vieclam24h:200734388
```

**Solutions:**
1. Verify Redis connection in crawler
2. Check `updated_at` timestamp format
3. Ensure crawler isn't running multiple instances
4. Clear dedup keys: `redis-cli DEL job:seen:vieclam24h:*`

### 14.2 Health Checks

#### Crawler Health

```bash
# Check if crawler is running
docker ps | grep crawler

# Check last crawled page
redis-cli GET crawler:last_page:vieclam24h

# Check queue growth
watch -n 1 'redis-cli LLEN jobs:pending:vieclam24h'
```

#### Enricher Health

```bash
# Check enricher instances
docker ps | grep enricher

# Monitor pending queue drain rate
watch -n 1 'redis-cli LLEN jobs:pending:vieclam24h'

# Check raw queue growth
watch -n 1 'redis-cli LLEN jobs:raw:vieclam24h'
```

#### Worker Health

```bash
# Check ES index count growth
watch -n 1 'curl -s localhost:9200/jobs_vieclam24h/_count | jq .count'

# Check ES indexing rate
curl -s localhost:9200/_nodes/stats/indices | jq '.nodes[].indices.indexing.index_total'
```

### 14.3 Debug Commands

```bash
# Inspect queue job
redis-cli LINDEX jobs:pending:vieclam24h 0 | jq .

# Search ES by ID
curl localhost:9200/jobs_vieclam24h/_search -d '{"query":{"term":{"id":"vieclam24h-200734388"}}}'

# View dedup key TTL
redis-cli TTL job:seen:vieclam24h:200734388

# Count dedup keys
redis-cli KEYS job:seen:vieclam24h:* | wc -l

# Check ES mapping
curl localhost:9200/jobs_vieclam24h/_mapping | jq .

# Force refresh ES index
curl -X POST localhost:9200/jobs_vieclam24h/_refresh
```

---

## 15. Configuration Reference

### 15.1 Environment Variables

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `REDIS_ADDR` | `redis:6379` | Redis connection | `localhost:6379` |
| `REDIS_PASSWORD` | `""` | Redis password | `secret123` |
| `REDIS_DB` | `0` | Redis database | `0` |
| `ELASTICSEARCH_URL` | `http://elasticsearch:9200` | ES endpoint | `http://localhost:9200` |
| `ELASTICSEARCH_INDEX` | `jobs_vieclam24h` | Index name | `jobs_vieclam24h_v2` |
| `ELASTICSEARCH_USERNAME` | `""` | ES username | `elastic` |
| `ELASTICSEARCH_PASSWORD` | `""` | ES password | `changeme` |
| `REDIS_JOB_QUEUE_PENDING` | `jobs:pending:vieclam24h` | Pending queue | Custom name |
| `REDIS_JOB_QUEUE_RAW` | `jobs:raw:vieclam24h` | Raw queue | Custom name |
| `VIECLAM24H_MAX_PAGES` | `999` | Max pages to crawl | `100` (for testing) |
| `VIECLAM24H_PER_PAGE` | `30` | Items per page | `30` |
| `VIECLAM24H_REQUEST_DELAY` | `2s` | Crawler delay | `5s` |
| `VIECLAM24H_BEARER_TOKEN` | (see below) | API auth token | - |
| `VIECLAM24H_CHANNEL_CODE` | `vl24h` | Channel code | `vl24h` |
| `VIECLAM24H_BRANCH` | `vl24h.north` | Branch filter | `vl24h.south` |
| `CRAWLER_DELAY_MS` | `5000` | Enricher delay (ms) | `2000` |
| `WORKER_CONCURRENCY` | `5` | Worker goroutines | `10` |
| `WORKER_BATCH_SIZE` | `50` | ES bulk size | `100` |
| `LOG_LEVEL` | `info` | Log level | `debug` |

### 15.2 Bearer Token

```
eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJjaGFubmVsX2NvZGUiOiJ2bDI0aCIsInVzZXIiOm51bGx9.a0POm2ZVRwetYs2QsMj0sRg8lZSSbKufX4sewqhAM5o
```

**Decoded Payload:**
```json
{
  "channel_code": "vl24h",
  "user": null
}
```

**Expiration:** None (long-lived token)

### 15.3 Docker Compose Config

```yaml
version: '3.8'

services:
  redis:
    image: redis:7-alpine
    command: redis-server --maxmemory 2gb --maxmemory-policy allkeys-lru
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data

  elasticsearch:
    image: docker.elastic.co/elasticsearch/elasticsearch:8.11.0
    environment:
      - discovery.type=single-node
      - "ES_JAVA_OPTS=-Xms2g -Xmx2g"
      - xpack.security.enabled=false
    ports:
      - "9200:9200"
    volumes:
      - es_data:/usr/share/elasticsearch/data

  crawler:
    build: .
    command: /app/bin/vieclam24h-crawler
    environment:
      - REDIS_ADDR=redis:6379
      - VIECLAM24H_MAX_PAGES=999
      - VIECLAM24H_PER_PAGE=30
      - VIECLAM24H_REQUEST_DELAY=2s
    depends_on:
      - redis

  enricher:
    build: .
    command: /app/bin/vieclam24h-enricher
    environment:
      - REDIS_ADDR=redis:6379
      - CRAWLER_DELAY_MS=5000
    depends_on:
      - redis
    deploy:
      replicas: 5  # Run 5 instances

  worker:
    build: .
    command: /app/bin/worker
    environment:
      - REDIS_ADDR=redis:6379
      - ELASTICSEARCH_URL=http://elasticsearch:9200
      - ELASTICSEARCH_INDEX=jobs_vieclam24h
      - WORKER_BATCH_SIZE=50
      - WORKER_CONCURRENCY=5
    depends_on:
      - redis
      - elasticsearch

volumes:
  redis_data:
  es_data:
```

---

## 16. Sample Queries

### 16.1 Basic Search

```json
GET /jobs_vieclam24h/_search
{
  "query": {
    "match": {
      "title": "kỹ thuật viên"
    }
  }
}
```

### 16.2 Filter by Location

```json
GET /jobs_vieclam24h/_search
{
  "query": {
    "bool": {
      "must": [
        {"match": {"title": "kỹ thuật"}},
        {"term": {"location_city": "Hà Nội"}}
      ]
    }
  }
}
```

### 16.3 Filter by Experience Tag

```json
GET /jobs_vieclam24h/_search
{
  "query": {
    "bool": {
      "must": [
        {"term": {"experience_tags": "D"}}
      ]
    }
  },
  "sort": [
    {"crawled_at": "desc"}
  ]
}
```

### 16.4 Salary Range

```json
GET /jobs_vieclam24h/_search
{
  "query": {
    "bool": {
      "filter": [
        {"range": {"salary_min": {"gte": 10}}},
        {"range": {"salary_max": {"lte": 20}}}
      ]
    }
  }
}
```

### 16.5 Aggregations

```json
GET /jobs_vieclam24h/_search
{
  "size": 0,
  "aggs": {
    "by_city": {
      "terms": {
        "field": "location_city",
        "size": 10
      }
    },
    "by_industry": {
      "terms": {
        "field": "industry",
        "size": 20
      }
    },
    "salary_stats": {
      "stats": {
        "field": "salary_min"
      }
    }
  }
}
```

---

## 17. Commands

### 17.1 Docker Commands

```bash
# Start all services
docker-compose up -d

# Stop all services
docker-compose down

# Reset (delete all data)
docker-compose down -v
docker volume rm go-crawler_redis_data go-crawler_es_data

# View logs
docker-compose logs -f
docker-compose logs -f crawler
docker-compose logs -f enricher
docker-compose logs -f worker

# Scale enricher
docker-compose up -d --scale enricher=10

# Restart specific service
docker-compose restart crawler

# Build images
docker-compose build
```

### 17.2 Redis Commands

```bash
# Redis CLI
docker exec -it go-crawler-redis-1 redis-cli

# Check queue sizes
redis-cli LLEN jobs:pending:vieclam24h
redis-cli LLEN jobs:raw:vieclam24h

# Count dedup keys
redis-cli KEYS job:seen:vieclam24h:* | wc -l

# Inspect job
redis-cli LINDEX jobs:pending:vieclam24h 0

# Clear queue
redis-cli DEL jobs:pending:vieclam24h
redis-cli DEL jobs:raw:vieclam24h

# Clear dedup
redis-cli EVAL "return redis.call('del', unpack(redis.call('keys', 'job:seen:vieclam24h:*')))" 0

# Monitor commands
redis-cli MONITOR
```

### 17.3 Elasticsearch Commands

```bash
# Cluster health
curl localhost:9200/_cluster/health?pretty

# Index stats
curl localhost:9200/jobs_vieclam24h/_stats?pretty

# Count documents
curl localhost:9200/jobs_vieclam24h/_count

# Sample documents
curl localhost:9200/jobs_vieclam24h/_search?size=5&pretty

# View mapping
curl localhost:9200/jobs_vieclam24h/_mapping?pretty

# Delete index
curl -X DELETE localhost:9200/jobs_vieclam24h

# Create index with mapping
curl -X PUT localhost:9200/jobs_vieclam24h -H 'Content-Type: application/json' -d @mapping.json

# Force refresh
curl -X POST localhost:9200/jobs_vieclam24h/_refresh

# Search by ID
curl localhost:9200/jobs_vieclam24h/_doc/vieclam24h-200734388?pretty
```

### 17.4 Makefile Commands

```makefile
.PHONY: up down reset status logs count sample

up:
	docker-compose up -d

down:
	docker-compose down

reset:
	docker-compose down -v
	docker volume prune -f

status:
	@echo "=== Docker Services ==="
	@docker-compose ps
	@echo ""
	@echo "=== Queue Sizes ==="
	@docker exec go-crawler-redis-1 redis-cli LLEN jobs:pending:vieclam24h | awk '{print "Pending: " $$1}'
	@docker exec go-crawler-redis-1 redis-cli LLEN jobs:raw:vieclam24h | awk '{print "Raw: " $$1}'
	@echo ""
	@echo "=== ES Job Count ==="
	@curl -s localhost:9200/jobs_vieclam24h/_count | jq -r '.count' | awk '{print "Indexed: " $$1}'

logs:
	docker-compose logs -f

logs-crawler:
	docker-compose logs -f crawler

logs-enricher:
	docker-compose logs -f enricher

logs-worker:
	docker-compose logs -f worker

count:
	curl -s localhost:9200/jobs_vieclam24h/_count | jq .

sample:
	curl -s localhost:9200/jobs_vieclam24h/_search?size=3 | jq '.hits.hits[]._source'

queues:
	@echo "Pending: $$(docker exec go-crawler-redis-1 redis-cli LLEN jobs:pending:vieclam24h)"
	@echo "Raw: $$(docker exec go-crawler-redis-1 redis-cli LLEN jobs:raw:vieclam24h)"

es-health:
	curl -s localhost:9200/_cluster/health | jq .

es-mapping:
	curl -s localhost:9200/jobs_vieclam24h/_mapping | jq .

redis:
	docker exec -it go-crawler-redis-1 redis-cli

build:
	docker-compose build

test:
	go test ./...
```

**Usage:**
```bash
make up
make status
make logs-crawler
make count
make queues
```

---

## Appendix

### A. Glossary

| Term | Definition |
|------|------------|
| **Dedup** | Deduplication - preventing duplicate job processing |
| **Enricher** | Stage 2: HTML scraping and JSON-LD extraction |
| **Experience Tags** | A-F tags for aggregated experience search |
| **JSON-LD** | JSON for Linking Data (Schema.org structured data) |
| **RawJob** | Intermediate data structure before normalization |
| **TTL** | Time To Live - Redis key expiration |

### B. External References

- [Vieclam24h Website](https://vieclam24h.vn)
- [Schema.org JobPosting](https://schema.org/JobPosting)
- [Elasticsearch Documentation](https://www.elastic.co/guide/en/elasticsearch/reference/current/index.html)
- [Redis Documentation](https://redis.io/documentation)
- [goquery Documentation](https://github.com/PuerkitoBio/goquery)

### C. Changelog

| Version | Date | Changes |
|---------|------|---------|
| 2.0 | 2026-01-08 | Complete rewrite with comprehensive documentation |
| 1.0 | 2025-12-01 | Initial version |

---

**End of Documentation**