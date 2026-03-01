package db

import (
	"context"
	"fmt"
	"sort"
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

type ServiceHealth struct {
	Service string  `json:"service"`
	Metric  string  `json:"metric"`
	P75     float64 `json:"p75"`
}

type DirectiveCount struct {
	Directive string `json:"directive"`
	Count     int64  `json:"count"`
}

func GetAllServicesHealth(ctx context.Context, d *gorm.DB) (map[string][]ServiceHealth, error) {
	cutoff := time.Now().AddDate(0, 0, -28)
	var results []ServiceHealth
	// PERCENTILE_CONT requires raw SQL -- no GORM builder equivalent.
	err := d.WithContext(ctx).Raw(`
		SELECT service, name AS metric,
			PERCENTILE_CONT(0.75) WITHIN GROUP (ORDER BY value) AS p75
		FROM web_vitals
		WHERE created_at >= ? AND deleted_at IS NULL
		GROUP BY service, name
		ORDER BY service, name
	`, cutoff).Scan(&results).Error
	if err != nil {
		return nil, fmt.Errorf("querying all services health: %w", err)
	}

	out := make(map[string][]ServiceHealth)
	for _, r := range results {
		out[r.Service] = append(out[r.Service], r)
	}
	return out, nil
}

func GetServices(ctx context.Context, d *gorm.DB) ([]string, error) {
	seen := make(map[string]bool)

	var wvServices []string
	if err := d.WithContext(ctx).Model(&WebVital{}).Distinct("service").Pluck("service", &wvServices).Error; err != nil {
		return nil, fmt.Errorf("querying web_vitals services: %w", err)
	}
	for _, s := range wvServices {
		seen[s] = true
	}

	var rtServices []string
	if err := d.WithContext(ctx).Model(&ReportToEntry{}).Distinct("service").Pluck("service", &rtServices).Error; err != nil {
		return nil, fmt.Errorf("querying report_to services: %w", err)
	}
	for _, s := range rtServices {
		seen[s] = true
	}

	var srServices []string
	if err := d.WithContext(ctx).Model(&SecurityReportEntry{}).Distinct("service").Pluck("service", &srServices).Error; err != nil {
		return nil, fmt.Errorf("querying security_report services: %w", err)
	}
	for _, s := range srServices {
		seen[s] = true
	}

	services := make([]string, 0, len(seen))
	for s := range seen {
		services = append(services, s)
	}
	sort.Strings(services)
	return services, nil
}

func GetWebVitalSummaries(ctx context.Context, d *gorm.DB, service string) ([]WebVitalDailySummary, error) {
	cutoff := time.Now().AddDate(0, -3, 0)
	var results []WebVitalDailySummary
	err := d.WithContext(ctx).
		Model(&WebVital{}).
		Select("DATE(created_at) AS day, service, name, AVG(value) AS value").
		Where("service = ? AND created_at >= ?", service, cutoff).
		Group("day, service, name").
		Order("day DESC").
		Find(&results).Error
	if err != nil {
		return nil, fmt.Errorf("querying web vital summaries: %w", err)
	}
	return results, nil
}

func GetWebVitalP75s(ctx context.Context, d *gorm.DB, service string) ([]WebVitalP75, error) {
	cutoff := time.Now().AddDate(0, 0, -28)
	var results []WebVitalP75
	// PERCENTILE_CONT requires raw SQL.
	err := d.WithContext(ctx).Raw(`
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

func GetReportCounts(ctx context.Context, d *gorm.DB, service string) ([]ReportDailyCount, error) {
	cutoff := time.Now().AddDate(0, -3, 0)

	var rtCounts []ReportDailyCount
	err := d.WithContext(ctx).
		Model(&ReportToEntry{}).
		Select("DATE(created_at) AS day, report_type, COUNT(*) AS count").
		Where("service = ? AND created_at >= ?", service, cutoff).
		Group("day, report_type").
		Order("day DESC").
		Find(&rtCounts).Error
	if err != nil {
		return nil, fmt.Errorf("querying report-to counts: %w", err)
	}

	var srCounts []ReportDailyCount
	err = d.WithContext(ctx).
		Model(&SecurityReportEntry{}).
		Select("DATE(created_at) AS day, report_type, COUNT(*) AS count").
		Where("service = ? AND created_at >= ?", service, cutoff).
		Group("day, report_type").
		Order("day DESC").
		Find(&srCounts).Error
	if err != nil {
		return nil, fmt.Errorf("querying security report counts: %w", err)
	}

	return append(rtCounts, srCounts...), nil
}

func GetRecentReports(ctx context.Context, d *gorm.DB, service string, limit int) ([]SecurityReportEntry, error) {
	var results []SecurityReportEntry
	err := d.WithContext(ctx).
		Where("service = ?", service).
		Order("created_at DESC").
		Limit(limit).
		Find(&results).Error
	if err != nil {
		return nil, fmt.Errorf("querying recent reports: %w", err)
	}
	return results, nil
}

func GetRecentReportToEntries(ctx context.Context, d *gorm.DB, service string, limit int) ([]ReportToEntry, error) {
	var results []ReportToEntry
	err := d.WithContext(ctx).
		Where("service = ?", service).
		Order("created_at DESC").
		Limit(limit).
		Find(&results).Error
	if err != nil {
		return nil, fmt.Errorf("querying recent report-to entries: %w", err)
	}
	return results, nil
}

func GetTopViolatedDirectives(ctx context.Context, d *gorm.DB, service string, limit int) ([]DirectiveCount, error) {
	cutoff := time.Now().AddDate(0, -1, 0)
	var results []DirectiveCount
	err := d.WithContext(ctx).
		Model(&SecurityReportEntry{}).
		Select("effective_directive AS directive, COUNT(*) AS count").
		Where("service = ? AND created_at >= ? AND report_type = ? AND effective_directive != ''",
			service, cutoff, "csp-violation").
		Group("effective_directive").
		Order("count DESC").
		Limit(limit).
		Find(&results).Error
	if err != nil {
		return nil, fmt.Errorf("querying top violated directives: %w", err)
	}
	return results, nil
}
