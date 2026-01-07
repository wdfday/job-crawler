package extractor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/project-tktt/go-crawler/internal/domain"
)

// CareerVietExtractor implements hybrid extraction for CareerViet
// First 20 jobs from HTML, remaining 30 from POST API (using shared cookies)
type CareerVietExtractor struct {
	client       *http.Client
	collector    *colly.Collector
	config       ExtractorConfig
	cookiesMutex sync.Mutex
	hasVisited   bool
}

const (
	careerVietBaseURL     = "https://careerviet.vn"
	careerVietSearchAPI   = "https://careerviet.vn/vi/search-jobs"
	careerVietJobsPerPage = 50
)

// CareerVietAPIJob represents a job from the CareerViet API response
type CareerVietAPIJob struct {
	JobID           string   `json:"JOB_ID"`
	JobTitle        string   `json:"JOB_TITLE"`
	CompanyName     string   `json:"EMP_NAME"`
	JobLink         string   `json:"LINK_JOB"`
	Salary          string   `json:"JOB_SALARY_STRING"`
	Locations       []string `json:"LOCATION_NAME_ARR"`
	Benefits        []string `json:"BENEFIT_NAME"`
	ExpireDate      string   `json:"EXPIRE_DATE"`
	CompanyLogoLink string   `json:"LINK_LOGO_EMP"`
}

// CareerVietAPIResponse represents the API response structure
type CareerVietAPIResponse struct {
	Result struct {
		Data []CareerVietAPIJob `json:"data"`
	} `json:"result"`
}

// NewCareerVietExtractor creates a new CareerViet hybrid extractor with shared cookie jar
func NewCareerVietExtractor(config ExtractorConfig) *CareerVietExtractor {
	// Create shared cookie jar for both Colly and HTTP client
	jar, _ := cookiejar.New(nil)

	c := colly.NewCollector(
		colly.UserAgent(config.UserAgent),
	)
	c.SetRequestTimeout(60 * time.Second)
	c.SetCookieJar(jar)

	// Configure transport with longer timeouts
	transport := &http.Transport{
		ResponseHeaderTimeout: 60 * time.Second,
		IdleConnTimeout:       90 * time.Second,
	}

	client := &http.Client{
		Timeout:   60 * time.Second,
		Jar:       jar, // Share the same cookie jar
		Transport: transport,
	}

	return &CareerVietExtractor{
		client:    client,
		collector: c,
		config:    config,
	}
}

func (e *CareerVietExtractor) Name() string {
	return "careerviet_hybrid"
}

// Extract fetches job detail from a single job URL
func (e *CareerVietExtractor) Extract(ctx context.Context, jobURL string) (*domain.RawJob, error) {
	var rawData = make(map[string]any)

	c := e.collector.Clone()

	c.OnHTML("h1.title, h2.title", func(el *colly.HTMLElement) {
		rawData["title"] = strings.TrimSpace(el.Text)
	})

	c.OnHTML(".company-name, .employer-name a", func(el *colly.HTMLElement) {
		if rawData["company"] == nil {
			rawData["company"] = strings.TrimSpace(el.Text)
		}
	})

	c.OnHTML(".location, .job-location", func(el *colly.HTMLElement) {
		rawData["location"] = strings.TrimSpace(el.Text)
	})

	c.OnHTML(".salary, .lbl-salary", func(el *colly.HTMLElement) {
		rawData["salary"] = strings.TrimSpace(el.Text)
	})

	c.OnHTML(".job-exp, .experience", func(el *colly.HTMLElement) {
		rawData["experience"] = strings.TrimSpace(el.Text)
	})

	c.OnHTML(".content-group .job-tag, .content-group__tag", func(el *colly.HTMLElement) {
		benefits := rawData["benefits"]
		if benefits == nil {
			benefits = []string{}
		}
		benefitsList := benefits.([]string)
		benefitsList = append(benefitsList, strings.TrimSpace(el.Text))
		rawData["benefits"] = benefitsList
	})

	// Job description
	c.OnHTML(".job-description, .content-tab", func(el *colly.HTMLElement) {
		desc := el.Text
		rawData["description"] = strings.TrimSpace(desc)
	})

	if err := c.Visit(jobURL); err != nil {
		return nil, fmt.Errorf("visit job page: %w", err)
	}

	// Extract job ID from URL
	jobID := extractJobID(jobURL)

	return &domain.RawJob{
		ID:          jobID,
		URL:         jobURL,
		Source:      string(domain.SourceCareerViet),
		RawData:     rawData,
		ExtractedAt: time.Now(),
	}, nil
}

