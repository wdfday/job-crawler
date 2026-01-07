package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the crawler system
type Config struct {
	Redis         RedisConfig
	Elasticsearch ESConfig
	Postgres      PostgresConfig
	Crawler       CrawlerConfig
	Worker        WorkerConfig
}

type PostgresConfig struct {
	// Connection string (e.g. postgres://user:pass@localhost:5432/dbname?sslmode=disable)
	ConnectionString string
	// Table name for jobs
	TableName string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
	// Queue names
	JobQueue string
}

type ESConfig struct {
	Addresses []string
	Index     string
}

type CrawlerConfig struct {
	// Rate limiting
	RequestDelay time.Duration
	MaxRetries   int
	// Proxy settings (for VietnamWorks)
	ProxyURL string
	// User agent
	UserAgent string
}

type WorkerConfig struct {
	// Number of concurrent workers
	Concurrency int
	// Batch size for Elasticsearch bulk indexing
	BatchSize int
}

// Load creates a Config from environment variables with defaults
func Load() *Config {
	return &Config{
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
			JobQueue: getEnv("REDIS_JOB_QUEUE", "jobs:raw"),
		},
		Elasticsearch: ESConfig{
			Addresses: []string{getEnv("ELASTICSEARCH_URL", "http://localhost:9200")},
			Index:     getEnv("ELASTICSEARCH_INDEX", "jobs"),
		},
		Postgres: PostgresConfig{
			ConnectionString: getEnv("POSTGRES_URL", "postgres://postgres:postgres@localhost:5432/jobs?sslmode=disable"),
			TableName:        getEnv("POSTGRES_TABLE", "Vieclam24hJob"),
		},
		Crawler: CrawlerConfig{
			RequestDelay: time.Duration(getEnvInt("CRAWLER_DELAY_MS", 1000)) * time.Millisecond,
			MaxRetries:   getEnvInt("CRAWLER_MAX_RETRIES", 3),
			ProxyURL:     getEnv("PROXY_URL", ""),
			UserAgent:    getEnv("USER_AGENT", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"),
		},
		Worker: WorkerConfig{
			Concurrency: getEnvInt("WORKER_CONCURRENCY", 5),
			BatchSize:   getEnvInt("WORKER_BATCH_SIZE", 100),
		},
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}
