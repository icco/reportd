package db

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"
)

// Day is a date-only value that scans cleanly from Postgres (date →
// time.Time) and SQLite (DATE() → string) and marshals as "YYYY-MM-DD".
type Day time.Time

// MarshalJSON encodes d as "YYYY-MM-DD".
func (d Day) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(d).Format(time.DateOnly))
}

// Scan accepts nil, time.Time, []byte, or string (date-only or ISO
// timestamp; only the date prefix is used).
func (d *Day) Scan(v any) error {
	switch x := v.(type) {
	case nil:
		*d = Day{}
		return nil
	case time.Time:
		*d = Day(x)
		return nil
	case []byte:
		return d.Scan(string(x))
	case string:
		// Some drivers return a longer ISO timestamp; we only want the date.
		if len(x) > len(time.DateOnly) {
			x = x[:len(time.DateOnly)]
		}
		t, err := time.Parse(time.DateOnly, x)
		if err != nil {
			return fmt.Errorf("parsing day %q: %w", x, err)
		}
		*d = Day(t)
		return nil
	default:
		return fmt.Errorf("unsupported type for Day: %T", v)
	}
}

// WebVitalDailySummary is one (service, metric, day) average.
type WebVitalDailySummary struct {
	Day     Day     `json:"day"`
	Service string  `json:"service"`
	Name    string  `json:"name"`
	Value   float64 `json:"value"`
}

// WebVitalAverage is one metric's trailing-28-day average.
type WebVitalAverage struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// ReportDailyCount is the count of one report type on one day.
type ReportDailyCount struct {
	Day        Day    `json:"day"`
	ReportType string `json:"report_type"`
	Count      int64  `json:"count"`
}

// ServiceHealth is one (metric, average) pair for a service.
type ServiceHealth struct {
	Service string  `json:"service"`
	Metric  string  `json:"metric"`
	Average float64 `json:"average"`
}

// DirectiveCount is the violation count for a single CSP directive.
type DirectiveCount struct {
	Directive string `json:"directive"`
	Count     int64  `json:"count"`
}

// GetAllServicesHealth returns trailing-28-day metric averages keyed by
// service.
func GetAllServicesHealth(ctx context.Context, d *gorm.DB) (map[string][]ServiceHealth, error) {
	cutoff := time.Now().AddDate(0, 0, -28)
	var results []ServiceHealth
	err := d.WithContext(ctx).
		Model(&WebVital{}).
		Select("service, name AS metric, AVG(value) AS average").
		Where("created_at >= ?", cutoff).
		Group("service, name").
		Order("service, name").
		Find(&results).Error
	if err != nil {
		return nil, fmt.Errorf("querying all services health: %w", err)
	}

	out := make(map[string][]ServiceHealth)
	for _, r := range results {
		out[r.Service] = append(out[r.Service], r)
	}
	return out, nil
}

// GetServices returns the sorted union of services across all three
// ingestion tables.
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

// GetWebVitalSummaries returns daily metric averages for service over the
// trailing 3 months, newest first.
func GetWebVitalSummaries(ctx context.Context, d *gorm.DB, service string) ([]WebVitalDailySummary, error) {
	cutoff := time.Now().AddDate(0, -3, 0)
	var results []WebVitalDailySummary
	err := d.WithContext(ctx).
		Model(&WebVital{}).
		Select("DATE(created_at) AS day, service, name, AVG(value) AS value").
		Where("service = ? AND created_at >= ?", service, cutoff).
		Group("DATE(created_at), service, name").
		Order("DATE(created_at) DESC").
		Find(&results).Error
	if err != nil {
		return nil, fmt.Errorf("querying web vital summaries: %w", err)
	}
	return results, nil
}

