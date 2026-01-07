package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/project-tktt/go-crawler/internal/common/cleaner"
	"github.com/project-tktt/go-crawler/internal/common/dedup"
	"github.com/project-tktt/go-crawler/internal/common/extractor"
	"github.com/project-tktt/go-crawler/internal/common/indexer"
	"github.com/project-tktt/go-crawler/internal/common/normalizer"
	"github.com/project-tktt/go-crawler/internal/config"
	"github.com/project-tktt/go-crawler/internal/domain"
	"github.com/project-tktt/go-crawler/internal/module"
	vietnamworks2 "github.com/project-tktt/go-crawler/internal/module/vietnamworks"
	"github.com/project-tktt/go-crawler/internal/module/worker"
	// "github.com/project-tktt/go-crawler/internal/crawler/careerviet" // Temporarily disabled
	// "github.com/project-tktt/go-crawler/internal/crawler/topcv" // Temporarily disabled
	// "github.com/project-tktt/go-crawler/internal/crawler/topdev" // Temporarily disabled
	"github.com/project-tktt/go-crawler/internal/queue"
	"github.com/redis/go-redis/v9"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Job Crawler Service")

	// Load configuration
	cfg := config.Load()

	// Initialize Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test Redis connection
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Redis connection failed: %v", err)
	}
	log.Println("Redis connected")

	// Initialize PostgreSQL indexer (temporarily replacing Elasticsearch)
	pgIndexer, err := indexer.NewPostgresIndexer(cfg.Postgres.ConnectionString, cfg.Postgres.TableName)
	if err != nil {
		log.Fatalf("PostgreSQL connection failed: %v", err)
	}
	log.Println("PostgreSQL connected")

	// Elasticsearch indexer - temporarily disabled
	// esIndexer, err := indexer.NewElasticsearchIndexer(cfg.Elasticsearch.Addresses, cfg.Elasticsearch.Index)
	// if err != nil {
	// 	log.Fatalf("Elasticsearch connection failed: %v", err)
	// }
	// log.Println("Elasticsearch connected")
	// if err := esIndexer.EnsureIndex(ctx); err != nil {
	// 	log.Printf("Warning: ensure index failed: %v", err)
	// }

	// Initialize components
	deduplicator := dedup.NewDeduplicator(rdb, "job:seen", 30*24*time.Hour)
	htmlCleaner := cleaner.NewCleaner()
	norm := normalizer.NewNormalizer()
	publisher := queue.NewPublisher(rdb, cfg.Redis.JobQueue)
	consumer := queue.NewConsumer(rdb, cfg.Redis.JobQueue, 5*time.Second)

	// Initialize crawlers
	// extractorCfg is temporarily unused since TopCV and CareerViet are disabled
	_ = extractor.ExtractorConfig{
		UserAgent:    cfg.Crawler.UserAgent,
		ProxyURL:     cfg.Crawler.ProxyURL,
		MaxRetries:   cfg.Crawler.MaxRetries,
		RequestDelay: int(cfg.Crawler.RequestDelay.Milliseconds()),
	}

	crawlers := []module.Crawler{
		// TopCV crawler - temporarily disabled
		// topcv.NewCrawler(
		// 	topcv.NewDefaultExtractor(extractorCfg),
		// 	topcv.Config{MaxPages: 1000, RequestDelay: 2 * time.Second},
		// ),
		// VietnamWorks uses API directly, no extractor needed
		vietnamworks2.NewCrawler(
			vietnamworks2.Config{MaxPages: 1000, RequestDelay: 2 * time.Second},
		),
		// CareerViet crawler - temporarily disabled
		// careerviet.NewCrawler(
		// 	careerviet.NewDefaultExtractor(extractorCfg),
		// 	careerviet.Config{MaxPages: 1000, RequestDelay: 2 * time.Second},
		// ),
		// TopDev crawler - temporarily disabled
		// topdev.NewCrawler(
		// 	topdev.Config{MaxPages: 1000, RequestDelay: 2 * time.Second},
		// ),
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	// Start worker pool (processes queue -> normalizes -> indexes to PostgreSQL)
	wg.Add(1)
	go func() {
		defer wg.Done()
		w := worker.NewWorker(consumer, norm, htmlCleaner, pgIndexer, worker.Config{
			Concurrency: cfg.Worker.Concurrency,
			BatchSize:   cfg.Worker.BatchSize,
		})
		if err := w.Run(ctx); err != nil && err != context.Canceled {
			log.Printf("Worker error: %v", err)
		}
	}()

	// Start crawler scheduler (runs each crawler periodically)
	wg.Add(1)
	go func() {
		defer wg.Done()
		runCrawlerScheduler(ctx, crawlers, deduplicator, publisher)
	}()

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutdown signal received, stopping...")
	cancel()

	// Wait for goroutines to finish
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("Graceful shutdown complete")
	case <-time.After(30 * time.Second):
		log.Println("Shutdown timeout, forcing exit")
	}
}

// runCrawlerScheduler runs each crawler sequentially at intervals
func runCrawlerScheduler(ctx context.Context, crawlers []module.Crawler, deduplicator *dedup.Deduplicator, publisher *queue.Publisher) {
	// Run immediately on startup
	runAllCrawlers(ctx, crawlers, deduplicator, publisher)

	// Then schedule to run every hour
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runAllCrawlers(ctx, crawlers, deduplicator, publisher)
		}
	}
}

func runAllCrawlers(ctx context.Context, crawlers []module.Crawler, deduplicator *dedup.Deduplicator, publisher *queue.Publisher) {
	for _, c := range crawlers {
		select {
		case <-ctx.Done():
			return
		default:
		}

		log.Printf("Running crawler: %s", c.Source())

		var newJobs, updatedJobs, unchangedJobs, totalJobs int

		// Use streaming callback to process each page immediately
		err := c.CrawlWithCallback(ctx, func(jobs []*domain.RawJob) error {
			pageNew, pageUpdated := 0, 0
			for _, job := range jobs {
				jobID := job.ID
				if jobID == "" {
					jobID = job.URL
				}

				// Smart dedup: check if new, updated, or unchanged
				result, err := deduplicator.CheckJob(ctx, string(c.Source()), jobID, job.LastUpdatedOn)
				if err != nil {
					log.Printf("Dedup check error: %v", err)
					continue
				}

				switch result {
				case dedup.ResultUnchanged:
					unchangedJobs++
					continue
				case dedup.ResultUpdated:
					pageUpdated++
					log.Printf("[%s] Job %s updated, re-processing", c.Source(), jobID)
				case dedup.ResultNew:
					pageNew++
				}

				// Publish to queue (new or updated)
				if err := publisher.Publish(ctx, job); err != nil {
					log.Printf("Publish error: %v", err)
					continue
				}

				// Mark as seen with TTL based on expiredOn
				if err := deduplicator.MarkSeenWithTTL(ctx, string(c.Source()), jobID, job.LastUpdatedOn, job.ExpiredOn); err != nil {
					log.Printf("Mark seen error: %v", err)
				}
			}
			newJobs += pageNew
			updatedJobs += pageUpdated
			totalJobs += len(jobs)
			log.Printf("Crawler %s: page - %d new, %d updated, %d unchanged", c.Source(), pageNew, pageUpdated, len(jobs)-pageNew-pageUpdated)
			return nil
		})

		if err != nil {
			log.Printf("Crawler %s error: %v", c.Source(), err)
		}

		log.Printf("Crawler %s: %d total, %d new, %d updated, %d unchanged", c.Source(), totalJobs, newJobs, updatedJobs, unchangedJobs)
	}
}
