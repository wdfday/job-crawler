package extractor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/project-tktt/go-crawler/internal/domain"
)

// CollyExtractor implements Extractor using Colly for HTML scraping
type CollyExtractor struct {
	collector *colly.Collector
	config    ExtractorConfig
	source    domain.JobSource
	selectors Selectors
}

// Selectors defines CSS selectors for extracting job data
type Selectors struct {
	// List page selectors
	JobItem string
	JobLink string

	// Detail page selectors
	Title        string
	Company      string
	Location     string
	Salary       string
	Experience   string
	WorkType     string
	Industry     string
	Description  string
	Requirements string
	Benefits     string

	// For Next.js sites (TopCV) - extract from script tag
	NextDataScript string
}

// NewCollyExtractor creates a new Colly-based HTML scraper
func NewCollyExtractor(source domain.JobSource, selectors Selectors, config ExtractorConfig) *CollyExtractor {
	c := colly.NewCollector(
		colly.UserAgent(config.UserAgent),
		colly.AllowURLRevisit(),
	)

	// Configure rate limiting
	if config.RequestDelay > 0 {
		c.Limit(&colly.LimitRule{
			DomainGlob:  "*",
			Delay:       time.Duration(config.RequestDelay) * time.Millisecond,
			RandomDelay: time.Duration(config.RequestDelay/2) * time.Millisecond,
		})
	}

	// Set proxy if configured
	if config.ProxyURL != "" {
		c.SetProxy(config.ProxyURL)
	}

	return &CollyExtractor{
		collector: c,
		config:    config,
		source:    source,
		selectors: selectors,
	}
}

func (e *CollyExtractor) Name() string {
	return fmt.Sprintf("colly_%s", e.source)
}

func (e *CollyExtractor) Extract(ctx context.Context, url string) (*domain.RawJob, error) {
	var rawJob *domain.RawJob
	var extractErr error

	collector := e.collector.Clone()

	// Handle Next.js __NEXT_DATA__ for TopCV
	if e.selectors.NextDataScript != "" {
		collector.OnHTML(e.selectors.NextDataScript, func(el *colly.HTMLElement) {
			scriptContent := el.Text
			rawJob = &domain.RawJob{
				URL:         url,
				Source:      string(e.source),
				HTMLContent: scriptContent,
				RawData:     map[string]any{"__NEXT_DATA__": scriptContent},
				ExtractedAt: time.Now(),
			}
		})
	}

	// Standard HTML extraction
	collector.OnHTML("body", func(el *colly.HTMLElement) {
		if rawJob != nil {
			return // Already extracted from __NEXT_DATA__
		}

		rawData := make(map[string]any)

		if e.selectors.Title != "" {
			rawData["title"] = strings.TrimSpace(el.ChildText(e.selectors.Title))
		}
		if e.selectors.Company != "" {
			rawData["company"] = strings.TrimSpace(el.ChildText(e.selectors.Company))
		}
		if e.selectors.Location != "" {
			rawData["location"] = strings.TrimSpace(el.ChildText(e.selectors.Location))
		}
		if e.selectors.Salary != "" {
			rawData["salary"] = strings.TrimSpace(el.ChildText(e.selectors.Salary))
		}
		if e.selectors.Experience != "" {
			rawData["experience"] = strings.TrimSpace(el.ChildText(e.selectors.Experience))
		}
		if e.selectors.WorkType != "" {
			rawData["work_type"] = strings.TrimSpace(el.ChildText(e.selectors.WorkType))
		}
		if e.selectors.Industry != "" {
			rawData["industry"] = strings.TrimSpace(el.ChildText(e.selectors.Industry))
		}
		if e.selectors.Description != "" {
			desc, _ := el.DOM.Find(e.selectors.Description).Html()
			rawData["description"] = desc
		}
		if e.selectors.Requirements != "" {
			req, _ := el.DOM.Find(e.selectors.Requirements).Html()
			rawData["requirements"] = req
		}
		if e.selectors.Benefits != "" {
			ben, _ := el.DOM.Find(e.selectors.Benefits).Html()
			rawData["benefits"] = ben
		}

		rawJob = &domain.RawJob{
			URL:         url,
			Source:      string(e.source),
			RawData:     rawData,
			ExtractedAt: time.Now(),
		}
	})

	collector.OnError(func(r *colly.Response, err error) {
		extractErr = fmt.Errorf("colly error: %w (status: %d)", err, r.StatusCode)
	})

	if err := collector.Visit(url); err != nil {
		return nil, fmt.Errorf("visit url: %w", err)
	}

	if extractErr != nil {
		return nil, extractErr
	}

	if rawJob == nil {
		return nil, fmt.Errorf("no data extracted from %s", url)
	}

	return rawJob, nil
}

func (e *CollyExtractor) ExtractList(ctx context.Context, listURL string, page int) ([]*domain.RawJob, error) {
	var jobs []*domain.RawJob
	var extractErr error

	collector := e.collector.Clone()

	collector.OnHTML(e.selectors.JobItem, func(el *colly.HTMLElement) {
		link := el.ChildAttr(e.selectors.JobLink, "href")
		if link == "" {
			link = el.Attr("href")
		}

		// Make absolute URL if needed
		if !strings.HasPrefix(link, "http") {
			link = el.Request.AbsoluteURL(link)
		}

		jobs = append(jobs, &domain.RawJob{
			URL:         link,
			Source:      string(e.source),
			ExtractedAt: time.Now(),
		})
	})

	collector.OnError(func(r *colly.Response, err error) {
		extractErr = fmt.Errorf("colly error: %w", err)
	})

	url := fmt.Sprintf("%s?page=%d", listURL, page)
	if err := collector.Visit(url); err != nil {
		return nil, fmt.Errorf("visit list url: %w", err)
	}

	if extractErr != nil {
		return nil, extractErr
	}

	return jobs, nil
}
