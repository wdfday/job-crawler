package normalizer

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/project-tktt/go-crawler/internal/domain"
)

// Normalizer converts RawJob to normalized Job format
type Normalizer struct{}

// NewNormalizer creates a new normalizer
func NewNormalizer() *Normalizer {
	return &Normalizer{}
}

// Normalize converts a RawJob to a standardized Job
func (n *Normalizer) Normalize(raw *domain.RawJob) (*domain.Job, error) {
	data := raw.RawData

	job := &domain.Job{
		ID:        raw.ID,
		Source:    raw.Source,
		SourceURL: raw.URL,
		CrawledAt: raw.ExtractedAt,
	}

	// Normalize based on source
	switch raw.Source {
	case string(domain.SourceVietnamWorks):
		n.normalizeVietnamWorks(job, data)
	case string(domain.SourceTopDev):
		n.normalizeTopDev(job, data)
	case string(domain.SourceVieclam24h):
		n.normalizeVieclam24h(job, data)
	default:
		n.normalizeGeneric(job, data)
	}

	// Decode HTML entities in all text fields
	job.Title = html.UnescapeString(job.Title)
	job.Company = html.UnescapeString(job.Company)
	job.Location = html.UnescapeString(job.Location)
	job.Description = html.UnescapeString(job.Description)
	job.Requirements = html.UnescapeString(job.Requirements)
	job.Benefits = html.UnescapeString(job.Benefits)

	// Map experience to tags if not already set
	if len(job.ExpTags) == 0 {
		job.ExpTags = mapExperienceToTags(job.Experience)
	}

	return job, nil
}

// normalizeVietnamWorks handles VietnamWorks-specific data
func (n *Normalizer) normalizeVietnamWorks(job *domain.Job, data map[string]any) {
	// Basic fields - API uses camelCase
	job.Title = getString(data, "jobTitle", "title")
	job.Company = getString(data, "companyName", "company")
	job.Description = getString(data, "jobDescription", "description")
	job.Requirements = getString(data, "jobRequirement", "requirement")

	// Parse benefits from array
	job.Benefits = parseBenefitsVNW(data["benefits"])

	// Extract location from address or workingLocations
	job.Location = getString(data, "address")
	if job.Location == "" {
		job.Location = parseLocationsVNW(data["workingLocations"])
	}

	// Extract location city from workingLocations
	job.LocationCity = parseLocationCityVNW(data["workingLocations"])

	// Parse salary - VietnamWorks stores as integers directly
	job.SalaryMin = getInt(data, "salaryMin", "salary_min")
	job.SalaryMax = getInt(data, "salaryMax", "salary_max")

	// Convert salary to millions (VietnamWorks returns in VND)
	if job.SalaryMin > 1000 {
		job.SalaryMin = job.SalaryMin / 1000000
	}
	if job.SalaryMax > 1000 {
		job.SalaryMax = job.SalaryMax / 1000000
	}

	// Use prettySalary if available, otherwise format
	prettySalary := getString(data, "prettySalary")
	if prettySalary != "" {
		job.Salary = prettySalary
		// Check if salary is negotiable
		job.IsNegotiable = isNegotiableSalary(prettySalary)
	} else if job.SalaryMin > 0 && job.SalaryMax > 0 && job.SalaryMax < 999 {
		job.Salary = fmt.Sprintf("%d - %d triệu", job.SalaryMin, job.SalaryMax)
		job.IsNegotiable = false
	} else if job.SalaryMin > 0 {
		job.Salary = fmt.Sprintf("Trên %d triệu", job.SalaryMin)
		job.IsNegotiable = false
	} else {
		job.Salary = "Thỏa thuận"
		job.IsNegotiable = true
		job.SalaryMin = 0
		job.SalaryMax = 0
	}

	// Parse skills
	job.Field = parseSkillsVNW(data["skills"])

	// Parse experience
	yearsExp := getInt(data, "yearsOfExperience")
	if yearsExp > 0 {
		job.Experience = fmt.Sprintf("%d năm", yearsExp)
		job.ExpTags = mapExperienceYearsToTags(yearsExp)
	}

	// Parse position/level
	job.Position = getString(data, "jobLevelVI", "jobLevel")

	// Parse industry from industriesV3 (as array)
	job.Industry = parseIndustryVNWArray(data["industriesV3"])

	// Parse job function as fallback
	if len(job.Industry) == 0 {
		if jf := parseJobFunctionVNW(data["jobFunction"]); jf != "" {
			job.Industry = []string{jf}
		}
	}
}