// GetWebVitalAverages returns trailing-28-day metric averages for service.
func GetWebVitalAverages(ctx context.Context, d *gorm.DB, service string) ([]WebVitalAverage, error) {
	cutoff := time.Now().AddDate(0, 0, -28)
	var results []WebVitalAverage
	err := d.WithContext(ctx).
		Model(&WebVital{}).
		Select("name, AVG(value) AS value").
		Where("service = ? AND created_at >= ?", service, cutoff).
		Group("name").
		Order("name").
		Find(&results).Error
	if err != nil {
		return nil, fmt.Errorf("querying web vital averages: %w", err)
	}
	return results, nil
}

// GetReportCounts returns per-day, per-type counts for service across
// both ingestion tables over the trailing 3 months.
func GetReportCounts(ctx context.Context, d *gorm.DB, service string) ([]ReportDailyCount, error) {
	cutoff := time.Now().AddDate(0, -3, 0)
	const daySelect = "DATE(created_at) AS day, report_type, COUNT(*) AS count"

	var rtCounts []ReportDailyCount
	err := d.WithContext(ctx).
		Model(&ReportToEntry{}).
		Select(daySelect).
		Where("service = ? AND created_at >= ?", service, cutoff).
		Group("DATE(created_at), report_type").
		Order("DATE(created_at) DESC").
		Find(&rtCounts).Error
	if err != nil {
		return nil, fmt.Errorf("querying report-to counts: %w", err)
	}

	var srCounts []ReportDailyCount
	err = d.WithContext(ctx).
		Model(&SecurityReportEntry{}).
		Select(daySelect).
		Where("service = ? AND created_at >= ?", service, cutoff).
		Group("DATE(created_at), report_type").
		Order("DATE(created_at) DESC").
		Find(&srCounts).Error
	if err != nil {
		return nil, fmt.Errorf("querying security report counts: %w", err)
	}

	return append(rtCounts, srCounts...), nil
}

// GetRecentReports returns up to limit recent SecurityReportEntry rows
// for service.
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

// GetRecentReportToEntries returns up to limit recent ReportToEntry rows
// for service.
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

// GetTopViolatedDirectives returns up to limit most-violated CSP
// directives for service over the trailing month, merged across both
// ingestion tables.
func GetTopViolatedDirectives(ctx context.Context, d *gorm.DB, service string, limit int) ([]DirectiveCount, error) {
	cutoff := time.Now().AddDate(0, -1, 0)
	cspTypes := []string{reportTypeCSPViolation, reportTypeCSP}
	const directiveExpr = "COALESCE(NULLIF(violated_directive, ''), effective_directive)"
	const whereClause = "service = ? AND created_at >= ? AND report_type IN ? AND " + directiveExpr + " != ''"

	var srResults []DirectiveCount
	err := d.WithContext(ctx).
		Model(&SecurityReportEntry{}).
		Select(directiveExpr+" AS directive, COUNT(*) AS count").
		Where(whereClause, service, cutoff, cspTypes).
		Group(directiveExpr).
		Find(&srResults).Error
	if err != nil {
		return nil, fmt.Errorf("querying top violated directives (security_report): %w", err)
	}

	var rtResults []DirectiveCount
	err = d.WithContext(ctx).
		Model(&ReportToEntry{}).
		Select(directiveExpr+" AS directive, COUNT(*) AS count").
		Where(whereClause, service, cutoff, cspTypes).
		Group(directiveExpr).
		Find(&rtResults).Error
	if err != nil {
		return nil, fmt.Errorf("querying top violated directives (report_to): %w", err)
	}

	merged := make(map[string]int64, len(srResults)+len(rtResults))
	for _, r := range srResults {
		merged[r.Directive] += r.Count
	}
	for _, r := range rtResults {
		merged[r.Directive] += r.Count
	}

	results := make([]DirectiveCount, 0, len(merged))
	for directive, count := range merged {
		results = append(results, DirectiveCount{Directive: directive, Count: count})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Count != results[j].Count {
			return results[i].Count > results[j].Count
		}
		return results[i].Directive < results[j].Directive
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}
