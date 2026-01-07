package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/project-tktt/go-crawler/internal/domain"
	"github.com/redis/go-redis/v9"
)

// Publisher pushes jobs to Redis queue
type Publisher struct {
	client    *redis.Client
	queueName string
}

// NewPublisher creates a new queue publisher
func NewPublisher(client *redis.Client, queueName string) *Publisher {
	if queueName == "" {
		queueName = "jobs:raw"
	}
	return &Publisher{
		client:    client,
		queueName: queueName,
	}
}

// Publish pushes a single job to the queue
func (p *Publisher) Publish(ctx context.Context, job *domain.RawJob) error {
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}

	if err := p.client.LPush(ctx, p.queueName, data).Err(); err != nil {
		return fmt.Errorf("lpush: %w", err)
	}

	return nil
}

// PublishBatch pushes multiple jobs to the queue
func (p *Publisher) PublishBatch(ctx context.Context, jobs []*domain.RawJob) error {
	if len(jobs) == 0 {
		return nil
	}

	pipe := p.client.Pipeline()
	for _, job := range jobs {
		data, err := json.Marshal(job)
		if err != nil {
			return fmt.Errorf("marshal job: %w", err)
		}
		pipe.LPush(ctx, p.queueName, data)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("pipeline exec: %w", err)
	}

	return nil
}

// QueueLength returns the current queue length
func (p *Publisher) QueueLength(ctx context.Context) (int64, error) {
	return p.client.LLen(ctx, p.queueName).Result()
}

// PublishRaw pushes arbitrary data to the queue (for validation/debugging)
func (p *Publisher) PublishRaw(ctx context.Context, data any) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal data: %w", err)
	}

	if err := p.client.LPush(ctx, p.queueName, jsonData).Err(); err != nil {
		return fmt.Errorf("lpush: %w", err)
	}

	return nil
}
