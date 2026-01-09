package worker

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/project-tktt/go-crawler/internal/common/cleaner"
	"github.com/project-tktt/go-crawler/internal/common/indexer"
	"github.com/project-tktt/go-crawler/internal/common/normalizer"
	"github.com/project-tktt/go-crawler/internal/domain"
	"github.com/project-tktt/go-crawler/internal/queue"
)

// Worker processes jobs from queue and indexes to storage
type Worker struct {
	consumer   *queue.Consumer
	normalizer *normalizer.Normalizer
	cleaner    *cleaner.Cleaner
	indexer    indexer.Indexer

	batchSize   int
	concurrency int
}

// Config holds worker configuration
type Config struct {
	Concurrency int
	BatchSize   int
}

// NewWorker creates a new worker
func NewWorker(
	consumer *queue.Consumer,
	norm *normalizer.Normalizer,
	clean *cleaner.Cleaner,
	idx indexer.Indexer,
	cfg Config,
) *Worker {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 5
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}

	return &Worker{
		consumer:    consumer,
		normalizer:  norm,
		cleaner:     clean,
		indexer:     idx,
		batchSize:   cfg.BatchSize,
		concurrency: cfg.Concurrency,
	}
}

// Run starts the worker pool
func (w *Worker) Run(ctx context.Context) error {
	log.Printf("Starting worker pool with %d workers", w.concurrency)

	var wg sync.WaitGroup
	errChan := make(chan error, w.concurrency)

	for i := 0; i < w.concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			if err := w.runSingle(ctx, workerID); err != nil {
				errChan <- fmt.Errorf("worker %d: %w", workerID, err)
			}
		}(i)
	}

	// Wait for all workers or context cancellation
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		return err
	case <-done:
		return nil
	}
}

func (w *Worker) runSingle(ctx context.Context, workerID int) error {
	log.Printf("Worker %d started", workerID)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %d stopping", workerID)
			return nil
		default:
		}

		// ConsumeBatch uses BRPOP for first item (blocking), so no CPU spinning
		rawJobs, err := w.consumer.ConsumeBatch(ctx, w.batchSize)
		if err != nil {
			log.Printf("Worker %d consume error: %v", workerID, err)
			continue
		}

		if len(rawJobs) == 0 {
			continue // Timeout from BRPOP, try again
		}

		log.Printf("Worker %d processing %d jobs", workerID, len(rawJobs))

		// Process and index jobs
		jobs := w.processJobs(rawJobs)
		if len(jobs) > 0 {
			if err := w.indexer.BulkIndex(ctx, jobs); err != nil {
				log.Printf("Worker %d index error: %v", workerID, err)
			} else {
				log.Printf("Worker %d indexed %d jobs", workerID, len(jobs))
			}
		}
	}
}

func (w *Worker) processJobs(rawJobs []*domain.RawJob) []*domain.Job {
	jobs := make([]*domain.Job, 0, len(rawJobs))

	for _, raw := range rawJobs {
		// Clean raw data
		if raw.RawData != nil {
			raw.RawData = w.cleaner.CleanMap(raw.RawData)
		}

		// Normalize to standard format
		job, err := w.normalizer.Normalize(raw)
		if err != nil {
			log.Printf("Normalize error for %s: %v", raw.ID, err)
			continue
		}

		// Clean text fields
		job.Description = w.cleaner.CleanToText(job.Description)
		job.Requirements = w.cleaner.CleanToText(job.Requirements)
		job.Benefits = w.cleaner.CleanToText(job.Benefits)

		jobs = append(jobs, job)
	}

	return jobs
}
