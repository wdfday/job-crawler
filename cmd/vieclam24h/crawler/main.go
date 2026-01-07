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
	vieclam24h2 "github.com/project-tktt/go-crawler/internal/module/vieclam24h"
	"github.com/project-tktt/go-crawler/internal/queue"
	"github.com/redis/go-redis/v9"
)

const (
	PendingQueue = "jobs:pending:vieclam24h"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Vieclam24h List Crawler Service")

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

	// Initialize Vieclam24h Crawler (Producer -> Pending Queue)
	crawlerConfig := vieclam24h2.DefaultConfig()
	if cfg.Crawler.RequestDelay > 0 {
		crawlerConfig.RequestDelay = cfg.Crawler.RequestDelay
	}

	vl24hCrawler := vieclam24h2.NewCrawler(
		crawlerConfig,
		deduplicator,
		pendingPub,
	)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run crawler scheduler
	go runCrawlerScheduler(ctx, vl24hCrawler)

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutdown signal received, stopping...")
	cancel()

	time.Sleep(1 * time.Second)
	log.Println("Graceful shutdown complete")
}

// runCrawlerScheduler runs the crawler periodically
func runCrawlerScheduler(ctx context.Context, c module.Crawler) {
	// Run immediately
	runCrawler(ctx, c)

	// Schedule every hour
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runCrawler(ctx, c)
		}
	}
}

func runCrawler(ctx context.Context, c module.Crawler) {
	log.Printf("Running crawler: %s", c.Source())
	if _, err := c.Crawl(ctx); err != nil {
		log.Printf("Crawler %s error: %v", c.Source(), err)
	}
	log.Printf("Crawler %s finished cycle", c.Source())
}