// normalizeVieclam24h handles Vieclam24h-specific data
func (n *Normalizer) normalizeVieclam24h(job *domain.Job, data map[string]any) {
	// Basic fields
	job.Title = getString(data, "jobTitle", "title")
	job.Company = getString(data, "companyName", "company")
	job.Location = getString(data, "contactAddress", "address")

	// LocationCity from JSON-LD (already parsed as string array by scraper)
	job.LocationCity = getStringArray(data, "locationCity")

	// LocationDistrict from JSON-LD
	job.LocationDistrict = getStringArray(data, "locationDistrict")

	// Position from JSON-LD occupationalCategory
	job.Position = getString(data, "occupationalCategory")

	// WorkType from JSON-LD employmentType
	job.WorkType = getString(data, "employmentType")

	// Field - not available in JSON-LD, skip

	// Requirements: Combine jobRequirement and otherRequirement
	req := getString(data, "jobRequirement")
	other := getString(data, "otherRequirement")
	if req != "" && other != "" {
		job.Requirements = req + "<br/>" + other
	} else {
		job.Requirements = req + other
	}

	// Description
	job.Description = getString(data, "jobDescription")

	// Salary - prioritize JSON-LD baseSalary over API
	job.SalaryMin = getInt(data, "salaryMinJsonLd", "salaryFrom", "salaryMin")
	job.SalaryMax = getInt(data, "salaryMaxJsonLd", "salaryTo", "salaryMax")

	// Check if negotiable from JSON-LD
	if getBool(data, "isNegotiable") {
		job.IsNegotiable = true
		job.Salary = getString(data, "salaryTextJsonLd")
		if job.Salary == "" {
			job.Salary = "Thỏa thuận"
		}
	} else if job.SalaryMin > 0 && job.SalaryMax > 0 {
		job.Salary = fmt.Sprintf("%d - %d triệu", job.SalaryMin/1000000, job.SalaryMax/1000000)
		job.IsNegotiable = false
	} else if job.SalaryMin > 0 {
		job.Salary = fmt.Sprintf("Trên %d triệu", job.SalaryMin/1000000)
		job.IsNegotiable = false
	} else {
		// Fallback to API salaryText
		salaryText := getString(data, "salaryText")
		if salaryText != "" {
			job.Salary = salaryText
			job.IsNegotiable = isNegotiableSalary(salaryText)
		} else {
			job.Salary = "Thỏa thuận"
			job.IsNegotiable = true
		}
	}

	// Convert to millions for storage
	if job.SalaryMin > 1000 {
		job.SalaryMin = job.SalaryMin / 1000000
	}
	if job.SalaryMax > 1000 {
		job.SalaryMax = job.SalaryMax / 1000000
	}

	// Experience - prefer HTML extracted text over API experienceRange ID
	job.Experience = getString(data, "experienceText")

	// Map experience to tags with aggregation principle
	// A=0, B=0-1, C=1-2, D=2-3, E=3-5, F=5+
	// Higher experience includes all lower tags
	job.ExpTags = mapExperienceToTags(job.Experience)

	// ========== NEW ENRICHED FIELDS ==========
	// Stats from Crawler
	job.TotalViews = getInt(data, "totalViews")
	job.TotalResumeApplied = getInt(data, "totalResumeApplied")
	job.RateResponse = getFloat(data, "rateResponse")

	// Skills from JSON-LD (may be string with delimiters or array)
	job.Skills = parseSkillsString(data["skills"])
	job.Qualifications = getString(data, "qualifications")
	if job.Qualifications == "" {
		job.Qualifications = "Không yêu cầu"
	}
	job.CompanyWebsite = getString(data, "companyWebsite")
	job.OccupationalCategory = getString(data, "occupationalCategory")
	job.EmploymentType = getString(data, "employmentType")

	// Benefits from JSON-LD
	job.Benefits = getString(data, "jobBenefits")

	// Industry from JSON-LD (array parsed by scraper)
	job.Industry = getStringArray(data, "industry")

	// ExpiredAt from expiredAt field (Unix timestamp)
	if expiredAt, ok := data["expiredAt"]; ok {
		job.ExpiredAt = parseUnixTimestamp(expiredAt)
	}

	// CreatedAt from createdAt field (Unix timestamp) - when job was created on source
	if createdAt, ok := data["createdAt"]; ok {
		job.CreatedAt = parseUnixTimestamp(createdAt)
	}

	// UpdatedAt from updatedAt field (Unix timestamp) - when job was last updated on source
	if updatedAt, ok := data["updatedAt"]; ok {
		job.UpdatedAt = parseUnixTimestamp(updatedAt)
	}
}

