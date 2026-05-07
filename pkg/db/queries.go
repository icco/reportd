package db

import (
	"context"
	"fmt"
	"math"
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
	var rows []WebVital
	err := d.WithContext(ctx).
		Model(&WebVital{}).
		Select("service, name, value").
		Where("created_at >= ?", cutoff).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("querying all services health: %w", err)
	}

	grouped := make(map[string]map[string][]float64)
	for _, row := range rows {
		if _, ok := grouped[row.Service]; !ok {
			grouped[row.Service] = make(map[string][]float64)
		}
		grouped[row.Service][row.Name] = append(grouped[row.Service][row.Name], row.Value)
	}

	out := make(map[string][]ServiceHealth)
	for service, metrics := range grouped {
		for name, values := range metrics {
			out[service] = append(out[service], ServiceHealth{
				Service: service,
				Metric:  name,
				P75:     percentileCont(values, 0.75),
			})
		}
		sort.Slice(out[service], func(i, j int) bool {
			return out[service][i].Metric < out[service][j].Metric
		})
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
		Group("DATE(created_at), service, name").
		Order("DATE(created_at) DESC").
		Find(&results).Error
	if err != nil {
		return nil, fmt.Errorf("querying web vital summaries: %w", err)
	}
	return results, nil
}

func GetWebVitalP75s(ctx context.Context, d *gorm.DB, service string) ([]WebVitalP75, error) {
	cutoff := time.Now().AddDate(0, 0, -28)
	var rows []WebVital
	err := d.WithContext(ctx).
		Model(&WebVital{}).
		Select("name, value").
		Where("service = ? AND created_at >= ?", service, cutoff).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("querying web vital p75s: %w", err)
	}

	grouped := make(map[string][]float64)
	for _, row := range rows {
		grouped[row.Name] = append(grouped[row.Name], row.Value)
	}

	names := make([]string, 0, len(grouped))
	for name := range grouped {
		names = append(names, name)
	}
	sort.Strings(names)

	results := make([]WebVitalP75, 0, len(names))
	for _, name := range names {
		results = append(results, WebVitalP75{
			Name:  name,
			Value: percentileCont(grouped[name], 0.75),
		})
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
		Group("DATE(created_at), report_type").
		Order("DATE(created_at) DESC").
		Find(&rtCounts).Error
	if err != nil {
		return nil, fmt.Errorf("querying report-to counts: %w", err)
	}

	var srCounts []ReportDailyCount
	err = d.WithContext(ctx).
		Model(&SecurityReportEntry{}).
		Select("DATE(created_at) AS day, report_type, COUNT(*) AS count").
		Where("service = ? AND created_at >= ?", service, cutoff).
		Group("DATE(created_at), report_type").
		Order("DATE(created_at) DESC").
		Find(&srCounts).Error
	if err != nil {
		return nil, fmt.Errorf("querying security report counts: %w", err)
	}

	return append(rtCounts, srCounts...), nil
}

func percentileCont(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}

	cpy := append([]float64(nil), values...)
	sort.Float64s(cpy)
	if len(cpy) == 1 {
		return cpy[0]
	}

	rank := p * float64(len(cpy)-1)
	lower := int(math.Floor(rank))
	upper := int(math.Ceil(rank))
	if lower == upper {
		return cpy[lower]
	}

	weight := rank - float64(lower)
	return cpy[lower] + weight*(cpy[upper]-cpy[lower])
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
