package vietnamworks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/project-tktt/go-crawler/internal/domain"
	"github.com/project-tktt/go-crawler/internal/module"
)

const (
	// Real API endpoint discovered from browser network analysis
	SearchAPIURL = "https://ms.vietnamworks.com/job-search/v1.0/search"
	JobsPerPage  = 50
)

// Crawler implements job crawling for VietnamWorks
type Crawler struct {
	client *http.Client
	config Config
}

// NewCrawler creates a new VietnamWorks crawler
func NewCrawler(cfg Config) *Crawler {
	if cfg.MaxPages <= 0 {
		cfg.MaxPages = 10
	}
	if cfg.RequestDelay <= 0 {
		cfg.RequestDelay = 2 * time.Second // Base delay, will add random 0-2000ms
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"
	}

	return &Crawler{
		client: &http.Client{Timeout: 30 * time.Second},
		config: cfg,
	}
}

// Crawl fetches job listings from VietnamWorks API
func (c *Crawler) Crawl(ctx context.Context) ([]*domain.RawJob, error) {
	var allJobs []*domain.RawJob
	err := c.CrawlWithCallback(ctx, func(jobs []*domain.RawJob) error {
		allJobs = append(allJobs, jobs...)
		return nil
	})
	return allJobs, err
}

// CrawlWithCallback fetches jobs page by page and calls handler after each page
func (c *Crawler) CrawlWithCallback(ctx context.Context, handler module.JobHandler) error {
	totalJobCount := 0

	for page := 0; page < c.config.MaxPages; page++ {
		log.Printf("[VietnamWorks] Fetching page %d/%d", page+1, c.config.MaxPages)

		jobs, totalPages, err := c.fetchPage(ctx, page)
		if err != nil {
			log.Printf("[VietnamWorks] Error on page %d: %v", page+1, err)
			break
		}

		if len(jobs) == 0 {
			log.Printf("[VietnamWorks] No more jobs on page %d", page+1)
			break
		}

		// Process jobs immediately via callback
		if err := handler(jobs); err != nil {
			log.Printf("[VietnamWorks] Handler error on page %d: %v", page+1, err)
		}

		totalJobCount += len(jobs)
		log.Printf("[VietnamWorks] Page %d: %d jobs processed", page+1, len(jobs))

		// Stop if we've reached the last page
		if page >= totalPages-1 {
			log.Printf("[VietnamWorks] Reached last page (%d)", totalPages)
			break
		}

		// Random delay: base delay + random 0-2000ms
		randomDelay := c.config.RequestDelay + time.Duration(rand.Intn(2000))*time.Millisecond
		time.Sleep(randomDelay)
	}

	log.Printf("[VietnamWorks] Crawled %d jobs total", totalJobCount)
	return nil
}

// fetchPage fetches a single page of jobs
func (c *Crawler) fetchPage(ctx context.Context, page int) ([]*domain.RawJob, int, error) {
	payload := SearchRequest{
		UserID:      0,
		Query:       "",
		HitsPerPage: JobsPerPage,
		Page:        page,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, SearchAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Language", "vi")
	req.Header.Set("X-Source", "Page-Container")
	req.Header.Set("User-Agent", c.config.UserAgent)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("read body: %w", err)
	}

	var searchResp SearchResponse
	if err := json.Unmarshal(respBody, &searchResp); err != nil {
		return nil, 0, fmt.Errorf("parse response: %w", err)
	}

	jobs := make([]*domain.RawJob, 0, len(searchResp.Data))
	for _, item := range searchResp.Data {
		// API returns full URL, use it directly
		jobURL := item.JobURL
		if !strings.HasPrefix(jobURL, "http") {
			jobURL = "https://www.vietnamworks.com/" + jobURL
		}

		// Parse expiredOn for TTL
		expiredOn, _ := time.Parse(time.RFC3339, item.ExpiredOn)
		if expiredOn.IsZero() {
			expiredOn = time.Now().Add(30 * 24 * time.Hour) // Default 30 days
		}

		jobs = append(jobs, &domain.RawJob{
			ID:            fmt.Sprintf("%d", item.JobID),
			URL:           jobURL,
			Source:        string(domain.SourceVietnamWorks),
			LastUpdatedOn: item.LastUpdatedOn, // For change detection
			ExpiredOn:     expiredOn,          // For TTL calculation
			RawData: map[string]any{
				// Basic info
				"jobId":       item.JobID,
				"jobTitle":    item.JobTitle,
				"jobUrl":      item.JobURL,
				"companyName": item.CompanyName,
				"companyId":   item.CompanyID,
				"companyLogo": item.CompanyLogo,
				"companySize": item.CompanySize,
				// Location
				"address":          item.Address,
				"workingLocations": item.WorkingLocations,
				// Salary
				"salaryMin":      item.SalaryMin,
				"salaryMax":      item.SalaryMax,
				"prettySalary":   item.PrettySalary,
				"salaryCurrency": item.SalaryCurrency,
				// Content
				"jobDescription": item.JobDescription,
				"jobRequirement": item.JobRequirement,
				"benefits":       item.Benefits,
				"skills":         item.Skills,
				// Classification
				"industriesV3":       item.IndustriesV3,
				"jobFunction":        item.JobFunction,
				"jobLevelVI":         item.JobLevelVI,
				"yearsOfExperience":  item.YearsOfExperience,
				"typeWorkingId":      item.TypeWorkingID,
				"languageSelectedVI": item.LanguageSelectedVI,
				// Dates
				"approvedOn":    item.ApprovedOn,
				"expiredOn":     item.ExpiredOn,
				"createdOn":     item.CreatedOn,
				"lastUpdatedOn": item.LastUpdatedOn,
			},
			ExtractedAt: time.Now(),
		})
	}

	return jobs, searchResp.Meta.NbPages, nil
}

// Source returns the source identifier
func (c *Crawler) Source() domain.JobSource {
	return domain.SourceVietnamWorks
}