// normalizeTopDev handles TopDev-specific data
func (n *Normalizer) normalizeTopDev(job *domain.Job, data map[string]any) {
	job.Title = getString(data, "title")
	job.Company = getString(data, "company")
	job.Description = getString(data, "description")
	job.Requirements = getString(data, "requirement")

	// Parse benefits from array
	job.Benefits = parseBenefitsArray(data["benefits"])

	// Parse locations
	locations := parseLocationsArray(data["locations"])
	if len(locations) > 0 {
		job.Location = strings.Join(locations, "; ")
		// Extract cities
		for _, loc := range locations {
			parts := strings.Split(loc, ",")
			if len(parts) > 0 {
				job.LocationCity = append(job.LocationCity, strings.TrimSpace(parts[len(parts)-1]))
			}
		}
	}

	// Parse salary from integers
	job.SalaryMin = getInt(data, "salary_min")
	job.SalaryMax = getInt(data, "salary_max")

	// TopDev returns salary in VND, convert to millions
	if job.SalaryMin > 1000 {
		job.SalaryMin = job.SalaryMin / 1000000
	}
	if job.SalaryMax > 1000 {
		job.SalaryMax = job.SalaryMax / 1000000
	}

	// Use salary text if available
	salaryText := getString(data, "salary_text")
	if salaryText != "" {
		job.Salary = salaryText
	} else if job.SalaryMin > 0 && job.SalaryMax > 0 {
		job.Salary = fmt.Sprintf("%d - %d triệu", job.SalaryMin, job.SalaryMax)
	} else {
		job.Salary = "Thỏa thuận"
	}

	// Parse skills
	skills := parseSkillsArray(data["skills"])
	if len(skills) > 0 {
		job.Field = strings.Join(skills, ", ")
	}

	// Parse experience
	job.Experience = parseExperience(data["experience"])
	job.ExpTags = mapExperienceToTags(job.Experience)

	// Parse level
	job.Position = parseLevel(data["level"])
}

// normalizeGeneric handles generic data format
func (n *Normalizer) normalizeGeneric(job *domain.Job, data map[string]any) {
	job.Title = getString(data, "title", "Tiêu đề tin")
	job.Company = getString(data, "company", "company_name", "Công ty")
	job.Location = getString(data, "location", "Địa điểm tuyển dụng", "address")

	// LocationCity as array
	if city := getString(data, "province", "Tỉnh thành tuyển dụng", "city"); city != "" {
		job.LocationCity = []string{city}
	}

	job.Position = getString(data, "position", "Chức vụ", "job_level")
	job.Salary = getString(data, "salary", "Mức lương")
	job.WorkType = getString(data, "work_type", "Hình thức làm việc", "job_type")

	// Industry as array
	if industry := getString(data, "industry", "Ngành nghề"); industry != "" {
		job.Industry = []string{industry}
	}

	job.Field = getString(data, "field", "Lĩnh vực")
	job.Experience = getString(data, "experience", "Kinh nghiệm")
	job.Description = getString(data, "description", "job_description")
	job.Requirements = getString(data, "requirements", "job_requirements")
	job.Benefits = getString(data, "benefits", "job_benefits")

	// Parse salary to numeric values
	job.SalaryMin, job.SalaryMax = parseSalary(job.Salary)
}

// getString tries multiple keys and returns the first non-empty value
func getString(data map[string]any, keys ...string) string {
	for _, key := range keys {
		if val, ok := data[key]; ok {
			switch v := val.(type) {
			case string:
				if v != "" {
					return strings.TrimSpace(v)
				}
			case float64:
				return fmt.Sprintf("%.0f", v)
			case int:
				return strconv.Itoa(v)
			}
		}
	}
	return ""
}

// getInt extracts integer from data
func getInt(data map[string]any, keys ...string) int {
	for _, key := range keys {
		if val, ok := data[key]; ok {
			switch v := val.(type) {
			case float64:
				return int(v)
			case int:
				return v
			case string:
				if i, err := strconv.Atoi(v); err == nil {
					return i
				}
			}
		}
	}
	return 0
}

// getFloat extracts float64 from data
func getFloat(data map[string]any, keys ...string) float64 {
	for _, key := range keys {
		if val, ok := data[key]; ok {
			switch v := val.(type) {
			case float64:
				return v
			case int:
				return float64(v)
			case string:
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					return f
				}
			}
		}
	}
	return 0
}

