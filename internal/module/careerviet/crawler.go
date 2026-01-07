package careerviet

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"regexp"
	"time"

	extractor2 "github.com/project-tktt/go-crawler/internal/common/extractor"
	"github.com/project-tktt/go-crawler/internal/domain"
)

const (
	BaseURL    = "https://careerviet.vn"
	ListingURL = "https://careerviet.vn/viec-lam/tat-ca-viec-lam-vi.html"
)

// Crawler implements job crawling for CareerViet
type Crawler struct {
	extractor extractor2.Extractor
	config    Config
}

// Config holds CareerViet-specific configuration
type Config struct {
	MaxPages     int
	RequestDelay time.Duration
}

// NewCrawler creates a new CareerViet crawler
func NewCrawler(ext extractor2.Extractor, cfg Config) *Crawler {
	if cfg.MaxPages <= 0 {
		cfg.MaxPages = 10
	}
	if cfg.RequestDelay <= 0 {
		cfg.RequestDelay = 2 * time.Second // Base delay, will add random 0-2000ms
	}
	return &Crawler{
		extractor: ext,
		config:    cfg,
	}
}

// NewDefaultExtractor creates the hybrid CareerViet extractor (HTML + API)
// This extractor fetches all 50 jobs per page: 20 from HTML, 30 from API
func NewDefaultExtractor(cfg extractor2.ExtractorConfig) extractor2.Extractor {
	return extractor2.NewCareerVietExtractor(cfg)
}

// buildPageURL creates the correct pagination URL for CareerViet
// Format: tat-ca-viec-lam-trang-N-vi.html
func buildPageURL(page int) string {
	if page <= 1 {
		return ListingURL
	}
	return regexp.MustCompile(`-vi\.html$`).ReplaceAllString(ListingURL, fmt.Sprintf("-trang-%d-vi.html", page))
}

// Crawl fetches job listings from CareerViet
func (c *Crawler) Crawl(ctx context.Context) ([]*domain.RawJob, error) {
	var allJobs []*domain.RawJob

	for page := 1; page <= c.config.MaxPages; page++ {
		select {
		case <-ctx.Done():
			return allJobs, ctx.Err()
		default:
		}

		log.Printf("[CareerViet] Crawling page %d/%d", page, c.config.MaxPages)

		listURL := buildPageURL(page)

		jobs, err := c.extractor.ExtractList(ctx, listURL, page)
		if err != nil {
			log.Printf("[CareerViet] Error on page %d: %v", page, err)
			continue
		}

		if len(jobs) == 0 {
			log.Printf("[CareerViet] No more jobs on page %d", page)
			break
		}

		for _, job := range jobs {
			if job.URL == "" {
				continue
			}

			detail, err := c.extractor.Extract(ctx, job.URL)
			if err != nil {
				log.Printf("[CareerViet] Error extracting %s: %v", job.URL, err)
				continue
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

	log.Printf("[CareerViet] Crawled %d jobs", len(allJobs))
	return allJobs, nil
}

// Source returns the source identifier
func (c *Crawler) Source() domain.JobSource {
	return domain.SourceCareerViet
}
