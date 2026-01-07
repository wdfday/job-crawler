package module

import (
	"context"

	"github.com/project-tktt/go-crawler/internal/domain"
)

// JobHandler is a callback function for processing jobs from each page
type JobHandler func(jobs []*domain.RawJob) error

// Crawler is the common interface for all job crawlers
type Crawler interface {
	// Crawl fetches jobs from the source
	Crawl(ctx context.Context) ([]*domain.RawJob, error)
	// CrawlWithCallback fetches jobs page by page and calls handler after each page
	CrawlWithCallback(ctx context.Context, handler JobHandler) error
	// Source returns the source identifier
	Source() domain.JobSource
}
