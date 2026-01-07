package extractor

import (
	"context"

	"github.com/project-tktt/go-crawler/internal/domain"
)

// Extractor defines the interface for extracting job data from sources
// Two implementations: APIExtractor (for API-based sources) and CollyExtractor (for HTML scraping)
type Extractor interface {
	// Extract fetches and extracts a single job from the given URL
	Extract(ctx context.Context, url string) (*domain.RawJob, error)

	// ExtractList fetches a listing page and extracts multiple jobs
	ExtractList(ctx context.Context, listURL string, page int) ([]*domain.RawJob, error)

	// Name returns the name of this extractor
	Name() string
}

// ExtractorConfig holds common configuration for extractors
type ExtractorConfig struct {
	UserAgent    string
	ProxyURL     string
	MaxRetries   int
	RequestDelay int // milliseconds
}