// getBool extracts bool from data
func getBool(data map[string]any, key string) bool {
	if val, ok := data[key]; ok {
		switch v := val.(type) {
		case bool:
			return v
		case int:
			return v != 0
		case string:
			return v == "true" || v == "1"
		}
	}
	return false
}

// parseUnixTimestamp parses Unix timestamp from various types
func parseUnixTimestamp(val any) time.Time {
	if val == nil {
		return time.Time{}
	}
	switch v := val.(type) {
	case float64:
		return time.Unix(int64(v), 0)
	case int64:
		return time.Unix(v, 0)
	case int:
		return time.Unix(int64(v), 0)
	case string:
		if ts, err := strconv.ParseInt(v, 10, 64); err == nil {
			return time.Unix(ts, 0)
		}
	case time.Time:
		return v
	}
	return time.Time{}
}

// mapExperienceToTags maps experience text to tags with aggregation
// A=0, B=0-1, C=1-2, D=2-3, E=3-5, F=5+
// Higher experience profiles can apply to lower requirement jobs
func mapExperienceToTags(exp string) []string {
	exp = strings.TrimSpace(exp)
	if exp == "" || strings.Contains(exp, "Không yêu cầu") {
		// No requirement - all levels can apply
		return []string{"A", "B", "C", "D", "E", "F"}
	}
	if strings.Contains(exp, "Chưa có kinh nghiệm") {
		// 0 years - include all higher tags
		return []string{"A", "B", "C", "D", "E", "F"}
	}
	if strings.Contains(exp, "Dưới 1 năm") {
		// 0-1 year
		return []string{"B", "C", "D", "E", "F"}
	}
	if strings.Contains(exp, "Hơn 5 năm") || strings.Contains(exp, "Trên 5 năm") {
		// 5+ years
		return []string{"F"}
	}
	// Parse number from text like "1 năm", "2 năm", "3 năm", "5 năm"
	var years int
	fmt.Sscanf(exp, "%d", &years)

	switch {
	case years <= 1:
		return []string{"C", "D", "E", "F"} // 1-2 range + higher
	case years <= 2:
		return []string{"D", "E", "F"} // 2-3 range + higher
	case years <= 3:
		return []string{"E", "F"} // 3-5 range + higher
	case years <= 5:
		return []string{"E", "F"} // 3-5 range + higher
	default:
		return []string{"F"} // 5+ range
	}
}

// getStringArray extracts []string from data
func getStringArray(data map[string]any, key string) []string {
	val, ok := data[key]
	if !ok || val == nil {
		return nil
	}
	switch v := val.(type) {
	case []string:
		return v
	case []any:
		var result []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case string:
		// Single string, return as single-element slice
		if v != "" {
			return []string{v}
		}
	}
	return nil
}

