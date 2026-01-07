package indexer

import (
	"context"

	"github.com/project-tktt/go-crawler/internal/domain"
)

// Indexer defines the interface for job indexing backends
type Indexer interface {
	// BulkIndex indexes multiple jobs at once
	BulkIndex(ctx context.Context, jobs []*domain.Job) error
}
