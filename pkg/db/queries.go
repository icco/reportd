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

func isSQLite(d *gorm.DB) bool {
	return d.Dialector != nil && d.Name() == dialectSQLite
}

// dayExpr returns a SQL expression that yields a 'YYYY-MM-DD' text value for
// the created_at column on the active dialect.
func dayExpr(d *gorm.DB) string {
	if isSQLite(d) {
		return "strftime('%Y-%m-%d', created_at)"
	}
	return "TO_CHAR(DATE(created_at), 'YYYY-MM-DD')"
}

// p75ByGroupSQL returns a query computing the linear-interpolation 75th
// percentile of `value` from web_vitals over the given grouping columns
// (e.g. "name" or "service, name"). Both dialects emit the same column shape.
//
// Postgres uses native PERCENTILE_CONT; SQLite emulates it via window
// functions to keep the Go path identical.
func p75ByGroupSQL(d *gorm.DB, groupCols, valueAlias, where string) string {
	if isSQLite(d) {
		// Linear interpolation: rank = 0.75*(n-1); contribution from
		// floor(rank) is value*(1-frac), from floor(rank)+1 is value*frac.
		return fmt.Sprintf(`
			WITH ranked AS (
				SELECT %[1]s, value,
					ROW_NUMBER() OVER (PARTITION BY %[1]s ORDER BY value) - 1 AS rn,
					COUNT(*) OVER (PARTITION BY %[1]s) AS cnt
				FROM web_vitals
				WHERE %[3]s
			)
			SELECT %[1]s,
				SUM(
					CASE
						WHEN rn = CAST(0.75 * (cnt - 1) AS INTEGER)
							THEN value * (1 - (0.75 * (cnt - 1) - CAST(0.75 * (cnt - 1) AS INTEGER)))
						WHEN rn = CAST(0.75 * (cnt - 1) AS INTEGER) + 1
							THEN value * (0.75 * (cnt - 1) - CAST(0.75 * (cnt - 1) AS INTEGER))
						ELSE 0
					END
				) AS %[2]s
			FROM ranked
			GROUP BY %[1]s
			ORDER BY %[1]s
		`, groupCols, valueAlias, where)
	}
	return fmt.Sprintf(`
		SELECT %[1]s,
			PERCENTILE_CONT(0.75) WITHIN GROUP (ORDER BY value) AS %[2]s
		FROM web_vitals
		WHERE %[3]s AND deleted_at IS NULL
		GROUP BY %[1]s
		ORDER BY %[1]s
	`, groupCols, valueAlias, where)
}

func GetAllServicesHealth(ctx context.Context, d *gorm.DB) (map[string][]ServiceHealth, error) {
	cutoff := time.Now().AddDate(0, 0, -28)
	type row struct {
		Service string
		Name    string
		P75     float64
	}
	var rows []row
	err := d.WithContext(ctx).
		Raw(p75ByGroupSQL(d, "service, name", "p75", "created_at >= ?"), cutoff).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("querying all services health: %w", err)
	}

	out := make(map[string][]ServiceHealth)
	for _, r := range rows {
		out[r.Service] = append(out[r.Service], ServiceHealth{
			Service: r.Service,
			Metric:  r.Name,
			P75:     r.P75,
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
		Select(fmt.Sprintf("%s AS day, service, name, AVG(value) AS value", dayExpr(d))).
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
	var results []WebVitalP75
	err := d.WithContext(ctx).
		Raw(p75ByGroupSQL(d, "name", "value", "service = ? AND created_at >= ?"), service, cutoff).
		Scan(&results).Error
	if err != nil {
		return nil, fmt.Errorf("querying web vital p75s: %w", err)
	}
	return results, nil
}

func GetReportCounts(ctx context.Context, d *gorm.DB, service string) ([]ReportDailyCount, error) {
	cutoff := time.Now().AddDate(0, -3, 0)
	daySelect := fmt.Sprintf("%s AS day, report_type, COUNT(*) AS count", dayExpr(d))

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
