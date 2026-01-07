package indexer

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"

	_ "github.com/lib/pq"
	"github.com/project-tktt/go-crawler/internal/domain"
)

// PostgresIndexer indexes jobs to PostgreSQL
type PostgresIndexer struct {
	db        *sql.DB
	tableName string
}

// NewPostgresIndexer creates a new PostgreSQL indexer
func NewPostgresIndexer(connStr string, tableName string) (*PostgresIndexer, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	indexer := &PostgresIndexer{
		db:        db,
		tableName: tableName,
	}

	// Ensure table exists
	if err := indexer.ensureTable(); err != nil {
		return nil, fmt.Errorf("ensure table: %w", err)
	}

	return indexer, nil
}

// ensureTable creates the jobs table if it doesn't exist
func (i *PostgresIndexer) ensureTable() error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			company TEXT,
			location TEXT,
			position TEXT,
			salary TEXT,
			salary_min INTEGER,
			salary_max INTEGER,
			work_type TEXT,
			industry TEXT[],
			field TEXT,
			experience TEXT,
			experience_tags TEXT[],
			description TEXT,
			requirements TEXT,
			benefits TEXT,
			source TEXT,
			source_url TEXT,
			crawled_at TIMESTAMP WITH TIME ZONE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			-- New enriched fields
			total_views INTEGER DEFAULT 0,
			total_resume_applied INTEGER DEFAULT 0,
			rate_response FLOAT DEFAULT 0,
			skills TEXT[],
			qualifications TEXT,
			company_website TEXT,
			occupational_category TEXT,
			employment_type TEXT,
			location_city TEXT[],
			location_district TEXT[],
			expired_at TIMESTAMP WITH TIME ZONE,
			is_negotiable BOOLEAN DEFAULT FALSE
		)
	`, i.tableName)

	_, err := i.db.Exec(query)
	return err
}

// Index indexes a single job
func (i *PostgresIndexer) Index(ctx context.Context, job *domain.Job) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (
			id, title, company, location, position,
			salary, salary_min, salary_max, work_type, industry, field,
			experience, experience_tags, description, requirements, benefits,
			source, source_url, crawled_at, updated_at,
			total_views, total_resume_applied, rate_response, skills, qualifications,
			company_website, occupational_category, employment_type, location_city, location_district, expired_at, is_negotiable
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10, $11,
			$12, $13, $14, $15, $16,
			$17, $18, $19, NOW(),
			$20, $21, $22, $23, $24,
			$25, $26, $27, $28, $29, $30, $31
		)
		ON CONFLICT (id) DO UPDATE SET
			title = EXCLUDED.title,
			company = EXCLUDED.company,
			location = EXCLUDED.location,
			position = EXCLUDED.position,
			salary = EXCLUDED.salary,
			salary_min = EXCLUDED.salary_min,
			salary_max = EXCLUDED.salary_max,
			work_type = EXCLUDED.work_type,
			industry = EXCLUDED.industry,
			field = EXCLUDED.field,
			experience = EXCLUDED.experience,
			experience_tags = EXCLUDED.experience_tags,
			description = EXCLUDED.description,
			requirements = EXCLUDED.requirements,
			benefits = EXCLUDED.benefits,
			source = EXCLUDED.source,
			source_url = EXCLUDED.source_url,
			crawled_at = EXCLUDED.crawled_at,
			updated_at = NOW(),
			total_views = EXCLUDED.total_views,
			total_resume_applied = EXCLUDED.total_resume_applied,
			rate_response = EXCLUDED.rate_response,
			skills = EXCLUDED.skills,
			qualifications = EXCLUDED.qualifications,
			company_website = EXCLUDED.company_website,
			occupational_category = EXCLUDED.occupational_category,
			employment_type = EXCLUDED.employment_type,
			location_city = EXCLUDED.location_city,
			location_district = EXCLUDED.location_district,
			expired_at = EXCLUDED.expired_at,
			is_negotiable = EXCLUDED.is_negotiable
	`, i.tableName)

	expTags := "{}"
	if len(job.ExpTags) > 0 {
		expTags = "{" + strings.Join(job.ExpTags, ",") + "}"
	}

	skillsArr := "{}"
	if len(job.Skills) > 0 {
		skillsArr = "{" + strings.Join(job.Skills, ",") + "}"
	}

	industryArr := "{}"
	if len(job.Industry) > 0 {
		industryArr = "{" + strings.Join(job.Industry, ",") + "}"
	}

	locationCityArr := "{}"
	if len(job.LocationCity) > 0 {
		locationCityArr = "{" + strings.Join(job.LocationCity, ",") + "}"
	}

	locationDistrictArr := "{}"
	if len(job.LocationDistrict) > 0 {
		locationDistrictArr = "{" + strings.Join(job.LocationDistrict, ",") + "}"
	}

	_, err := i.db.ExecContext(ctx, query,
		job.ID, job.Title, job.Company, job.Location, job.Position,
		job.Salary, job.SalaryMin, job.SalaryMax, job.WorkType, industryArr, job.Field,
		job.Experience, expTags, job.Description, job.Requirements, job.Benefits,
		job.Source, job.SourceURL, job.CrawledAt,
		job.TotalViews, job.TotalResumeApplied, job.RateResponse, skillsArr, job.Qualifications,
		job.CompanyWebsite, job.OccupationalCategory, job.EmploymentType, locationCityArr, locationDistrictArr, job.ExpiredAt, job.IsNegotiable,
	)

	return err
}

