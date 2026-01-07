package topcv

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"time"

	extractor2 "github.com/project-tktt/go-crawler/internal/common/extractor"
	"github.com/project-tktt/go-crawler/internal/domain"
)

const (
	BaseURL    = "https://www.topcv.vn"
	ListingURL = "https://www.topcv.vn/tim-viec-lam-moi-nhat"
)

// Crawler implements job crawling for TopCV
type Crawler struct {
	extractor extractor2.Extractor
	config    Config
}

// Config holds TopCV-specific configuration
type Config struct {
	MaxPages     int
	RequestDelay time.Duration
}

// NewCrawler creates a new TopCV crawler
func NewCrawler(ext extractor2.Extractor, cfg Config) *Crawler {
	if cfg.MaxPages <= 0 {
		cfg.MaxPages = 10
	}
	if cfg.RequestDelay <= 0 {
		cfg.RequestDelay = 2 * time.Second
	}
	return &Crawler{
		extractor: ext,
		config:    cfg,
	}
}

// NewDefaultExtractor creates a Colly extractor configured for TopCV
func NewDefaultExtractor(cfg extractor2.ExtractorConfig) extractor2.Extractor {
	selectors := extractor2.Selectors{
		// TopCV uses Next.js, extract from __NEXT_DATA__ script
		NextDataScript: "script#__NEXT_DATA__",

		// Fallback selectors for HTML parsing
		JobItem:  ".job-item",
		JobLink:  "a.job-item-link",
		Title:    "h1.job-title",
		Company:  ".company-name",
		Location: ".job-location",
		Salary:   ".salary-text",
	}

	return extractor2.NewCollyExtractor(domain.SourceTopCV, selectors, cfg)
}

// Crawl fetches job listings from TopCV
func (c *Crawler) Crawl(ctx context.Context) ([]*domain.RawJob, error) {
	var allJobs []*domain.RawJob

	for page := 1; page <= c.config.MaxPages; page++ {
		select {
		case <-ctx.Done():
			return allJobs, ctx.Err()
		default:
		}

		log.Printf("[TopCV] Crawling page %d/%d", page, c.config.MaxPages)

		jobs, err := c.extractor.ExtractList(ctx, ListingURL, page)
		if err != nil {
			log.Printf("[TopCV] Error on page %d: %v", page, err)
			continue
		}

		if len(jobs) == 0 {
			log.Printf("[TopCV] No more jobs on page %d, stopping", page)
			break
		}

		// Fetch detail for each job
		for _, job := range jobs {
			if job.URL == "" {
				continue
			}

			detail, err := c.extractor.Extract(ctx, job.URL)
			if err != nil {
				log.Printf("[TopCV] Error extracting %s: %v", job.URL, err)
				continue
			}

			// Parse __NEXT_DATA__ if present
			if nextData, ok := detail.RawData["__NEXT_DATA__"].(string); ok {
				parsed := parseNextData(nextData)
				if parsed != nil {
					detail.RawData = parsed
				}
			}

			allJobs = append(allJobs, detail)
			// Random delay: base delay + random 0-2000ms
			randomDelay := c.config.RequestDelay + time.Duration(rand.Intn(2000))*time.Millisecond
			time.Sleep(randomDelay)
		}

		// Random delay between pages
		randomDelay := c.config.RequestDelay + time.Duration(rand.Intn(2000))*time.Millisecond
		time.Sleep(randomDelay)
	}

	log.Printf("[TopCV] Crawled %d jobs", len(allJobs))
	return allJobs, nil
}

// Source returns the source identifier
func (c *Crawler) Source() domain.JobSource {
	return domain.SourceTopCV
}

// parseNextData extracts job data from Next.js __NEXT_DATA__ JSON
func parseNextData(script string) map[string]any {
	var data struct {
		Props struct {
			PageProps struct {
				Job map[string]any `json:"job"`
			} `json:"pageProps"`
		} `json:"props"`
	}

	if err := json.Unmarshal([]byte(script), &data); err != nil {
		return nil
	}

	return data.Props.PageProps.Job
}