// parseSkillsString parses skills from string (delimiter separated) or array
func parseSkillsString(val any) []string {
	if val == nil {
		return nil
	}
	switch v := val.(type) {
	case []string:
		return v
	case []any:
		var result []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case string:
		// Parse skills string by common delimiters: - , ;
		if v == "" {
			return nil
		}
		// Try different delimiters
		var skills []string
		if strings.Contains(v, " - ") {
			skills = strings.Split(v, " - ")
		} else if strings.Contains(v, ",") {
			skills = strings.Split(v, ",")
		} else if strings.Contains(v, ";") {
			skills = strings.Split(v, ";")
		} else {
			return []string{v}
		}
		// Trim spaces
		var result []string
		for _, s := range skills {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	}
	return nil
}

// parseLocations extracts location string from VietnamWorks locations array
func parseLocations(val any) string {
	if val == nil {
		return ""
	}
	locations, ok := val.([]any)
	if !ok {
		return ""
	}

	var cities []string
	for _, loc := range locations {
		if m, ok := loc.(map[string]any); ok {
			if city, ok := m["cityNameVi"].(string); ok && city != "" {
				cities = append(cities, city)
			}
		}
	}
	return strings.Join(cities, ", ")
}

// parseProvince extracts first province from locations
func parseProvince(val any) string {
	if val == nil {
		return ""
	}
	locations, ok := val.([]any)
	if !ok || len(locations) == 0 {
		return ""
	}

	if m, ok := locations[0].(map[string]any); ok {
		if city, ok := m["cityNameVi"].(string); ok {
			return city
		}
	}
	return ""
}

// parseLocationsArray extracts locations from TopDev format
func parseLocationsArray(val any) []string {
	if val == nil {
		return nil
	}
	locations, ok := val.([]any)
	if !ok {
		// Try string array
		if strArr, ok := val.([]string); ok {
			return strArr
		}
		return nil
	}

	var result []string
	for _, loc := range locations {
		if s, ok := loc.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// parseBenefits extracts benefits from various formats
func parseBenefits(val any) string {
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case []any:
		var benefits []string
		for _, b := range v {
			if s, ok := b.(string); ok {
				benefits = append(benefits, s)
			} else if m, ok := b.(map[string]any); ok {
				if desc, ok := m["description"].(string); ok {
					benefits = append(benefits, desc)
				}
			}
		}
		return strings.Join(benefits, "; ")
	}
	return ""
}

// parseBenefitsArray extracts benefits from TopDev format
func parseBenefitsArray(val any) string {
	if val == nil {
		return ""
	}
	arr, ok := val.([]any)
	if !ok {
		if strArr, ok := val.([]string); ok {
			return strings.Join(strArr, "; ")
		}
		return ""
	}

	var benefits []string
	for _, b := range arr {
		if s, ok := b.(string); ok {
			benefits = append(benefits, s)
		}
	}
	return strings.Join(benefits, "; ")
}

// parseSkills extracts skills from VietnamWorks format
func parseSkills(val any) string {
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case []any:
		var skills []string
		for _, s := range v {
			if str, ok := s.(string); ok {
				skills = append(skills, str)
			} else if m, ok := s.(map[string]any); ok {
				if name, ok := m["name"].(string); ok {
					skills = append(skills, name)
				}
			}
		}
		return strings.Join(skills, ", ")
	}
	return ""
}

// parseLocationCityVNW extracts cities from VietnamWorks workingLocations as array
func parseLocationCityVNW(val any) []string {
	if val == nil {
		return nil
	}
	locations, ok := val.([]any)
	if !ok {
		return nil
	}

	var cities []string
	for _, loc := range locations {
		if m, ok := loc.(map[string]any); ok {
			if city, ok := m["cityNameVi"].(string); ok && city != "" {
				cities = append(cities, city)
			}
		}
	}
	return cities
}

// parseIndustryVNWArray extracts industries from VietnamWorks industriesV3 as array
func parseIndustryVNWArray(val any) []string {
	if val == nil {
		return nil
	}
	industries, ok := val.([]any)
	if !ok {
		return nil
	}

	var result []string
	for _, ind := range industries {
		if m, ok := ind.(map[string]any); ok {
			// Try industryNameVi first, then name
			if name, ok := m["industryNameVi"].(string); ok && name != "" {
				result = append(result, name)
			} else if name, ok := m["name"].(string); ok && name != "" {
				result = append(result, name)
			}
		}
	}
	return result
}

// parseSkillsArray extracts skills from TopDev format
func parseSkillsArray(val any) []string {
	if val == nil {
		return nil
	}
	arr, ok := val.([]any)
	if !ok {
		if strArr, ok := val.([]string); ok {
			return strArr
		}
		return nil
	}

	var skills []string
	for _, s := range arr {
		if str, ok := s.(string); ok {
			skills = append(skills, str)
		}
	}
	return skills
}

// parseExperience extracts experience from various formats
func parseExperience(val any) string {
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f năm", v)
	case int:
		return fmt.Sprintf("%d năm", v)
	}
	return ""
}

// parseLevel extracts job level from various formats
func parseLevel(val any) string {
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case map[string]any:
		if name, ok := v["name"].(string); ok {
			return name
		}
	}
	return ""
}

// parseSalary extracts min/max salary values from Vietnamese salary strings
func parseSalary(salary string) (min, max int) {
	if salary == "" || strings.Contains(strings.ToLower(salary), "thỏa thuận") {
		return 0, 0
	}

	// Pattern: "10 - 15 triệu"
	rangeRe := regexp.MustCompile(`(\d+)\s*-\s*(\d+)`)
	if matches := rangeRe.FindStringSubmatch(salary); len(matches) == 3 {
		min, _ = strconv.Atoi(matches[1])
		max, _ = strconv.Atoi(matches[2])
		return min, max
	}

	// Pattern: "Trên 30 triệu"
	aboveRe := regexp.MustCompile(`[Tt]rên\s*(\d+)`)
	if matches := aboveRe.FindStringSubmatch(salary); len(matches) == 2 {
		min, _ = strconv.Atoi(matches[1])
		return min, 0
	}

	// Pattern: "Dưới 5 triệu"
	belowRe := regexp.MustCompile(`[Dd]ưới\s*(\d+)`)
	if matches := belowRe.FindStringSubmatch(salary); len(matches) == 2 {
		max, _ = strconv.Atoi(matches[1])
		return 0, max
	}

	return 0, 0
}

