package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/project-tktt/go-crawler/internal/common/dedup"
	"github.com/project-tktt/go-crawler/internal/config"
	"github.com/project-tktt/go-crawler/internal/domain"
	"github.com/project-tktt/go-crawler/internal/module"
	vietnamworks2 "github.com/project-tktt/go-crawler/internal/module/vietnamworks"
	"github.com/project-tktt/go-crawler/internal/queue"
	"github.com/redis/go-redis/v9"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting VietnamWorks Crawler Service")

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

	// Initialize Components
	deduplicator := dedup.NewDeduplicator(rdb, "job:seen", 30*24*time.Hour)
	publisher := queue.NewPublisher(rdb, cfg.Redis.JobQueue)

	// Initialize VietnamWorks Crawler
	vnwCrawler := vietnamworks2.NewCrawler(
		vietnamworks2.Config{MaxPages: 1000, RequestDelay: cfg.Crawler.RequestDelay},
	)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	// Run crawler scheduler
	go runCrawlerScheduler(ctx, vnwCrawler, deduplicator, publisher)

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutdown signal received, stopping...")
	cancel()
	time.Sleep(1 * time.Second) // Give some time for cleanup
	log.Println("Graceful shutdown complete")
}

// runCrawlerScheduler runs the crawler periodically
func runCrawlerScheduler(ctx context.Context, c module.Crawler, deduplicator *dedup.Deduplicator, publisher *queue.Publisher) {
	// Run immediately
	runCrawler(ctx, c, deduplicator, publisher)

	// Schedule every hour
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runCrawler(ctx, c, deduplicator, publisher)
		}
	}
}

func runCrawler(ctx context.Context, c module.Crawler, deduplicator *dedup.Deduplicator, publisher *queue.Publisher) {
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

	log.Printf("Crawler %s finished cycle: %d total, %d new, %d updated, %d unchanged", c.Source(), totalJobs, newJobs, updatedJobs, unchangedJobs)
}
