package db

import (
	"context"
	"math"
	"strings"
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
	// RawJSON must be a syntactically-valid JSON document because the column
	// is `gorm:"type:jsonb"` and Postgres rejects empty strings (SQLite is
	// flexible). Production code in pkg/db/convert.go always populates this
	// via json.Marshal; the fixtures should match that.
	if err := d.Create(&ReportToEntry{
		CreatedAt:         now,
		Service:           service,
		ReportType:        reportTypeCSP,
		ViolatedDirective: "script-src",
		RawJSON:           "{}",
	}).Error; err != nil {
		t.Fatalf("creating report_to_entry: %v", err)
	}
	if err := d.Create(&ReportToEntry{
		CreatedAt:          now,
		Service:            service,
		ReportType:         reportTypeCSP,
		EffectiveDirective: "img-src",
		RawJSON:            "{}",
	}).Error; err != nil {
		t.Fatalf("creating report_to_entry: %v", err)
	}
	if err := d.Create(&SecurityReportEntry{
		CreatedAt:         now,
		Service:           service,
		ReportType:        "csp-violation",
		ViolatedDirective: "script-src",
		RawJSON:           "{}",
	}).Error; err != nil {
		t.Fatalf("creating security_report_entry: %v", err)
	}
	if err := d.Create(&SecurityReportEntry{
		CreatedAt:          now,
		Service:            service,
		ReportType:         "csp-violation",
		EffectiveDirective: "style-src",
		RawJSON:            "{}",
	}).Error; err != nil {
		t.Fatalf("creating security_report_entry: %v", err)
	}
}

// assertQueryHelpers runs the read-side query helpers against the seeded
// fixtures and asserts the expected aggregates.
func assertQueryHelpers(ctx context.Context, t *testing.T, d *gorm.DB, service string) {
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

	services, err := GetServices(ctx, d)
	if err != nil {
		t.Fatalf("GetServices() error = %v", err)
	}
	if !containsString(services, service) {
		t.Errorf("GetServices() = %v, want it to contain %q", services, service)
	}

	recent, err := GetRecentReports(ctx, d, service, 10)
	if err != nil {
		t.Fatalf("GetRecentReports() error = %v", err)
	}
	if len(recent) != 2 {
		t.Errorf("GetRecentReports() returned %d rows, want 2", len(recent))
	}
	for _, r := range recent {
		if r.Service != service {
			t.Errorf("recent report has wrong service: %q", r.Service)
		}
	}

	recentRT, err := GetRecentReportToEntries(ctx, d, service, 10)
	if err != nil {
		t.Fatalf("GetRecentReportToEntries() error = %v", err)
	}
	if len(recentRT) != 2 {
		t.Errorf("GetRecentReportToEntries() returned %d rows, want 2", len(recentRT))
	}

	// Limit clamps results.
	limited, err := GetRecentReports(ctx, d, service, 1)
	if err != nil {
		t.Fatalf("GetRecentReports(limit=1) error = %v", err)
	}
	if len(limited) != 1 {
		t.Errorf("limit=1: got %d rows, want 1", len(limited))
	}
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func TestDayMarshalJSON(t *testing.T) {
	d := Day(time.Date(2026, 5, 7, 12, 34, 56, 0, time.UTC))
	got, err := d.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	if string(got) != `"2026-05-07"` {
		t.Errorf("MarshalJSON = %s, want \"2026-05-07\"", got)
	}
}

func TestDayScan(t *testing.T) {
	want := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		input     any
		wantErr   bool
		wantZero  bool
		wantEqual bool
	}{
		{name: "nil", input: nil, wantZero: true},
		{name: "time.Time", input: time.Date(2026, 5, 7, 9, 0, 0, 0, time.UTC), wantEqual: true},
		{name: "string date-only", input: "2026-05-07", wantEqual: true},
		{name: "string ISO timestamp", input: "2026-05-07T09:30:00Z", wantEqual: true},
		{name: "byte slice", input: []byte("2026-05-07"), wantEqual: true},
		{name: "invalid string", input: "not-a-date", wantErr: true},
		{name: "unsupported int", input: 42, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var d Day
			err := d.Scan(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Scan(%v) err = %v, wantErr %v", tc.input, err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			got := time.Time(d)
			if tc.wantZero {
				if !got.IsZero() {
					t.Errorf("expected zero time, got %v", got)
				}
				return
			}
			if tc.wantEqual && !got.Equal(want) && got.Format(time.DateOnly) != "2026-05-07" {
				t.Errorf("got %v, want date 2026-05-07", got)
			}
		})
	}
}

func TestDayScanInvalidDateRejected(t *testing.T) {
	var d Day
	err := d.Scan("garbage-date")
	if err == nil {
		t.Fatal("expected error for invalid date string")
	}
	if !strings.Contains(err.Error(), "parsing day") {
		t.Errorf("error %q should mention 'parsing day'", err.Error())
	}
}
