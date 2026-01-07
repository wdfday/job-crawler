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
	"github.com/project-tktt/go-crawler/internal/common/indexer"
	"github.com/project-tktt/go-crawler/internal/common/normalizer"
	"github.com/project-tktt/go-crawler/internal/config"
	"github.com/project-tktt/go-crawler/internal/module/worker"
	"github.com/project-tktt/go-crawler/internal/queue"
	"github.com/redis/go-redis/v9"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Job Worker Service")

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

	// Initialize Elasticsearch indexer
	esIndexer, err := indexer.NewElasticsearchIndexer(cfg.Elasticsearch.Addresses, cfg.Elasticsearch.Index)
	if err != nil {
		log.Fatalf("Elasticsearch connection failed: %v", err)
	}
	log.Printf("Elasticsearch connected, index: %s", cfg.Elasticsearch.Index)

	// Ensure index exists with proper mapping
	if err := esIndexer.EnsureIndex(ctx); err != nil {
		log.Printf("Warning: Failed to ensure index: %v", err)
	}

	// Initialize Components
	htmlCleaner := cleaner.NewCleaner()
	norm := normalizer.NewNormalizer()
	consumer := queue.NewConsumer(rdb, cfg.Redis.JobQueue, 5*time.Second)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	// Start worker pool (processes queue -> normalizes -> indexes to Elasticsearch)
	wg.Add(1)
	go func() {
		defer wg.Done()
		w := worker.NewWorker(consumer, norm, htmlCleaner, esIndexer, worker.Config{
			Concurrency: cfg.Worker.Concurrency,
			BatchSize:   cfg.Worker.BatchSize,
		})
		if err := w.Run(ctx); err != nil && err != context.Canceled {
			log.Printf("Worker error: %v", err)
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
