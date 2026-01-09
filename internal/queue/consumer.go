package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/project-tktt/go-crawler/internal/domain"
	"github.com/redis/go-redis/v9"
)

// Consumer consumes jobs from Redis queue
type Consumer struct {
	client    *redis.Client
	queueName string
	timeout   time.Duration
}

// NewConsumer creates a new queue consumer
func NewConsumer(client *redis.Client, queueName string, timeout time.Duration) *Consumer {
	if queueName == "" {
		queueName = "jobs:raw"
	}
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &Consumer{
		client:    client,
		queueName: queueName,
		timeout:   timeout,
	}
}

// Consume blocks and waits for a job from the queue
// Returns nil, nil if timeout occurs with no job
func (c *Consumer) Consume(ctx context.Context) (*domain.RawJob, error) {
	result, err := c.client.BRPop(ctx, c.timeout, c.queueName).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Timeout, no job available
		}
		return nil, fmt.Errorf("brpop: %w", err)
	}

	if len(result) < 2 {
		return nil, nil
	}

	var job domain.RawJob
	if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
		return nil, fmt.Errorf("unmarshal job: %w", err)
	}

	return &job, nil
}

// ConsumeBatch consumes up to maxBatch jobs from the queue
// Uses BRPOP to block-wait for first item (prevents CPU spinning)
// Then uses RPOP to quickly grab remaining items for the batch
func (c *Consumer) ConsumeBatch(ctx context.Context, maxBatch int) ([]*domain.RawJob, error) {
	jobs := make([]*domain.RawJob, 0, maxBatch)

	// First item: use BRPOP to block until available (prevents CPU spinning)
	result, err := c.client.BRPop(ctx, c.timeout, c.queueName).Result()
	if err != nil {
		if err == redis.Nil {
			return jobs, nil // Timeout, no jobs
		}
		return nil, fmt.Errorf("brpop: %w", err)
	}

	if len(result) >= 2 {
		var job domain.RawJob
		if err := json.Unmarshal([]byte(result[1]), &job); err == nil {
			jobs = append(jobs, &job)
		}
	}

	// Remaining items: use non-blocking RPOP to fill the batch
	for i := 1; i < maxBatch; i++ {
		result, err := c.client.RPop(ctx, c.queueName).Result()
		if err != nil {
			if err == redis.Nil {
				break // No more jobs
			}
			return jobs, fmt.Errorf("rpop: %w", err)
		}

		var job domain.RawJob
		if err := json.Unmarshal([]byte(result), &job); err != nil {
			continue // Skip malformed jobs
		}

		jobs = append(jobs, &job)
	}

	return jobs, nil
}

// Run starts a continuous consumer loop
func (c *Consumer) Run(ctx context.Context, handler func(*domain.RawJob) error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		job, err := c.Consume(ctx)
		if err != nil {
			return fmt.Errorf("consume: %w", err)
		}

		if job == nil {
			continue // Timeout, try again
		}

		if err := handler(job); err != nil {
			// Log error but continue processing
			fmt.Printf("handler error: %v\n", err)
		}
	}
}
