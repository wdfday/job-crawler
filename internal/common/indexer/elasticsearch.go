package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/project-tktt/go-crawler/internal/domain"
)

// ElasticsearchIndexer indexes jobs to Elasticsearch
type ElasticsearchIndexer struct {
	client    *elasticsearch.Client
	indexName string
}

// NewElasticsearchIndexer creates a new Elasticsearch indexer
func NewElasticsearchIndexer(addresses []string, indexName string) (*ElasticsearchIndexer, error) {
	cfg := elasticsearch.Config{
		Addresses: addresses,
	}

	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("create es client: %w", err)
	}

	// Check connection
	res, err := client.Info()
	if err != nil {
		return nil, fmt.Errorf("es info: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("es error: %s", res.Status())
	}

	return &ElasticsearchIndexer{
		client:    client,
		indexName: indexName,
	}, nil
}

// Index indexes a single job
func (i *ElasticsearchIndexer) Index(ctx context.Context, job *domain.Job) error {
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}

	req := esapi.IndexRequest{
		Index:      i.indexName,
		DocumentID: job.ID,
		Body:       bytes.NewReader(data),
		Refresh:    "false",
	}

	res, err := req.Do(ctx, i.client)
	if err != nil {
		return fmt.Errorf("index request: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("index error: %s", res.Status())
	}

	return nil
}

// BulkIndex indexes multiple jobs at once
func (i *ElasticsearchIndexer) BulkIndex(ctx context.Context, jobs []*domain.Job) error {
	if len(jobs) == 0 {
		return nil
	}

	var buf bytes.Buffer

	for _, job := range jobs {
		// Meta line
		meta := map[string]any{
			"index": map[string]any{
				"_index": i.indexName,
				"_id":    job.ID,
			},
		}
		metaBytes, _ := json.Marshal(meta)
		buf.Write(metaBytes)
		buf.WriteByte('\n')

		// Document line
		docBytes, err := json.Marshal(job)
		if err != nil {
			log.Printf("marshal job %s: %v", job.ID, err)
			continue
		}
		buf.Write(docBytes)
		buf.WriteByte('\n')
	}

	res, err := i.client.Bulk(bytes.NewReader(buf.Bytes()), i.client.Bulk.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("bulk request: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("bulk error: %s", res.Status())
	}

	// Parse response to check for individual errors
	var bulkRes struct {
		Errors bool `json:"errors"`
		Items  []struct {
			Index struct {
				ID     string `json:"_id"`
				Status int    `json:"status"`
				Error  struct {
					Type   string `json:"type"`
					Reason string `json:"reason"`
				} `json:"error"`
			} `json:"index"`
		} `json:"items"`
	}

	if err := json.NewDecoder(res.Body).Decode(&bulkRes); err != nil {
		return fmt.Errorf("parse bulk response: %w", err)
	}

	if bulkRes.Errors {
		for _, item := range bulkRes.Items {
			if item.Index.Status >= 400 {
				log.Printf("bulk index error for %s: %s - %s",
					item.Index.ID, item.Index.Error.Type, item.Index.Error.Reason)
			}
		}
	}

	return nil
}

// EnsureIndex creates the index with Vietnamese-friendly settings if it doesn't exist
func (i *ElasticsearchIndexer) EnsureIndex(ctx context.Context) error {
	// Check if index exists
	res, err := i.client.Indices.Exists([]string{i.indexName})
	if err != nil {
		return fmt.Errorf("check index: %w", err)
	}
	res.Body.Close()

	if res.StatusCode == 200 {
		return nil // Index already exists
	}

	// Create index with Vietnamese analyzer settings
	mapping := `{
		"settings": {
			"analysis": {
				"analyzer": {
					"vietnamese_analyzer": {
						"type": "custom",
						"tokenizer": "standard",
						"filter": ["lowercase", "asciifolding"]
					}
				}
			}
		},
		"mappings": {
			"properties": {
				"id": {"type": "keyword"},
				"title": {
					"type": "text",
					"analyzer": "vietnamese_analyzer",
					"fields": {"keyword": {"type": "keyword"}}
				},
				"company": {"type": "text", "analyzer": "vietnamese_analyzer"},
				"location": {"type": "text", "analyzer": "vietnamese_analyzer"},
				"location_city": {"type": "keyword"},
				"location_district": {"type": "keyword"},
				"position": {"type": "keyword"},
				"salary": {"type": "text", "fields": {"keyword": {"type": "keyword"}}},
				"salary_min": {"type": "integer"},
				"salary_max": {"type": "integer"},
				"is_negotiable": {"type": "boolean"},
				"work_type": {"type": "keyword"},
				"industry": {"type": "keyword"},
				"experience": {"type": "keyword"},
				"experience_tags": {"type": "keyword"},
				"qualifications": {"type": "keyword"},
				"description": {"type": "text", "analyzer": "vietnamese_analyzer"},
				"requirements": {"type": "text", "analyzer": "vietnamese_analyzer"},
				"benefits": {"type": "text", "analyzer": "vietnamese_analyzer"},
				"skills": {"type": "keyword"},
				"source": {"type": "keyword"},
				"source_url": {"type": "keyword"},
				"expired_at": {"type": "date"},
				"crawled_at": {"type": "date"}
			}
		}
	}`

	res, err = i.client.Indices.Create(
		i.indexName,
		i.client.Indices.Create.WithBody(bytes.NewReader([]byte(mapping))),
	)
	if err != nil {
		return fmt.Errorf("create index: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("create index error: %s", res.Status())
	}

	return nil
}