// ExtractList fetches listing page and returns all 50 jobs (20 HTML + 30 API)
func (e *CareerVietExtractor) ExtractList(ctx context.Context, listURL string, page int) ([]*domain.RawJob, error) {
	var allJobs []*domain.RawJob

	// Step 1: Extract first 20 jobs from HTML
	htmlJobs, err := e.extractFromHTML(ctx, listURL, page)
	if err != nil {
		return nil, fmt.Errorf("extract HTML jobs: %w", err)
	}
	allJobs = append(allJobs, htmlJobs...)

	// Step 2: Extract remaining 30 jobs from API
	apiJobs, err := e.extractFromAPI(ctx, page)
	if err != nil {
		// Log but don't fail - we still have HTML jobs
		fmt.Printf("[CareerViet] API extraction failed (continuing with HTML jobs): %v\n", err)
	} else {
		allJobs = append(allJobs, apiJobs...)
	}

	return allJobs, nil
}

// extractFromHTML gets the first 20 jobs from HTML page
func (e *CareerVietExtractor) extractFromHTML(ctx context.Context, listURL string, page int) ([]*domain.RawJob, error) {
	var jobs []*domain.RawJob

	c := e.collector.Clone()

	// Build correct URL for pagination
	targetURL := buildCareerVietListURL(listURL, page)

	c.OnHTML(".job-item", func(el *colly.HTMLElement) {
		title := strings.TrimSpace(el.ChildText(".job_link"))
		company := strings.TrimSpace(el.ChildText(".company-name"))
		link := el.ChildAttr(".job_link", "href")
		salary := strings.TrimSpace(el.ChildText(".salary"))
		location := strings.TrimSpace(el.ChildText(".location"))

		if link == "" {
			return
		}

		// Ensure absolute URL
		if !strings.HasPrefix(link, "http") {
			link = careerVietBaseURL + link
		}

		jobID := extractJobID(link)

		jobs = append(jobs, &domain.RawJob{
			ID:     jobID,
			URL:    link,
			Source: string(domain.SourceCareerViet),
			RawData: map[string]any{
				"title":    title,
				"company":  company,
				"salary":   salary,
				"location": location,
				"page":     page,
				"source":   "html",
			},
			ExtractedAt: time.Now(),
		})
	})

	if err := c.Visit(targetURL); err != nil {
		return nil, fmt.Errorf("visit list page: %w", err)
	}

	return jobs, nil
}

// extractFromAPI gets remaining 30 jobs from POST API
func (e *CareerVietExtractor) extractFromAPI(ctx context.Context, page int) ([]*domain.RawJob, error) {
	// Build form data
	formData := url.Values{}
	formData.Set("dataOne", "a:0:{}")
	formData.Set("dataTwo", "a:0:{}")

	// Add page parameter for pagination if needed
	if page > 1 {
		formData.Set("page", fmt.Sprintf("%d", page))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, careerVietSearchAPI, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("User-Agent", e.config.UserAgent)
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9,en;q=0.8")
	req.Header.Set("Origin", careerVietBaseURL)
	req.Header.Set("Referer", "https://careerviet.vn/viec-lam/tat-ca-viec-lam-vi.html")

	resp, err := e.client.Do(req)
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

	var apiResp CareerVietAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	var jobs []*domain.RawJob
	for _, job := range apiResp.Result.Data {
		if job.JobLink == "" {
			continue
		}

		jobs = append(jobs, &domain.RawJob{
			ID:     job.JobID,
			URL:    job.JobLink,
			Source: string(domain.SourceCareerViet),
			RawData: map[string]any{
				"title":     job.JobTitle,
				"company":   job.CompanyName,
				"salary":    job.Salary,
				"locations": job.Locations,
				"benefits":  job.Benefits,
				"expire":    job.ExpireDate,
				"logo":      job.CompanyLogoLink,
				"page":      page,
				"source":    "api",
			},
			ExtractedAt: time.Now(),
		})
	}

	return jobs, nil
}

// buildCareerVietListURL builds the correct pagination URL
// CareerViet uses format: tat-ca-viec-lam-trang-2-vi.html
func buildCareerVietListURL(baseURL string, page int) string {
	if page <= 1 {
		return baseURL
	}
	// Replace -vi.html with -trang-N-vi.html
	return regexp.MustCompile(`-vi\.html$`).ReplaceAllString(baseURL, fmt.Sprintf("-trang-%d-vi.html", page))
}

// extractJobID extracts job ID from URL
// URL format: https://careerviet.vn/vi/tim-viec-lam/job-title.JOB_ID.html
func extractJobID(jobURL string) string {
	re := regexp.MustCompile(`\.([A-Z0-9]+)\.html$`)
	matches := re.FindStringSubmatch(jobURL)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}
