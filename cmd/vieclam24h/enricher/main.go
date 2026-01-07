package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/project-tktt/go-crawler/internal/config"
	"github.com/project-tktt/go-crawler/internal/module/vieclam24h"
	"github.com/project-tktt/go-crawler/internal/queue"
	"github.com/redis/go-redis/v9"
)

const (
	PendingQueue = "jobs:pending:vieclam24h"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Vieclam24h Enricher Service")

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

	// Queues
	// Consumer: Pending Queue (from crawler)
	pendingCons := queue.NewConsumer(rdb, PendingQueue, 5*time.Second)

	// Producer: Raw Queue (to worker) - Configurable via env
	rawQueueName := cfg.Redis.JobQueue
	log.Printf("Output queue: %s", rawQueueName)
	rawPub := queue.NewPublisher(rdb, rawQueueName)

	// Producer: JSON-LD Validation Queue
	jsonLdQueueName := "jobs:jsonld:vieclam24h"
	log.Printf("JSON-LD validation queue: %s", jsonLdQueueName)
	jsonLdPub := queue.NewPublisher(rdb, jsonLdQueueName)

	// Initialize Detail Scraper (Consumer Pending -> Producer Raw + JSON-LD)
	vl24hScraper := vieclam24h.NewScraper(pendingCons, rawPub, jsonLdPub, cfg.Crawler.RequestDelay)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	// Start Enricher Loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := vl24hScraper.Run(ctx); err != nil && err != context.Canceled {
			log.Printf("Enricher error: %v", err)
		}
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
