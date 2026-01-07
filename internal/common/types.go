package common

import "time"

// EnrichedJob represents job data after scraping detail page and extracting JSON-LD
// This type contains structured data ready for further processing
type EnrichedJob struct {
	// Identification
	ID        string `json:"id"`
	URL       string `json:"url"`
	Canonical string `json:"canonical_url"`
	Source    string `json:"source"`

	// Basic Info (from API + JSON-LD)
	Title          string `json:"title"`
	Description    string `json:"description"`
	Requirements   string `json:"requirements"`
	Benefits       string `json:"benefits"`
	Skills         string `json:"skills"`
	Qualifications string `json:"qualifications"`

	// Company Info
	Company CompanyInfo `json:"company"`

	// Location
	Location LocationInfo `json:"location"`

	// Salary
	Salary SalaryInfo `json:"salary"`

	// Classification
	Industry        string `json:"industry"`
	Position        string `json:"position"` // occupational_category
	EmploymentType  string `json:"employment_type"`
	ExperienceRange string `json:"experience_range"`
	DegreeRequired  string `json:"degree_required"`
	Gender          string `json:"gender"`
	TotalOpenings   int    `json:"total_openings"`

	// Dates
	DatePosted   time.Time `json:"date_posted"`
	ValidThrough time.Time `json:"valid_through"`
	ExtractedAt  time.Time `json:"extracted_at"`

	// Raw data for reference
	RawAPIData map[string]any `json:"raw_api_data,omitempty"`
}

// CompanyInfo holds employer information
type CompanyInfo struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Logo string `json:"logo"`
	URL  string `json:"url"`
}

// LocationInfo holds job location details
type LocationInfo struct {
	FullAddress string `json:"full_address"`
	Street      string `json:"street"`
	District    string `json:"district"`
	Province    string `json:"province"`
	Country     string `json:"country"`
	ProvinceIDs []int  `json:"province_ids,omitempty"`
	DistrictIDs []int  `json:"district_ids,omitempty"`
}

// SalaryInfo holds salary details
type SalaryInfo struct {
	Text       string `json:"text"`     // Display text like "8 - 15 triệu"
	Min        int    `json:"min"`      // In VND
	Max        int    `json:"max"`      // In VND
	Currency   string `json:"currency"` // VND
	Unit       string `json:"unit"`     // MONTH, YEAR, etc.
	Negotiable bool   `json:"negotiable"`
}
