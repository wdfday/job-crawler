package vietnamworks

import "time"

// Config holds VietnamWorks-specific configuration
type Config struct {
	MaxPages     int
	RequestDelay time.Duration
	UserAgent    string
}

// SearchRequest is the payload for VietnamWorks search API
type SearchRequest struct {
	UserID      int    `json:"userId"`
	Query       string `json:"query"`
	HitsPerPage int    `json:"hitsPerPage"`
	Page        int    `json:"page"`
}

// SearchResponse is the VietnamWorks API response
type SearchResponse struct {
	Data []JobData `json:"data"`
	Meta struct {
		NbHits      int `json:"nbHits"`
		NbPages     int `json:"nbPages"`
		Page        int `json:"page"`
		HitsPerPage int `json:"hitsPerPage"`
	} `json:"meta"`
}

// JobData represents a single job from VietnamWorks API
type JobData struct {
	JobID              int               `json:"jobId"`
	JobTitle           string            `json:"jobTitle"`
	JobURL             string            `json:"jobUrl"`
	CompanyName        string            `json:"companyName"`
	CompanyID          int               `json:"companyId"`
	CompanyLogo        string            `json:"companyLogo"`
	CompanyProfile     string            `json:"companyProfile"`
	CompanySize        string            `json:"companySize"`
	Address            string            `json:"address"`
	SalaryMin          int               `json:"salaryMin"`
	SalaryMax          int               `json:"salaryMax"`
	PrettySalary       string            `json:"prettySalary"`
	SalaryCurrency     string            `json:"salaryCurrency"`
	JobDescription     string            `json:"jobDescription"`
	JobRequirement     string            `json:"jobRequirement"`
	Benefits           []Benefit         `json:"benefits"`
	Skills             []Skill           `json:"skills"`
	WorkingLocations   []WorkingLocation `json:"workingLocations"`
	IndustriesV3       []IndustryV3      `json:"industriesV3"`
	JobFunction        JobFunction       `json:"jobFunction"`
	JobLevel           string            `json:"jobLevel"`
	JobLevelVI         string            `json:"jobLevelVI"`
	YearsOfExperience  int               `json:"yearsOfExperience"`
	TypeWorkingID      int               `json:"typeWorkingId"`
	LanguageSelected   string            `json:"languageSelected"`
	LanguageSelectedVI string            `json:"languageSelectedVI"`
	ApprovedOn         string            `json:"approvedOn"`
	ExpiredOn          string            `json:"expiredOn"`
	CreatedOn          string            `json:"createdOn"`
	LastUpdatedOn      string            `json:"lastUpdatedOn"`
	OnlineOn           string            `json:"onlineOn"`
	ContactName        string            `json:"contactName"`
	HighestDegreeID    int               `json:"highestDegreeId"`
	RangeAge           string            `json:"rangeAge"`
}

// Benefit represents a job benefit
type Benefit struct {
	BenefitID       int    `json:"benefitId"`
	BenefitIconName string `json:"benefitIconName"`
	BenefitName     string `json:"benefitName"`
	BenefitNameVI   string `json:"benefitNameVI"`
	BenefitValue    string `json:"benefitValue"`
}

// Skill represents a job skill
type Skill struct {
	SkillID     int    `json:"skillId"`
	SkillWeight int    `json:"skillWeight"`
	SkillName   string `json:"skillName"`
}

// WorkingLocation represents a job location
type WorkingLocation struct {
	WorkingLocationID int    `json:"workingLocationId"`
	AddressID         int    `json:"addressId"`
	CityID            int    `json:"cityId"`
	DistrictID        int    `json:"districtId"`
	Address           string `json:"address"`
	CityName          string `json:"cityName"`
	CityNameVI        string `json:"cityNameVI"`
	GeoLoc            GeoLoc `json:"geoLoc"`
}

// GeoLoc represents geographic coordinates
type GeoLoc struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// IndustryV3 represents an industry category
type IndustryV3 struct {
	IndustryV3ID     int    `json:"industryV3Id"`
	IndustryV3Name   string `json:"industryV3Name"`
	IndustryV3NameVI string `json:"industryV3NameVI"`
}

// JobFunction represents job function category
type JobFunction struct {
	ParentID     int                `json:"parentId"`
	ParentName   string             `json:"parentName"`
	ParentNameVI string             `json:"parentNameVI"`
	Children     []JobFunctionChild `json:"children"`
}

// JobFunctionChild represents a child job function
type JobFunctionChild struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	NameVI string `json:"nameVI"`
}
