package vieclam24h

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/project-tktt/go-crawler/internal/common/dedup"
	"github.com/project-tktt/go-crawler/internal/domain"
	"github.com/project-tktt/go-crawler/internal/module"
	"github.com/project-tktt/go-crawler/internal/queue"
)

const (
	BaseURL   = "https://vieclam24h.vn"
	SearchAPI = "https://apiv2.vieclam24h.vn/employer/fe/job/get-job-list"
)

// Crawler implements job crawling for Vieclam24h using API
type Crawler struct {
	client       *http.Client
	config       Config
	dedup        *dedup.Deduplicator
	pendingQueue *queue.Publisher // Queue for jobs needing detail scrape
}

// NewCrawler creates a new Vieclam24h crawler
func NewCrawler(cfg Config, deduplicator *dedup.Deduplicator, pendingQueue *queue.Publisher) *Crawler {
	if cfg.MaxPages <= 0 {
		cfg.MaxPages = 50 // Unlimited, rely on API's LastPage
	}
	if cfg.PerPage <= 0 {
		cfg.PerPage = 30
	}
	if cfg.RequestDelay <= 0 {
		cfg.RequestDelay = 3 * time.Second
	}
	if cfg.BearerToken == "" {
		cfg.BearerToken = DefaultConfig().BearerToken
	}
	if cfg.Branch == "" {
		cfg.Branch = "vl24h.north"
	}

	return &Crawler{
		client:       &http.Client{Timeout: 30 * time.Second},
		config:       cfg,
		dedup:        deduplicator,
		pendingQueue: pendingQueue,
	}
}

// Crawl fetches job listings from Vieclam24h API
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
	newJobCount := 0

	for page := 1; page <= c.config.MaxPages; page++ {
		log.Printf("[Vieclam24h] Fetching page %d", page)

		resp, err := c.fetchPage(ctx, page)
		if err != nil {
			log.Printf("[Vieclam24h] Error on page %d: %v", page, err)
			break
		}

		if len(resp.Data.Items) == 0 {
			log.Printf("[Vieclam24h] No more jobs on page %d", page)
			break
		}

		// Convert API items to RawJobs and check dedup
		var pendingJobs []*domain.RawJob
		newCount := 0
		updatedCount := 0
		unchangedCount := 0

		for _, item := range resp.Data.Items {
			job := c.itemToRawJob(item)

			// Check dedup
			lastUpdated := fmt.Sprintf("%d", item.UpdatedAt)
			result, err := c.dedup.CheckJob(ctx, job.Source, job.ID, lastUpdated)
			if err != nil {
				log.Printf("[Vieclam24h] Dedup check error for %s: %v", job.ID, err)
				continue
			}

			// Log all jobs with status and URL (if verbose enabled)
			var status string
			switch result {
			case dedup.ResultNew:
				status = "NEW"
				newCount++
			case dedup.ResultUpdated:
				status = "UPDATED"
				updatedCount++
			case dedup.ResultUnchanged:
				status = "UNCHANGED"
				unchangedCount++
			}

			if c.config.VerboseLog {
				log.Printf("[Vieclam24h] Page %d | %-9s | ID: %s | %s",
					page, status, job.ID, job.URL)
			}

			// Skip unchanged jobs
			if result == dedup.ResultUnchanged {
				continue
			}

			pendingJobs = append(pendingJobs, job)

			// Push to pending queue for detail scraping (if needed)
			if c.pendingQueue != nil {
				if err := c.pendingQueue.Publish(ctx, job); err != nil {
					log.Printf("[Vieclam24h] Failed to publish job %s: %v", job.ID, err)
					continue // Don't mark as seen if publish failed
				}

				// Mark as seen/updated
				if err := c.dedup.MarkSeenWithTTL(ctx, job.Source, job.ID, job.LastUpdatedOn, job.ExpiredOn); err != nil {
					log.Printf("[Vieclam24h] Failed to mark job seen %s: %v", job.ID, err)
				}
			}
		}

		newJobCount += len(pendingJobs)
		totalJobCount += len(resp.Data.Items)

		// Call handler if provided
		if handler != nil && len(pendingJobs) > 0 {
			if err := handler(pendingJobs); err != nil {
				log.Printf("[Vieclam24h] Handler error on page %d: %v", page, err)
			}
		}

		log.Printf("[Vieclam24h] Page %d summary: %d total | %d NEW | %d UPDATED | %d UNCHANGED",
			page, len(resp.Data.Items), newCount, updatedCount, unchangedCount)

		// Stop if we've reached the last page (if LastPage is valid)
		if resp.Data.Pagination.LastPage > 0 && page >= resp.Data.Pagination.LastPage {
			log.Printf("[Vieclam24h] Reached last page (%d)", resp.Data.Pagination.LastPage)
			break
		}

		// Fallback: Stop if we received fewer items than requested
		if len(resp.Data.Items) < c.config.PerPage {
			log.Printf("[Vieclam24h] Page %d has %d items (< %d), stopping", page, len(resp.Data.Items), c.config.PerPage)
			break
		}

		// Random delay (Base + 0-3s jitter)
		randomDelay := c.config.RequestDelay + time.Duration(rand.Intn(3000))*time.Millisecond
		time.Sleep(randomDelay)
	}

	log.Printf("[Vieclam24h] Crawled %d jobs total, %d new/updated", totalJobCount, newJobCount)
	return nil
}

