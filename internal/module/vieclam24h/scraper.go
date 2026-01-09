package vieclam24h

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

	"github.com/PuerkitoBio/goquery"
	"github.com/project-tktt/go-crawler/internal/domain"
	"github.com/project-tktt/go-crawler/internal/queue"
)

// Scraper consumes pending jobs and scrapes details
type Scraper struct {
	consumer        *queue.Consumer
	publisher       *queue.Publisher
	jsonLdPublisher *queue.Publisher // For JSON-LD validation
	client          *http.Client
	requestDelay    time.Duration
}

// NewScraper creates a new detail scraper
func NewScraper(consumer *queue.Consumer, publisher *queue.Publisher, jsonLdPublisher *queue.Publisher, delay time.Duration) *Scraper {
	if delay <= 0 {
		delay = 5*time.Second + time.Duration(rand.Intn(3000))*time.Millisecond
	}
	return &Scraper{
		consumer:        consumer,
		publisher:       publisher,
		jsonLdPublisher: jsonLdPublisher,
		requestDelay:    delay,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:       10,
				IdleConnTimeout:    30 * time.Second,
				DisableCompression: true,
			},
		},
	}
}

// Run starts the scraper loop
func (s *Scraper) Run(ctx context.Context) error {
	log.Printf("[Vieclam24h] Starting detail scraper (delay: %v)...", s.requestDelay)

	return s.consumer.Run(ctx, func(job *domain.RawJob) error {
		log.Printf("[Vieclam24h] Scraping detail for %s (%s)", job.ID, job.URL)

		// Fetch full HTML content
		htmlContent, err := s.fetchHTML(ctx, job.URL)
		if err != nil {
			log.Printf("[Vieclam24h] Failed to fetch HTML for %s: %v", job.ID, err)
			// Decide whether to retry or push partial data.
			// For now, we log and proceed with existing API data (which is quite full)
			// But ideally we might want to retry.
		} else {
			// Update job with HTML content (temporary for enrichment)
			job.HTMLContent = htmlContent

			// Extract and publish raw JSON-LD for validation
			if s.jsonLdPublisher != nil {
				s.publishJsonLd(ctx, job.ID, htmlContent)
			}

			// Extract additional info from HTML (fallback/enrichment)
			enrichJobData(job, htmlContent)

			// Remove HTML content to avoid heavy Redis payload
			job.HTMLContent = ""
		}

		job.ExtractedAt = time.Now()

		if err := s.publisher.Publish(ctx, job); err != nil {
			return fmt.Errorf("publish to raw queue: %w", err)
		}
		log.Printf("[Vieclam24h] Published job %s to raw queue", job.ID)

		// Apply delay to be polite
		if s.requestDelay > 0 {
			// Add random jitter (0-3000ms) to the base requestDelay
			randomDelay := s.requestDelay + time.Duration(rand.Intn(3000))*time.Millisecond
			time.Sleep(randomDelay)
		}

		return nil
	})
}

func (s *Scraper) fetchHTML(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	// Emulate browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	return string(body), nil
}

// publishJsonLd extracts JSON-LD from HTML and publishes to validation queue
func (s *Scraper) publishJsonLd(ctx context.Context, jobID string, html string) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return
	}

	doc.Find("script[type='application/ld+json']").Each(func(i int, sel *goquery.Selection) {
		jsonContent := strings.TrimSpace(sel.Text())
		if jsonContent == "" {
			return
		}

		// Validate it's valid JSON
		var raw map[string]any
		if err := json.Unmarshal([]byte(jsonContent), &raw); err == nil {
			// Add job ID for reference
			raw["_jobId"] = jobID
			raw["_extractedAt"] = time.Now().Format(time.RFC3339)

			// Publish to validation queue
			if err := s.jsonLdPublisher.PublishRaw(ctx, raw); err != nil {
				log.Printf("[Vieclam24h] Failed to publish JSON-LD for %s: %v", jobID, err)
			} else {
				log.Printf("[Vieclam24h] Published JSON-LD for %s to validation queue", jobID)
			}
		}
	})
}

