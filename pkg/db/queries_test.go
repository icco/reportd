package db

import (
	"context"
	"math"
	"testing"
	"time"

	"gorm.io/gorm"
)

// seedQueryFixtures inserts a fixed set of web-vital and CSP rows under the
// given service name, so the same scenario can be exercised against multiple
// dialects. Callers should pick a service name that is unique to the test run
// when the underlying DB persists across tests (e.g. Postgres).
func seedQueryFixtures(t *testing.T, d *gorm.DB, service string) {
	t.Helper()
	now := time.Now()
	for _, value := range []float64{1, 2, 3, 4} {
		if err := d.Create(&WebVital{
			CreatedAt: now,
			Service:   service,
			Name:      "LCP",
			Value:     value,
		}).Error; err != nil {
			t.Fatalf("creating web vital: %v", err)
		}
	}
	if err := d.Create(&ReportToEntry{
		CreatedAt:         now,
		Service:           service,
		ReportType:        reportTypeCSP,
		ViolatedDirective: "script-src",
	}).Error; err != nil {
		t.Fatalf("creating report_to_entry: %v", err)
	}
	if err := d.Create(&ReportToEntry{
		CreatedAt:          now,
		Service:            service,
		ReportType:         reportTypeCSP,
		EffectiveDirective: "img-src",
	}).Error; err != nil {
		t.Fatalf("creating report_to_entry: %v", err)
	}
	if err := d.Create(&SecurityReportEntry{
		CreatedAt:         now,
		Service:           service,
		ReportType:        "csp-violation",
		ViolatedDirective: "script-src",
	}).Error; err != nil {
		t.Fatalf("creating security_report_entry: %v", err)
	}
	if err := d.Create(&SecurityReportEntry{
		CreatedAt:          now,
		Service:            service,
		ReportType:         "csp-violation",
		EffectiveDirective: "style-src",
	}).Error; err != nil {
		t.Fatalf("creating security_report_entry: %v", err)
	}
}

// assertQueryHelpers runs the read-side query helpers against the seeded
// fixtures and asserts the expected aggregates.
func assertQueryHelpers(t *testing.T, ctx context.Context, d *gorm.DB, service string) {
	t.Helper()

	avgs, err := GetWebVitalAverages(ctx, d, service)
	if err != nil {
		t.Fatalf("GetWebVitalAverages() error = %v", err)
	}
	if len(avgs) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(avgs))
	}
	if math.Abs(avgs[0].Value-2.5) > 1e-9 {
		t.Fatalf("expected average 2.5, got %v", avgs[0].Value)
	}

	health, err := GetAllServicesHealth(ctx, d)
	if err != nil {
		t.Fatalf("GetAllServicesHealth() error = %v", err)
	}
	if len(health[service]) != 1 {
		t.Fatalf("expected 1 health metric for service, got %d", len(health[service]))
	}
	if math.Abs(health[service][0].Average-2.5) > 1e-9 {
		t.Fatalf("expected service average 2.5, got %v", health[service][0].Average)
	}

	summaries, err := GetWebVitalSummaries(ctx, d, service)
	if err != nil {
		t.Fatalf("GetWebVitalSummaries() error = %v", err)
	}
	if len(summaries) == 0 || time.Time(summaries[0].Day).IsZero() {
		t.Fatalf("expected non-empty daily summaries, got %+v", summaries)
	}

	counts, err := GetReportCounts(ctx, d, service)
	if err != nil {
		t.Fatalf("GetReportCounts() error = %v", err)
	}
	if len(counts) != 2 {
		t.Fatalf("expected 2 report count rows, got %d", len(counts))
	}
	for _, c := range counts {
		if time.Time(c.Day).IsZero() {
			t.Fatalf("expected non-empty day in counts, got %+v", c)
		}
	}

	directives, err := GetTopViolatedDirectives(ctx, d, service, 10)
	if err != nil {
		t.Fatalf("GetTopViolatedDirectives() error = %v", err)
	}
	got := map[string]int64{}
	for _, dc := range directives {
		got[dc.Directive] = dc.Count
	}
	want := map[string]int64{
		"script-src": 2,
		"img-src":    1,
		"style-src":  1,
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d directives, got %d: %+v", len(want), len(got), directives)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("directive %q: want count %d, got %d", k, v, got[k])
		}
	}
}
