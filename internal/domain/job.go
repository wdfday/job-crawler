package domain

import "time"

// Job represents a normalized job posting from any source
type Job struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Company      string    `json:"company"`
	Location     string    `json:"location"`
	Position     string    `json:"position"`
	Salary       string    `json:"salary"`
	SalaryMin    int       `json:"salary_min"`
	SalaryMax    int       `json:"salary_max"`
	IsNegotiable bool      `json:"is_negotiable"` // Salary is negotiable
	WorkType     string    `json:"work_type"`
	Industry     []string  `json:"industry"` // Array for multiple industries
	Field        string    `json:"field"`
	Experience   string    `json:"experience"`
	ExpTags      []string  `json:"experience_tags"`
	Description  string    `json:"description"`
	Requirements string    `json:"requirements"`
	Benefits     string    `json:"benefits"`
	Source       string    `json:"source"` // topcv, vietnamworks, careerviet
	SourceURL    string    `json:"source_url"`
	CrawledAt    time.Time `json:"crawled_at"`

	// New enriched fields
	TotalViews           int       `json:"total_views"`
	TotalResumeApplied   int       `json:"total_resume_applied"`
	RateResponse         float64   `json:"rate_response"`
	Skills               []string  `json:"skills"`
	Qualifications       string    `json:"qualifications"`
	CompanyWebsite       string    `json:"company_website"`
	OccupationalCategory string    `json:"occupational_category"`
	EmploymentType       string    `json:"employment_type"`
	LocationCity         []string  `json:"location_city"`     // Province/City (array)
	LocationDistrict     []string  `json:"location_district"` // District (array)
	ExpiredAt            time.Time `json:"expired_at"`

	// Source timestamps
	CreatedAt time.Time `json:"created_at"` // When job was created on source
	UpdatedAt time.Time `json:"updated_at"` // When job was last updated on source
}

// RawJob represents raw extracted data before normalization
type RawJob struct {
	ID            string         `json:"id"`
	URL           string         `json:"url"`
	Source        string         `json:"source"`
	RawData       map[string]any `json:"raw_data"`
	HTMLContent   string         `json:"html_content,omitempty"`
	ExtractedAt   time.Time      `json:"extracted_at"`
	LastUpdatedOn string         `json:"last_updated_on,omitempty"` // For change detection
	ExpiredOn     time.Time      `json:"expired_on,omitempty"`      // For TTL calculation
}

// JobSource represents a job listing source
type JobSource string

const (
	SourceTopCV        JobSource = "topcv"
	SourceVietnamWorks JobSource = "vietnamworks"
	SourceCareerViet   JobSource = "careerviet"
	SourceTopDev       JobSource = "topdev"
	SourceVieclam24h   JobSource = "vieclam24h"
)
