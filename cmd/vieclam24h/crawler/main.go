package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/project-tktt/go-crawler/internal/common/dedup"
	"github.com/project-tktt/go-crawler/internal/config"
	"github.com/project-tktt/go-crawler/internal/module"
	vieclam24h "github.com/project-tktt/go-crawler/internal/module/vieclam24h"
	"github.com/project-tktt/go-crawler/internal/queue"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
)

const (
	PendingQueue = "jobs:pending:vieclam24h"
	// Cron schedule: every 6 hours at minute 0
	// Format: minute hour day-of-month month day-of-week
	CronSchedule = "0 */6 * * *"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Vieclam24h Crawler Service")

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
	pendingPub := queue.NewPublisher(rdb, PendingQueue)

	// Initialize Vieclam24h Crawler
	crawlerConfig := vieclam24h.DefaultConfig()
	if cfg.Crawler.RequestDelay > 0 {
		crawlerConfig.RequestDelay = cfg.Crawler.RequestDelay
	}

	// Enable verbose logging if env var is set
	if os.Getenv("CRAWLER_VERBOSE_LOG") == "true" || os.Getenv("CRAWLER_VERBOSE_LOG") == "1" {
		crawlerConfig.VerboseLog = true
		log.Println("Verbose logging enabled")
	}

	vl24hCrawler := vieclam24h.NewCrawler(
		crawlerConfig,
		deduplicator,
		pendingPub,
	)

	// Setup cron scheduler
	c := cron.New(cron.WithLogger(cron.VerbosePrintfLogger(log.Default())))

	// Add crawler job
	_, err := c.AddFunc(CronSchedule, func() {
		runCrawler(ctx, vl24hCrawler)
	})
	if err != nil {
		log.Fatalf("Failed to add cron job: %v", err)
	}

	log.Printf("Cron scheduled: %s", CronSchedule)

	// Run immediately on startup
	go runCrawler(ctx, vl24hCrawler)

	// Start cron scheduler
	c.Start()
	log.Printf("Cron started. Next run: %v", c.Entries()[0].Next)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutdown signal received, stopping...")

	// Stop cron scheduler
	cronCtx := c.Stop()
	<-cronCtx.Done()

	cancel()
	time.Sleep(1 * time.Second)
	log.Println("Graceful shutdown complete")
}

func runCrawler(ctx context.Context, c module.Crawler) {
	log.Printf("[Cron] Running crawler: %s", c.Source())
	start := time.Now()

	if _, err := c.Crawl(ctx); err != nil {
		log.Printf("[Cron] Crawler %s error: %v", c.Source(), err)
	}

	log.Printf("[Cron] Crawler %s finished in %v", c.Source(), time.Since(start))
}