// NormalizeTime parses various time formats
func NormalizeTime(s string) time.Time {
	formats := []string{
		"2006-01-02",
		"02/01/2006",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t
		}
	}

	return time.Now()
}

// ======== VietnamWorks-specific helpers ========

// parseBenefitsVNW extracts benefits from VietnamWorks format
func parseBenefitsVNW(val any) string {
	if val == nil {
		return ""
	}
	arr, ok := val.([]any)
	if !ok {
		return ""
	}

	var benefits []string
	for _, b := range arr {
		if m, ok := b.(map[string]any); ok {
			if v, ok := m["benefitValue"].(string); ok && v != "" {
				benefits = append(benefits, v)
			}
		}
	}
	return strings.Join(benefits, "; ")
}

// parseLocationsVNW extracts location from VietnamWorks workingLocations
func parseLocationsVNW(val any) string {
	if val == nil {
		return ""
	}
	arr, ok := val.([]any)
	if !ok {
		return ""
	}

	var locs []string
	for _, loc := range arr {
		if m, ok := loc.(map[string]any); ok {
			if addr, ok := m["address"].(string); ok && addr != "" {
				locs = append(locs, addr)
			}
		}
	}
	return strings.Join(locs, "; ")
}

// parseProvinceVNW extracts province from VietnamWorks workingLocations
func parseProvinceVNW(val any) string {
	if val == nil {
		return ""
	}
	arr, ok := val.([]any)
	if !ok || len(arr) == 0 {
		return ""
	}

	if m, ok := arr[0].(map[string]any); ok {
		if city, ok := m["cityNameVI"].(string); ok && city != "" {
			return city
		}
		if city, ok := m["cityName"].(string); ok && city != "" {
			return city
		}
	}
	return ""
}

// parseSkillsVNW extracts skills from VietnamWorks format
func parseSkillsVNW(val any) string {
	if val == nil {
		return ""
	}
	arr, ok := val.([]any)
	if !ok {
		return ""
	}

	var skills []string
	for _, s := range arr {
		if m, ok := s.(map[string]any); ok {
			if name, ok := m["skillName"].(string); ok && name != "" {
				skills = append(skills, name)
			}
		}
	}
	return strings.Join(skills, ", ")
}

// mapExperienceYearsToTags converts years to experience tags
func mapExperienceYearsToTags(years int) []string {
	switch {
	case years <= 1:
		return []string{"A", "B"}
	case years <= 2:
		return []string{"C"}
	case years <= 5:
		return []string{"D"}
	case years <= 10:
		return []string{"E"}
	default:
		return []string{"F"}
	}
}

// parseIndustryVNW extracts industry from VietnamWorks industriesV3
func parseIndustryVNW(val any) string {
	if val == nil {
		return ""
	}
	arr, ok := val.([]any)
	if !ok || len(arr) == 0 {
		return ""
	}

	var industries []string
	for _, ind := range arr {
		if m, ok := ind.(map[string]any); ok {
			if name, ok := m["industryV3NameVI"].(string); ok && name != "" {
				industries = append(industries, name)
			} else if name, ok := m["industryV3Name"].(string); ok && name != "" {
				industries = append(industries, name)
			}
		}
	}
	return strings.Join(industries, ", ")
}

// parseJobFunctionVNW extracts job function from VietnamWorks format
func parseJobFunctionVNW(val any) string {
	if val == nil {
		return ""
	}
	m, ok := val.(map[string]any)
	if !ok {
		return ""
	}

	if parentVI, ok := m["parentNameVI"].(string); ok && parentVI != "" {
		return parentVI
	}
	if parent, ok := m["parentName"].(string); ok && parent != "" {
		return parent
	}
	return ""
}

// isNegotiableSalary checks if salary text indicates negotiable salary
func isNegotiableSalary(salary string) bool {
	salaryLower := strings.ToLower(salary)
	negotiableTerms := []string{
		"thương lượng",
		"thỏa thuận",
		"thoả thuận",
		"cạnh tranh",
		"hấp dẫn",
		"negotiable",
		"competitive",
	}
	for _, term := range negotiableTerms {
		if strings.Contains(salaryLower, term) {
			return true
		}
	}
	return false
}
