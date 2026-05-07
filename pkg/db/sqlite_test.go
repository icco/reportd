package db

import (
	"context"
	"math"
	"testing"
	"time"
)

func TestConnectSQLiteAndQueryHelpers(t *testing.T) {
	ctx := context.Background()

	d, err := Connect(ctx, "sqlite://file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if err := AutoMigrate(ctx, d); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	now := time.Now()
	for _, value := range []float64{1, 2, 3, 4} {
		if err := d.Create(&WebVital{
			CreatedAt: now,
			Service:   "svc",
			Name:      "LCP",
			Value:     value,
		}).Error; err != nil {
			t.Fatalf("creating web vital: %v", err)
		}
	}
	if err := d.Create(&ReportToEntry{
		CreatedAt:  now,
		Service:    "svc",
		ReportType: reportTypeCSP,
	}).Error; err != nil {
		t.Fatalf("creating report_to_entry: %v", err)
	}
	if err := d.Create(&SecurityReportEntry{
		CreatedAt:  now,
		Service:    "svc",
		ReportType: "csp-violation",
	}).Error; err != nil {
		t.Fatalf("creating security_report_entry: %v", err)
	}

	p75s, err := GetWebVitalP75s(ctx, d, "svc")
	if err != nil {
		t.Fatalf("GetWebVitalP75s() error = %v", err)
	}
	if len(p75s) != 1 {
		t.Fatalf("expected 1 p75 metric, got %d", len(p75s))
	}
	if math.Abs(p75s[0].Value-3.25) > 1e-9 {
		t.Fatalf("expected p75 3.25, got %v", p75s[0].Value)
	}

	health, err := GetAllServicesHealth(ctx, d)
	if err != nil {
		t.Fatalf("GetAllServicesHealth() error = %v", err)
	}
	if len(health["svc"]) != 1 {
		t.Fatalf("expected 1 health metric for service, got %d", len(health["svc"]))
	}
	if math.Abs(health["svc"][0].P75-3.25) > 1e-9 {
		t.Fatalf("expected service p75 3.25, got %v", health["svc"][0].P75)
	}

	summaries, err := GetWebVitalSummaries(ctx, d, "svc")
	if err != nil {
		t.Fatalf("GetWebVitalSummaries() error = %v", err)
	}
	if len(summaries) == 0 || summaries[0].Day == "" {
		t.Fatalf("expected non-empty daily summaries, got %+v", summaries)
	}

	counts, err := GetReportCounts(ctx, d, "svc")
	if err != nil {
		t.Fatalf("GetReportCounts() error = %v", err)
	}
	if len(counts) != 2 {
		t.Fatalf("expected 2 report count rows, got %d", len(counts))
	}
	for _, c := range counts {
		if c.Day == "" {
			t.Fatalf("expected non-empty day in counts, got %+v", c)
		}
	}
}