func enrichJobData(job *domain.RawJob, html string) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return
	}

	// Example: Extract breadcrumbs or specific meta tags not in API
	// Since API is very detailed, we primarily store HTML for reference/archival
	// or for standardizing extraction if API schema changes.

	// Try to get canonical URL
	if canonical, exists := doc.Find("link[rel='canonical']").Attr("href"); exists {
		if job.RawData == nil {
			job.RawData = make(map[string]any)
		}
		job.RawData["canonicalUrl"] = canonical
	}

	// Extract experience text from HTML (more reliable than API experienceRange ID)
	// HTML structure:
	//   <div class="flex flex-col w-full">
	//     <div>Kinh nghiệm</div>
	//     <div>1 năm</div>
	//   </div>
	doc.Find("div.flex.flex-col").Each(func(i int, parent *goquery.Selection) {
		// Get all child divs
		children := parent.Children().Filter("div")
		if children.Length() >= 2 {
			labelText := strings.TrimSpace(children.First().Text())
			if labelText == "Kinh nghiệm" {
				valueText := strings.TrimSpace(children.Eq(1).Text())
				if job.RawData == nil {
					job.RawData = make(map[string]any)
				}
				job.RawData["experienceText"] = valueText
			}
		}
	})

	// Parse JSON-LD
	doc.Find("script[type='application/ld+json']").Each(func(i int, s *goquery.Selection) {
		jsonContent := s.Text()
		// Clean up content if needed
		jsonContent = strings.TrimSpace(jsonContent)

		var jobPosting JobPosting
		if err := json.Unmarshal([]byte(jsonContent), &jobPosting); err != nil {
			// Not a JobPosting or invalid JSON, skip
			return
		}

		// Check if it's actually a JobPosting
		if jobPosting.Type != "JobPosting" {
			return
		}

		if job.RawData == nil {
			job.RawData = make(map[string]any)
		}

		// Extract fields
		if jobPosting.Description != "" {
			job.RawData["jobDescription"] = jobPosting.Description
		}
		if jobPosting.JobBenefits != "" {
			job.RawData["jobBenefits"] = jobPosting.JobBenefits
		}
		if jobPosting.Skills != "" {
			job.RawData["skills"] = jobPosting.Skills
		}
		if jobPosting.Qualifications != "" {
			job.RawData["qualifications"] = jobPosting.Qualifications
		}
		if jobPosting.Industry != "" {
			// Split industry by comma for array indexing (GIN)
			parts := strings.Split(jobPosting.Industry, ",")
			var industries []string
			for _, p := range parts {
				if trimmed := strings.TrimSpace(p); trimmed != "" {
					industries = append(industries, trimmed)
				}
			}
			job.RawData["industry"] = industries
		}
		if jobPosting.OccupationalCategory != "" {
			job.RawData["occupationalCategory"] = jobPosting.OccupationalCategory
		}
		if jobPosting.EmploymentType != "" {
			job.RawData["employmentType"] = jobPosting.EmploymentType
		}

		// Company Website
		if jobPosting.HiringOrganization.SameAs != "" {
			job.RawData["companyWebsite"] = jobPosting.HiringOrganization.SameAs
		}

		// Structured Location - extract ALL locations as arrays (deduplicated)
		if len(jobPosting.JobLocation) > 0 {
			citySet := make(map[string]bool)
			districtSet := make(map[string]bool)
			var cities []string
			var districts []string
			for _, loc := range jobPosting.JobLocation {
				addr := loc.Address
				if addr.AddressRegion != "" && !citySet[addr.AddressRegion] {
					citySet[addr.AddressRegion] = true
					cities = append(cities, addr.AddressRegion)
				}
				if addr.AddressLocality != "" && !districtSet[addr.AddressLocality] {
					districtSet[addr.AddressLocality] = true
					districts = append(districts, addr.AddressLocality)
				}
			}
			if len(cities) > 0 {
				job.RawData["locationCity"] = cities
			}
			if len(districts) > 0 {
				job.RawData["locationDistrict"] = districts
			}
		}

		// Salary from JSON-LD baseSalary (prioritized over API)
		if jobPosting.BaseSalary.Value.MinValue > 0 || jobPosting.BaseSalary.Value.MaxValue > 0 {
			job.RawData["salaryMinJsonLd"] = jobPosting.BaseSalary.Value.MinValue
			job.RawData["salaryMaxJsonLd"] = jobPosting.BaseSalary.Value.MaxValue
			job.RawData["salaryCurrency"] = jobPosting.BaseSalary.Currency
		}
		if jobPosting.BaseSalary.Value.Value != "" {
			// Negotiable salary text like "Thỏa thuận"
			job.RawData["salaryTextJsonLd"] = jobPosting.BaseSalary.Value.Value
			job.RawData["isNegotiable"] = true
		}

		log.Printf("[Vieclam24h] Extracted JSON-LD for %s: Description len=%d", job.ID, len(jobPosting.Description))
	})
}
