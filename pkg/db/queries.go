package db

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

type WebVitalDailySummary struct {
	Day     string  `json:"day"`
	Service string  `json:"service"`
	Name    string  `json:"name"`
	Value   float64 `json:"value"`
}

type WebVitalP75 struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

type ReportDailyCount struct {
	Day        string `json:"day"`
	ReportType string `json:"report_type"`
	Count      int64  `json:"count"`
}

func GetServices(d *gorm.DB) ([]string, error) {
	var services []string
	err := d.Raw(`
		SELECT DISTINCT service FROM web_vitals
		UNION
		SELECT DISTINCT service FROM report_to_entries
		UNION
		SELECT DISTINCT service FROM security_report_entries
		ORDER BY service
	`).Scan(&services).Error
	if err != nil {
		return nil, fmt.Errorf("querying services: %w", err)
	}
	return services, nil
}

func GetWebVitalSummaries(d *gorm.DB, service string) ([]WebVitalDailySummary, error) {
	cutoff := time.Now().AddDate(0, -3, 0)
	var results []WebVitalDailySummary
	err := d.Raw(`
		SELECT DATE(created_at) AS day, service, name, AVG(value) AS value
		FROM web_vitals
		WHERE service = ? AND created_at >= ? AND deleted_at IS NULL
		GROUP BY DATE(created_at), service, name
		ORDER BY day DESC
	`, service, cutoff).Scan(&results).Error
	if err != nil {
		return nil, fmt.Errorf("querying web vital summaries: %w", err)
	}
	return results, nil
}

func GetWebVitalP75s(d *gorm.DB, service string) ([]WebVitalP75, error) {
	cutoff := time.Now().AddDate(0, 0, -28)
	var results []WebVitalP75
	err := d.Raw(`
		SELECT name, PERCENTILE_CONT(0.75) WITHIN GROUP (ORDER BY value) AS value
		FROM web_vitals
		WHERE service = ? AND created_at >= ? AND deleted_at IS NULL
		GROUP BY name
	`, service, cutoff).Scan(&results).Error
	if err != nil {
		return nil, fmt.Errorf("querying web vital p75s: %w", err)
	}
	return results, nil
}

func GetReportCounts(d *gorm.DB, service string) ([]ReportDailyCount, error) {
	cutoff := time.Now().AddDate(0, -3, 0)
	var results []ReportDailyCount

	var rtCounts []ReportDailyCount
	err := d.Raw(`
		SELECT DATE(created_at) AS day, report_type, COUNT(*) AS count
		FROM report_to_entries
		WHERE service = ? AND created_at >= ? AND deleted_at IS NULL
		GROUP BY DATE(created_at), report_type
		ORDER BY day DESC
	`, service, cutoff).Scan(&rtCounts).Error
	if err != nil {
		return nil, fmt.Errorf("querying report-to counts: %w", err)
	}
	results = append(results, rtCounts...)

	var srCounts []ReportDailyCount
	err = d.Raw(`
		SELECT DATE(created_at) AS day, report_type, COUNT(*) AS count
		FROM security_report_entries
		WHERE service = ? AND created_at >= ? AND deleted_at IS NULL
		GROUP BY DATE(created_at), report_type
		ORDER BY day DESC
	`, service, cutoff).Scan(&srCounts).Error
	if err != nil {
		return nil, fmt.Errorf("querying security report counts: %w", err)
	}
	results = append(results, srCounts...)

	return results, nil
}

func GetRecentReports(d *gorm.DB, service string, limit int) ([]SecurityReportEntry, error) {
	var results []SecurityReportEntry
	err := d.Where("service = ?", service).
		Order("created_at DESC").
		Limit(limit).
		Find(&results).Error
	if err != nil {
		return nil, fmt.Errorf("querying recent reports: %w", err)
	}
	return results, nil
}

func GetRecentReportToEntries(d *gorm.DB, service string, limit int) ([]ReportToEntry, error) {
	var results []ReportToEntry
	err := d.Where("service = ?", service).
		Order("created_at DESC").
		Limit(limit).
		Find(&results).Error
	if err != nil {
		return nil, fmt.Errorf("querying recent report-to entries: %w", err)
	}
	return results, nil
}

func GetTopViolatedDirectives(d *gorm.DB, service string, limit int) ([]struct {
	Directive string `json:"directive"`
	Count     int64  `json:"count"`
}, error) {
	cutoff := time.Now().AddDate(0, -1, 0)
	var results []struct {
		Directive string `json:"directive"`
		Count     int64  `json:"count"`
	}

	err := d.Raw(`
		SELECT effective_directive AS directive, COUNT(*) AS count
		FROM security_report_entries
		WHERE service = ? AND created_at >= ? AND report_type = 'csp-violation'
			AND effective_directive != '' AND deleted_at IS NULL
		GROUP BY effective_directive
		ORDER BY count DESC
		LIMIT ?
	`, service, cutoff, limit).Scan(&results).Error
	if err != nil {
		return nil, fmt.Errorf("querying top violated directives: %w", err)
	}
	return results, nil
}
