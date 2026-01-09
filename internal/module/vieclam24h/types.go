package vieclam24h

import "time"

// Config holds crawler configuration
type Config struct {
	MaxPages     int
	PerPage      int
	RequestDelay time.Duration
	UserAgent    string
	BearerToken  string
	Branch       string // "vl24h.north" or "vl24h.south"
	VerboseLog   bool   // Log every job with status and URL
}

// DefaultConfig returns default configuration
func DefaultConfig() Config {
	return Config{
		MaxPages:     50,
		PerPage:      30,
		RequestDelay: 2 * time.Second,
		UserAgent:    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
		BearerToken:  "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJjaGFubmVsX2NvZGUiOiJ2bDI0aCIsInVzZXIiOm51bGx9.a0POm2ZVRwetYs2QsMj0sRg8lZSSbKufX4sewqhAM5o",
		Branch:       "vl24h.north",
		VerboseLog:   false, // Disabled by default
	}
}

// ==================== API Response Types ====================

// APIResponse represents the API response structure
type APIResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Items      []JobItem  `json:"items"`
		Pagination Pagination `json:"pagination"`
	} `json:"data"`
}

// Pagination holds pagination info
type Pagination struct {
	CurrentPage int `json:"current_page"`
	LastPage    int `json:"last_page"`
	PerPage     int `json:"per_page"`
	Total       int `json:"total"`
}

// JobItem represents a job from the API
type JobItem struct {
	ID                   int          `json:"id"`
	ChannelCode          string       `json:"channel_code"`
	Title                string       `json:"title"`
	TitleSlug            string       `json:"title_slug"`
	LevelRequirement     int          `json:"level_requirement"`
	OccupationIDs        []int        `json:"occupation_ids_main"`
	FieldIDMain          int          `json:"field_ids_main"`
	FieldIDsSub          any          `json:"field_ids_sub"`
	ProvinceIDs          []int        `json:"province_ids"`
	DistrictIDs          []int        `json:"district_ids"`
	EmployerID           int          `json:"employer_id"`
	EmployerInfo         EmployerInfo `json:"employer_info"`
	VacancyQuantity      int          `json:"vacancy_quantity"`
	WorkingMethod        int          `json:"working_method"`
	SalaryUnit           int          `json:"salary_unit"`
	ResumeApplyExpired   int64        `json:"resume_apply_expired"`
	DegreeRequirement    int          `json:"degree_requirement"`
	Gender               int          `json:"gender"`
	ExperienceRange      int          `json:"experience_range"`
	Status               int          `json:"status"`
	CreatedAt            int64        `json:"created_at"`
	UpdatedAt            int64        `json:"updated_at"`
	TotalViews           int          `json:"total_views"`
	TotalResumeApplied   int          `json:"total_resume_applied"`
	ContactName          string       `json:"contact_name"`
	ContactAddress       string       `json:"contact_address"`
	JobRequirement       string       `json:"job_requirement"`
	JobRequirementHTML   string       `json:"job_requirement_html"`
	OtherRequirement     string       `json:"other_requirement"`
	OtherRequirementHTML string       `json:"other_requirement_html"`
	JobBox               []JobBox     `json:"job_box"`
	SalaryFrom           int          `json:"salary_from"`
	SalaryTo             int          `json:"salary_to"`
	SalaryText           string       `json:"salary_text"`
}

// EmployerInfo holds company information
type EmployerInfo struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	Status       int    `json:"status"`
	Logo         string `json:"logo"`
	RateResponse int    `json:"rate_response"`
}

// JobBox holds job description content
type JobBox struct {
	ID          int    `json:"id"`
	ServiceCode string `json:"service_code"`
	Content     string `json:"content"`
	ContentHTML string `json:"content_html"`
}

// ==================== JSON-LD Types (from detail page) ====================

// JobPosting represents JSON-LD structured data from detail page
type JobPosting struct {
	Context              string       `json:"@context"`
	Type                 string       `json:"@type"`
	Title                string       `json:"title"`
	Description          string       `json:"description"`
	JobBenefits          string       `json:"jobBenefits"`
	Skills               string       `json:"skills"`
	Qualifications       string       `json:"qualifications"`
	Industry             string       `json:"industry"`
	TotalJobOpenings     int          `json:"totalJobOpenings"`
	DatePosted           string       `json:"datePosted"`
	ValidThrough         string       `json:"validThrough"`
	EmploymentType       string       `json:"employmentType"`
	OccupationalCategory string       `json:"occupationalCategory"`
	Identifier           Identifier   `json:"identifier"`
	HiringOrganization   Organization `json:"hiringOrganization"`
	JobLocation          []Location   `json:"jobLocation"`
	BaseSalary           BaseSalary   `json:"baseSalary"`
}

// Identifier holds job identifier info
type Identifier struct {
	Type  string `json:"@type"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Organization holds company info from JSON-LD
type Organization struct {
	Type   string `json:"@type"`
	Name   string `json:"name"`
	SameAs string `json:"sameAs"`
	Logo   string `json:"logo"`
}

// Location holds job location from JSON-LD
type Location struct {
	Type    string  `json:"@type"`
	Address Address `json:"address"`
}

// Address holds address details from JSON-LD
type Address struct {
	Type            string `json:"@type"`
	StreetAddress   string `json:"streetAddress"`
	AddressLocality string `json:"addressLocality"`
	AddressRegion   string `json:"addressRegion"`
	PostalCode      string `json:"postalCode"`
	AddressCountry  string `json:"addressCountry"`
}

// BaseSalary holds salary info from JSON-LD
type BaseSalary struct {
	Type     string      `json:"@type"`
	Currency string      `json:"currency"`
	Value    SalaryValue `json:"value"`
}

// SalaryValue holds salary range from JSON-LD
type SalaryValue struct {
	Type     string `json:"@type"`
	UnitText string `json:"unitText"`
	MinValue int    `json:"minValue"`
	MaxValue int    `json:"maxValue"`
	Value    string `json:"value"` // For negotiable: "Thỏa thuận"
}
