package topdev

import (
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
	SearchAPIURL = "https://api.topdev.vn/td/v2/jobs"
	JobsPerPage  = 20
)

// Crawler implements job crawling for TopDev
type Crawler struct {
	client *http.Client
	config Config
}

// Config holds TopDev-specific configuration
type Config struct {
	MaxPages     int
	RequestDelay time.Duration
	UserAgent    string
}

// SearchResponse is the TopDev API response
type SearchResponse struct {
	Data []JobData `json:"data"`
	Meta struct {
		Total       int `json:"total"`
		PerPage     int `json:"per_page"`
		CurrentPage int `json:"current_page"`
		LastPage    int `json:"last_page"`
	} `json:"meta"`
}

type JobData struct {
	ID                       int         `json:"id"`
	Slug                     string      `json:"slug"`
	Title                    string      `json:"title"`
	OwnedID                  int         `json:"owned_id"`
	Company                  Company     `json:"company,omitempty"`
	Salary                   Salary      `json:"salary,omitempty"`
	SkillsStr                string      `json:"skills_str"`
	Skills                   []Skill     `json:"skills"`
	WorkLocations            []Location  `json:"work_locations"`
	ResponsibilitiesOriginal string      `json:"responsibilities_original"`
	RequirementsOriginal     string      `json:"requirements_original"`
	BenefitsOriginal         []Benefit   `json:"benefits_original"`
	PublishedAt              string      `json:"published_at"`
	ExpiredAt                string      `json:"expired_at"`
	IsSalaryVisible          bool        `json:"is_salary_visible"`
	YearsOfExperience        interface{} `json:"years_of_experience"`
	JobLevel                 interface{} `json:"job_level"`
}

type Company struct {
	ID          int    `json:"id"`
	DisplayName string `json:"display_name"`
	ImageLogo   string `json:"image_logo"`
	Slug        string `json:"slug"`
}

type Salary struct {
	MinFilter int    `json:"min_filter"`
	MaxFilter int    `json:"max_filter"`
	Currency  string `json:"currency"`
	Value     string `json:"value"`
}

type Skill struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Location struct {
	ID       int    `json:"id"`
	Address  string `json:"address"`
	City     string `json:"city"`
	District string `json:"district"`
}

type Benefit struct {
	Icon  string `json:"icon"`
	Value string `json:"value"`
}

// NewCrawler creates a new TopDev crawler
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

// Crawl fetches job listings from TopDev API
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

	for page := 1; page <= c.config.MaxPages; page++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		log.Printf("[TopDev] Fetching page %d/%d", page, c.config.MaxPages)

		jobs, totalPages, err := c.fetchPage(ctx, page)
		if err != nil {
			log.Printf("[TopDev] Error on page %d: %v", page, err)
			break
		}

		if len(jobs) == 0 {
			log.Printf("[TopDev] No more jobs on page %d", page)
			break
		}

		// Process jobs immediately via callback
		if err := handler(jobs); err != nil {
			log.Printf("[TopDev] Handler error on page %d: %v", page, err)
		}

		totalJobCount += len(jobs)
		log.Printf("[TopDev] Page %d: %d jobs processed", page, len(jobs))

		// Stop if we've reached the last page
		if page >= totalPages {
			log.Printf("[TopDev] Reached last page (%d)", totalPages)
			break
		}

		// Random delay: base delay + random 0-2000ms
		randomDelay := c.config.RequestDelay + time.Duration(rand.Intn(2000))*time.Millisecond
		time.Sleep(randomDelay)
	}

	log.Printf("[TopDev] Crawled %d jobs total", totalJobCount)
	return nil
}

// fetchPage fetches a single page of jobs
func (c *Crawler) fetchPage(ctx context.Context, page int) ([]*domain.RawJob, int, error) {
	// Use fields[job] parameter to get full job details
	url := fmt.Sprintf("%s?page=%d&limit=%d&locale=vi_VN&fields[job]=id,title,slug,company,salary,skills_str,work_locations,responsibilities_original,requirements_original,benefits_original", SearchAPIURL, page, JobsPerPage)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
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
		// Build job URL
		jobURL := fmt.Sprintf("https://topdev.vn/job/%s", item.Slug)
		if item.Slug == "" {
			jobURL = fmt.Sprintf("https://topdev.vn/job/%d", item.ID)
		}

		// Extract skill names - skills_str is a comma-separated string
		var skillNames []string
		if item.SkillsStr != "" {
			// Split comma-separated skills
			for _, s := range strings.Split(item.SkillsStr, ",") {
				if trimmed := strings.TrimSpace(s); trimmed != "" {
					skillNames = append(skillNames, trimmed)
				}
			}
		} else {
			for _, s := range item.Skills {
				skillNames = append(skillNames, s.Name)
			}
		}

		// Extract locations
		var locations []string
		for _, loc := range item.WorkLocations {
			parts := []string{}
			if loc.Address != "" {
				parts = append(parts, loc.Address)
			}
			if loc.District != "" {
				parts = append(parts, loc.District)
			}
			if loc.City != "" {
				parts = append(parts, loc.City)
			}
			if len(parts) > 0 {
				locations = append(locations, strings.Join(parts, ", "))
			}
		}

		// Extract benefits
		var benefits []string
		for _, b := range item.BenefitsOriginal {
			if b.Value != "" {
				benefits = append(benefits, b.Value)
			}
		}

		jobs = append(jobs, &domain.RawJob{
			ID:     fmt.Sprintf("%d", item.ID),
			URL:    jobURL,
			Source: string(domain.SourceTopDev),
			RawData: map[string]any{
				"title":        item.Title,
				"company":      item.Company.DisplayName,
				"company_logo": item.Company.ImageLogo,
				"salary_min":   item.Salary.MinFilter,
				"salary_max":   item.Salary.MaxFilter,
				"salary_text":  item.Salary.Value,
				"currency":     item.Salary.Currency,
				"skills":       skillNames,
				"locations":    locations,
				"description":  item.ResponsibilitiesOriginal,
				"requirement":  item.RequirementsOriginal,
				"benefits":     benefits,
				"published_at": item.PublishedAt,
				"expired_at":   item.ExpiredAt,
				"experience":   item.YearsOfExperience,
				"level":        item.JobLevel,
			},
			ExtractedAt: time.Now(),
		})
	}

	return jobs, searchResp.Meta.LastPage, nil
}

// Source returns the source identifier
func (c *Crawler) Source() domain.JobSource {
	return domain.SourceTopDev
}