// fetchPage fetches a single page from the API
func (c *Crawler) fetchPage(ctx context.Context, page int) (*APIResponse, error) {
	url := fmt.Sprintf("%s?page=%d&per_page=%d&request_from=search_result_web", SearchAPI, page, c.config.PerPage)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.BearerToken)
	req.Header.Set("X-Branch", c.config.Branch)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if apiResp.Code != 200 {
		return nil, fmt.Errorf("API error: %s", apiResp.Msg)
	}

	return &apiResp, nil
}

// itemToRawJob converts API item to domain.RawJob
func (c *Crawler) itemToRawJob(item JobItem) *domain.RawJob {
	jobURL := fmt.Sprintf("%s/%s-c%dp%did%d.html",
		BaseURL, item.TitleSlug, item.FieldIDMain, item.ProvinceIDs[0], item.ID)

	// Parse expiry time
	expiredOn := time.Unix(item.ResumeApplyExpired, 0)
	if expiredOn.IsZero() {
		expiredOn = time.Now().Add(30 * 24 * time.Hour)
	}

	// Job description is extracted in enricher from JSON-LD
	var description string

	return &domain.RawJob{
		ID:            fmt.Sprintf("%d", item.ID),
		URL:           jobURL,
		Source:        string(domain.SourceVieclam24h),
		LastUpdatedOn: fmt.Sprintf("%d", item.UpdatedAt),
		ExpiredOn:     expiredOn,
		RawData: map[string]any{
			// Basic info
			"jobId":       item.ID,
			"jobTitle":    item.Title,
			"jobUrl":      jobURL,
			"companyId":   item.EmployerID,
			"companyName": item.EmployerInfo.Name,
			"companyLogo": item.EmployerInfo.Logo,
			// Location
			"provinceIds":    item.ProvinceIDs,
			"districtIds":    item.DistrictIDs,
			"contactAddress": item.ContactAddress,
			// Salary
			"salaryFrom": item.SalaryFrom,
			"salaryTo":   item.SalaryTo,
			"salaryText": item.SalaryText,
			"salaryUnit": item.SalaryUnit,
			// Content
			"jobDescription":   description,
			"jobRequirement":   item.JobRequirementHTML,
			"otherRequirement": item.OtherRequirementHTML,
			// Classification
			"occupationIds":     item.OccupationIDs,
			"fieldIdMain":       item.FieldIDMain,
			"fieldIdsSub":       item.FieldIDsSub,
			"levelRequirement":  item.LevelRequirement,
			"degreeRequirement": item.DegreeRequirement,
			"experienceRange":   item.ExperienceRange,
			"workingMethod":     item.WorkingMethod,
			"gender":            item.Gender,
			"vacancyQuantity":   item.VacancyQuantity,
			// Stats & Trust Signals
			"totalViews":         item.TotalViews,
			"totalResumeApplied": item.TotalResumeApplied,
			"rateResponse":       item.EmployerInfo.RateResponse,
			// Dates
			"createdAt": item.CreatedAt,
			"updatedAt": item.UpdatedAt,
			"expiredAt": item.ResumeApplyExpired,
		},
		ExtractedAt: time.Now(),
	}
}

// Source returns the source identifier
func (c *Crawler) Source() domain.JobSource {
	return domain.SourceVieclam24h
}