// BulkIndex indexes multiple jobs at once using a transaction
func (i *PostgresIndexer) BulkIndex(ctx context.Context, jobs []*domain.Job) error {
	if len(jobs) == 0 {
		return nil
	}

	tx, err := i.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := fmt.Sprintf(`
		INSERT INTO %s (
			id, title, company, location, position,
			salary, salary_min, salary_max, work_type, industry, field,
			experience, experience_tags, description, requirements, benefits,
			source, source_url, crawled_at, updated_at,
			total_views, total_resume_applied, rate_response, skills, qualifications,
			company_website, occupational_category, employment_type, location_city, location_district, expired_at, is_negotiable
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10, $11,
			$12, $13, $14, $15, $16,
			$17, $18, $19, NOW(),
			$20, $21, $22, $23, $24,
			$25, $26, $27, $28, $29, $30, $31
		)
		ON CONFLICT (id) DO UPDATE SET
			title = EXCLUDED.title,
			company = EXCLUDED.company,
			location = EXCLUDED.location,
			position = EXCLUDED.position,
			salary = EXCLUDED.salary,
			salary_min = EXCLUDED.salary_min,
			salary_max = EXCLUDED.salary_max,
			work_type = EXCLUDED.work_type,
			industry = EXCLUDED.industry,
			field = EXCLUDED.field,
			experience = EXCLUDED.experience,
			experience_tags = EXCLUDED.experience_tags,
			description = EXCLUDED.description,
			requirements = EXCLUDED.requirements,
			benefits = EXCLUDED.benefits,
			source = EXCLUDED.source,
			source_url = EXCLUDED.source_url,
			crawled_at = EXCLUDED.crawled_at,
			updated_at = NOW(),
			total_views = EXCLUDED.total_views,
			total_resume_applied = EXCLUDED.total_resume_applied,
			rate_response = EXCLUDED.rate_response,
			skills = EXCLUDED.skills,
			qualifications = EXCLUDED.qualifications,
			company_website = EXCLUDED.company_website,
			occupational_category = EXCLUDED.occupational_category,
			employment_type = EXCLUDED.employment_type,
			location_city = EXCLUDED.location_city,
			location_district = EXCLUDED.location_district,
			expired_at = EXCLUDED.expired_at,
			is_negotiable = EXCLUDED.is_negotiable
	`, i.tableName)

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, job := range jobs {
		expTags := "{}"
		if len(job.ExpTags) > 0 {
			expTags = "{" + strings.Join(job.ExpTags, ",") + "}"
		}

		skillsArr := "{}"
		if len(job.Skills) > 0 {
			skillsArr = "{" + strings.Join(job.Skills, ",") + "}"
		}

		industryArr := "{}"
		if len(job.Industry) > 0 {
			industryArr = "{" + strings.Join(job.Industry, ",") + "}"
		}

		locationCityArr := "{}"
		if len(job.LocationCity) > 0 {
			locationCityArr = "{" + strings.Join(job.LocationCity, ",") + "}"
		}

		locationDistrictArr := "{}"
		if len(job.LocationDistrict) > 0 {
			locationDistrictArr = "{" + strings.Join(job.LocationDistrict, ",") + "}"
		}

		_, err := stmt.ExecContext(ctx,
			job.ID, job.Title, job.Company, job.Location, job.Position,
			job.Salary, job.SalaryMin, job.SalaryMax, job.WorkType, industryArr, job.Field,
			job.Experience, expTags, job.Description, job.Requirements, job.Benefits,
			job.Source, job.SourceURL, job.CrawledAt,
			job.TotalViews, job.TotalResumeApplied, job.RateResponse, skillsArr, job.Qualifications,
			job.CompanyWebsite, job.OccupationalCategory, job.EmploymentType, locationCityArr, locationDistrictArr, job.ExpiredAt, job.IsNegotiable,
		)
		if err != nil {
			log.Printf("Error indexing job %s: %v", job.ID, err)
			continue
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// Close closes the database connection
func (i *PostgresIndexer) Close() error {
	return i.db.Close()
}
