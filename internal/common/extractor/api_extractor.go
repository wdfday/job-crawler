package extractor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/project-tktt/go-crawler/internal/domain"
)

// APIExtractor implements Extractor for API-based sources
type APIExtractor struct {
	client    *http.Client
	config    ExtractorConfig
	source    domain.JobSource
	baseURL   string
	parseFunc func(data []byte) ([]*domain.RawJob, error)
}

// NewAPIExtractor creates a new API-based extractor
func NewAPIExtractor(source domain.JobSource, baseURL string, config ExtractorConfig) *APIExtractor {
	transport := &http.Transport{}

	// Note: Proxy configuration would be added here if ProxyURL is set
	// For residential proxy support (VietnamWorks)

	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}

	return &APIExtractor{
		client:  client,
		config:  config,
		source:  source,
		baseURL: baseURL,
	}
}

// SetParseFunc sets a custom parsing function for the API response
func (e *APIExtractor) SetParseFunc(fn func(data []byte) ([]*domain.RawJob, error)) {
	e.parseFunc = fn
}

func (e *APIExtractor) Name() string {
	return fmt.Sprintf("api_%s", e.source)
}

func (e *APIExtractor) Extract(ctx context.Context, url string) (*domain.RawJob, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	e.setHeaders(req)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var rawData map[string]any
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	return &domain.RawJob{
		URL:         url,
		Source:      string(e.source),
		RawData:     rawData,
		ExtractedAt: time.Now(),
	}, nil
}

func (e *APIExtractor) ExtractList(ctx context.Context, listURL string, page int) ([]*domain.RawJob, error) {
	url := fmt.Sprintf("%s?page=%d", listURL, page)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	e.setHeaders(req)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	// Use custom parse function if set
	if e.parseFunc != nil {
		return e.parseFunc(body)
	}

	// Default: try to parse as array of objects
	var items []map[string]any
	if err := json.Unmarshal(body, &items); err != nil {
		// Try nested structure
		var wrapper struct {
			Data []map[string]any `json:"data"`
		}
		if err := json.Unmarshal(body, &wrapper); err != nil {
			return nil, fmt.Errorf("parse list json: %w", err)
		}
		items = wrapper.Data
	}

	jobs := make([]*domain.RawJob, 0, len(items))
	for _, item := range items {
		id := ""
		if v, ok := item["id"]; ok {
			id = fmt.Sprintf("%v", v)
		}
		jobs = append(jobs, &domain.RawJob{
			ID:          id,
			Source:      string(e.source),
			RawData:     item,
			ExtractedAt: time.Now(),
		})
	}

	return jobs, nil
}

func (e *APIExtractor) setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", e.config.UserAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9,en;q=0.8")
}
