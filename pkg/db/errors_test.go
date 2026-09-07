package db

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/icco/reportd/pkg/reportto"
	"gorm.io/gorm"
)

// newSchemaDB returns a migrated SQLite database backed by a temp file.
func newSchemaDB(t *testing.T) *gorm.DB {
	t.Helper()
	ctx := context.Background()
	d, err := Connect(ctx, "sqlite://"+filepath.Join(t.TempDir(), "errors.db"))
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if err := AutoMigrate(ctx, d); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	return d
}

// dropTable removes one table so a single query inside a helper fails while
// the others still succeed. That is what makes the per-table error returns
// reachable: several helpers query all three tables in sequence and return on
// the first failure, so dropping everything would only ever exercise the first.
func dropTable(t *testing.T, d *gorm.DB, model any) {
	t.Helper()
	if err := d.Migrator().DropTable(model); err != nil {
		t.Fatalf("DropTable(%T) error = %v", model, err)
	}
}

func TestGetServicesErrorPerTable(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name  string
		model any
	}{
		{"web_vitals missing", &WebVital{}},
		{"report_to_entries missing", &ReportToEntry{}},
		{"security_report_entries missing", &SecurityReportEntry{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newSchemaDB(t)
			dropTable(t, d, tt.model)

			if _, err := GetServices(ctx, d); err == nil {
				t.Fatal("GetServices() error = nil, want an error")
			}
		})
	}
}

func TestGetTopViolatedDirectivesErrorPerTable(t *testing.T) {
	ctx := context.Background()

	for _, tt := range []struct {
		name  string
		model any
	}{
		{"security_report_entries missing", &SecurityReportEntry{}},
		{"report_to_entries missing", &ReportToEntry{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := newSchemaDB(t)
			dropTable(t, d, tt.model)

			if _, err := GetTopViolatedDirectives(ctx, d, "svc", 10); err == nil {
				t.Fatal("GetTopViolatedDirectives() error = nil, want an error")
			}
		})
	}
}

func TestGetReportCountsErrorPerTable(t *testing.T) {
	ctx := context.Background()

	for _, tt := range []struct {
		name  string
		model any
	}{
		{"security_report_entries missing", &SecurityReportEntry{}},
		{"report_to_entries missing", &ReportToEntry{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := newSchemaDB(t)
			dropTable(t, d, tt.model)

			if _, err := GetReportCounts(ctx, d, "svc"); err == nil {
				t.Fatal("GetReportCounts() error = nil, want an error")
			}
		})
	}
}

// TestSingleTableQueryErrors covers the helpers that touch exactly one table.
func TestSingleTableQueryErrors(t *testing.T) {
	ctx := context.Background()

	for _, tt := range []struct {
		name  string
		model any
		call  func(*gorm.DB) error
	}{
		{"GetAllServicesHealth", &WebVital{}, func(d *gorm.DB) error {
			_, err := GetAllServicesHealth(ctx, d)
			return err
		}},
		{"GetWebVitalSummaries", &WebVital{}, func(d *gorm.DB) error {
			_, err := GetWebVitalSummaries(ctx, d, "svc")
			return err
		}},
		{"GetWebVitalAverages", &WebVital{}, func(d *gorm.DB) error {
			_, err := GetWebVitalAverages(ctx, d, "svc")
			return err
		}},
		{"GetRecentReports", &SecurityReportEntry{}, func(d *gorm.DB) error {
			_, err := GetRecentReports(ctx, d, "svc", 10)
			return err
		}},
		{"GetRecentReportToEntries", &ReportToEntry{}, func(d *gorm.DB) error {
			_, err := GetRecentReportToEntries(ctx, d, "svc", 10)
			return err
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := newSchemaDB(t)
			dropTable(t, d, tt.model)

			if err := tt.call(d); err == nil {
				t.Fatalf("%s() error = nil, want an error", tt.name)
			}
		})
	}
}

// TestGetTopViolatedDirectivesTruncatesToLimit covers the limit branch, which
// only runs when more distinct directives exist than the caller asked for.
func TestGetTopViolatedDirectivesTruncatesToLimit(t *testing.T) {
	ctx := context.Background()
	d := newSchemaDB(t)

	const service = "limit-svc"
	for _, directive := range []string{"script-src", "img-src", "style-src", "font-src"} {
		entry := &SecurityReportEntry{
			Service:           service,
			ReportType:        "csp-violation",
			ViolatedDirective: directive,
		}
		if err := d.WithContext(ctx).Create(entry).Error; err != nil {
			t.Fatalf("seeding %q: %v", directive, err)
		}
	}

	got, err := GetTopViolatedDirectives(ctx, d, service, 2)
	if err != nil {
		t.Fatalf("GetTopViolatedDirectives() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(results) = %d, want 2 (limit)", len(got))
	}
}

// TestReportToEntriesPrefersBodyDirective covers the override branches: a
// report-to entry that carries its own directive and URL wins over the values
// derived from the CSP fields.
func TestReportToEntriesPrefersBodyDirective(t *testing.T) {
	entry := &reportto.Entry{Type: "csp-violation", URL: "https://example.com/page"}
	entry.Body.Directive = "script-src-elem"
	entry.Body.EffectiveDirective = "script-src"

	report := &reportto.Report{ReportTo: []*reportto.Entry{entry}}

	got := ReportToEntriesFromReport(report)
	if len(got) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(got))
	}
	if got[0].ViolatedDirective != "script-src-elem" {
		t.Errorf("ViolatedDirective = %q, want the body directive to win", got[0].ViolatedDirective)
	}
	if got[0].DocumentURI != "https://example.com/page" {
		t.Errorf("DocumentURI = %q, want the entry URL to win", got[0].DocumentURI)
	}
}
