package dedup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Deduplicator checks and tracks seen jobs using Redis
type Deduplicator struct {
	client     *redis.Client
	prefix     string
	defaultTTL time.Duration
}

// NewDeduplicator creates a new Redis-based deduplicator
func NewDeduplicator(client *redis.Client, prefix string, defaultTTL time.Duration) *Deduplicator {
	if prefix == "" {
		prefix = "dedup"
	}
	if defaultTTL == 0 {
		defaultTTL = 24 * time.Hour * 30 // 30 days default
	}
	return &Deduplicator{
		client:     client,
		prefix:     prefix,
		defaultTTL: defaultTTL,
	}
}

// CheckResult represents the result of checking a job
type CheckResult int

const (
	// ResultNew - job has never been seen
	ResultNew CheckResult = iota
	// ResultUpdated - job exists but has been updated
	ResultUpdated
	// ResultUnchanged - job exists and is unchanged
	ResultUnchanged
)

// CheckJob checks if a job needs to be processed
// Returns ResultNew if never seen, ResultUpdated if changed, ResultUnchanged if same
func (d *Deduplicator) CheckJob(ctx context.Context, source, jobID, lastUpdatedOn string) (CheckResult, error) {
	key := d.makeKey(source, jobID)

	storedValue, err := d.client.Get(ctx, key).Result()
	if err == redis.Nil {
		// Key doesn't exist - new job
		return ResultNew, nil
	}
	if err != nil {
		return ResultNew, fmt.Errorf("redis get: %w", err)
	}

	// Key exists - check if updated
	if storedValue != lastUpdatedOn {
		return ResultUpdated, nil
	}

	return ResultUnchanged, nil
}

// MarkSeenWithTTL marks a job as seen with custom TTL based on expiredOn
// lastUpdatedOn is stored as the value for change detection
// expiredOn is used to calculate TTL
func (d *Deduplicator) MarkSeenWithTTL(ctx context.Context, source, jobID, lastUpdatedOn string, expiredOn time.Time) error {
	key := d.makeKey(source, jobID)

	// Calculate TTL from expiredOn
	ttl := time.Until(expiredOn)
	if ttl <= 0 {
		// Already expired, use default TTL
		ttl = d.defaultTTL
	}
	// Add 1 day buffer after expiry
	ttl += 24 * time.Hour

	err := d.client.Set(ctx, key, lastUpdatedOn, ttl).Err()
	if err != nil {
		return fmt.Errorf("redis set: %w", err)
	}
	return nil
}

// IsSeen checks if a job URL/ID has been seen before (legacy method)
func (d *Deduplicator) IsSeen(ctx context.Context, source, jobID string) (bool, error) {
	key := d.makeKey(source, jobID)
	exists, err := d.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("redis exists: %w", err)
	}
	return exists > 0, nil
}

// MarkSeen marks a job as seen with default TTL (legacy method)
func (d *Deduplicator) MarkSeen(ctx context.Context, source, jobID string) error {
	key := d.makeKey(source, jobID)
	err := d.client.Set(ctx, key, time.Now().Unix(), d.defaultTTL).Err()
	if err != nil {
		return fmt.Errorf("redis set: %w", err)
	}
	return nil
}

// IsSeenByContent checks if content has been seen before (content-based dedup)
func (d *Deduplicator) IsSeenByContent(ctx context.Context, source, content string) (bool, error) {
	hash := d.hashContent(content)
	return d.IsSeen(ctx, source, "content:"+hash)
}

// MarkSeenByContent marks content hash as seen
func (d *Deduplicator) MarkSeenByContent(ctx context.Context, source, content string) error {
	hash := d.hashContent(content)
	return d.MarkSeen(ctx, source, "content:"+hash)
}

func (d *Deduplicator) makeKey(source, id string) string {
	return fmt.Sprintf("%s:%s:%s", d.prefix, source, id)
}

func (d *Deduplicator) hashContent(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:16]) // First 16 bytes (32 hex chars)
}
