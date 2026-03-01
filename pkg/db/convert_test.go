package db

import (
	"testing"

	"cloud.google.com/go/bigquery"
	"github.com/icco/reportd/pkg/analytics"
	"github.com/icco/reportd/pkg/reporting"
	"github.com/icco/reportd/pkg/reportto"
)

func nullStr(s string) bigquery.NullString {
	return bigquery.NullString{StringVal: s, Valid: true}
}

func TestWebVitalFromAnalytics(t *testing.T) {
	wv := &analytics.WebVital{
		Name:    "LCP",
		Value:   2500,
		Delta:   100,
		ID:      "v1-abc",
		Label:   bigquery.NullString{StringVal: "web-vital", Valid: true},
		Service: bigquery.NullString{StringVal: "mysite", Valid: true},
	}

	entry := WebVitalFromAnalytics(wv)

	if entry.Service != "mysite" {
		t.Errorf("expected service 'mysite', got %q", entry.Service)
	}
	if entry.Name != "LCP" {
		t.Errorf("expected name 'LCP', got %q", entry.Name)
	}
	if entry.Value != 2500 {
		t.Errorf("expected value 2500, got %f", entry.Value)
	}
	if entry.Delta != 100 {
		t.Errorf("expected delta 100, got %f", entry.Delta)
	}
	if entry.VitalID != "v1-abc" {
		t.Errorf("expected vital_id 'v1-abc', got %q", entry.VitalID)
	}
	if entry.Label != "web-vital" {
		t.Errorf("expected label 'web-vital', got %q", entry.Label)
	}
	if entry.CreatedAt.IsZero() {
		t.Error("created_at should not be zero")
	}
}

func TestReportToEntryFromCSPReport(t *testing.T) {
	r := &reportto.Report{
		Service: bigquery.NullString{StringVal: "mysite", Valid: true},
		CSP: &reportto.CSPReport{
			CSPReport: struct {
				DocumentURI        string `json:"document-uri"`
				Referer            string `json:"referrer"`
				ViolatedDirective  string `json:"violated-directive"`
				EffectiveDirective string `json:"effective-directive"`
				OriginalPolicy     string `json:"original-policy"`
				BlockedURI         string `json:"blocked-uri"`
				StatusCode         int    `json:"status-code"`
				SourceFile         string `json:"source-file"`
				LineNumber         int    `json:"line-number"`
				ColumnNumber       int    `json:"column-number"`
			}{
				DocumentURI:        "https://example.com/",
				BlockedURI:         "https://evil.com/script.js",
				EffectiveDirective: "script-src",
				ViolatedDirective:  "script-src",
				SourceFile:         "https://example.com/app.js",
				LineNumber:         10,
				ColumnNumber:       5,
				StatusCode:         200,
			},
		},
	}

	entry := ReportToEntryFromReport(r)

	if entry.ReportType != "csp" {
		t.Errorf("expected type 'csp', got %q", entry.ReportType)
	}
	if entry.Service != "mysite" {
		t.Errorf("expected service 'mysite', got %q", entry.Service)
	}
	if entry.DocumentURI != "https://example.com/" {
		t.Errorf("expected document_uri, got %q", entry.DocumentURI)
	}
	if entry.BlockedURI != "https://evil.com/script.js" {
		t.Errorf("expected blocked_uri, got %q", entry.BlockedURI)
	}
	if entry.LineNumber != 10 {
		t.Errorf("expected line_number 10, got %d", entry.LineNumber)
	}
	if entry.RawJSON == "" {
		t.Error("RawJSON should not be empty")
	}
}

func TestReportToEntryFromExpectCT(t *testing.T) {
	r := &reportto.Report{
		Service:  bigquery.NullString{StringVal: "mysite", Valid: true},
		ExpectCT: &reportto.ExpectCTReport{},
	}

	entry := ReportToEntryFromReport(r)

	if entry.ReportType != "expect-ct" {
		t.Errorf("expected type 'expect-ct', got %q", entry.ReportType)
	}
}

func TestReportToEntryFromReportTo(t *testing.T) {
	r := &reportto.Report{
		Service: bigquery.NullString{StringVal: "mysite", Valid: true},
		ReportTo: []*reportto.ReportToReport{
			{Type: "deprecation", URL: "https://example.com/"},
		},
	}

	entry := ReportToEntryFromReport(r)

	if entry.ReportType != "report-to" {
		t.Errorf("expected type 'report-to', got %q", entry.ReportType)
	}
}

func TestSecurityReportEntryFromCSP(t *testing.T) {
	sr := &reporting.SecurityReport{
		ReportType: nullStr("csp-violation"),
		RawJSON:    `{"type":"csp-violation"}`,
		Service:    nullStr("mysite"),
		CSP: &reporting.CSPReport{
			URL: "https://example.com/",
			Body: reporting.CSPReportBody{
				DocumentUri:        "https://example.com/page",
				BlockedUri:         "https://evil.com/script.js",
				EffectiveDirective: "script-src-elem",
				SourceFile:         "app.js",
				LineNumber:         42,
				ColumnNumber:       8,
			},
		},
	}

	entry := SecurityReportEntryFromReport(sr)

	if entry.ReportType != "csp-violation" {
		t.Errorf("expected type 'csp-violation', got %q", entry.ReportType)
	}
	if entry.URL != "https://example.com/" {
		t.Errorf("expected URL, got %q", entry.URL)
	}
	if entry.BlockedURI != "https://evil.com/script.js" {
		t.Errorf("expected blocked_uri, got %q", entry.BlockedURI)
	}
	if entry.EffectiveDirective != "script-src-elem" {
		t.Errorf("expected effective_directive, got %q", entry.EffectiveDirective)
	}
	if entry.LineNumber != 42 {
		t.Errorf("expected line 42, got %d", entry.LineNumber)
	}
}

func TestSecurityReportEntryFromDeprecation(t *testing.T) {
	sr := &reporting.SecurityReport{
		ReportType: nullStr("deprecation"),
		RawJSON:    `{"type":"deprecation"}`,
		Service:    nullStr("mysite"),
		Deprecation: &reporting.DeprecationReport{
			URL: "https://example.com/",
			Body: reporting.DeprecationReportBody{
				Message:    "WebSQL is deprecated",
				SourceFile: "db.js",
				LineNumber: 10,
			},
		},
	}

	entry := SecurityReportEntryFromReport(sr)

	if entry.ReportType != "deprecation" {
		t.Errorf("expected type 'deprecation', got %q", entry.ReportType)
	}
	if entry.Message != "WebSQL is deprecated" {
		t.Errorf("expected message, got %q", entry.Message)
	}
	if entry.SourceFile != "db.js" {
		t.Errorf("expected source_file 'db.js', got %q", entry.SourceFile)
	}
}

func TestSecurityReportEntryFromCrash(t *testing.T) {
	sr := &reporting.SecurityReport{
		ReportType: nullStr("crash"),
		RawJSON:    `{"type":"crash"}`,
		Service:    nullStr("mysite"),
		Crash: &reporting.CrashReport{
			URL:  "https://example.com/",
			Body: reporting.CrashReportBody{Reason: "oom"},
		},
	}

	entry := SecurityReportEntryFromReport(sr)

	if entry.ReportType != "crash" {
		t.Errorf("expected type 'crash', got %q", entry.ReportType)
	}
	if entry.Message != "oom" {
		t.Errorf("expected message 'oom', got %q", entry.Message)
	}
}

func TestSecurityReportEntryFromCOEP(t *testing.T) {
	sr := &reporting.SecurityReport{
		ReportType: nullStr("coep"),
		RawJSON:    `{"type":"coep"}`,
		Service:    nullStr("mysite"),
		COEP: &reporting.COEPReport{
			URL: "https://example.com/",
			Body: reporting.COEPReportBody{
				BlockedURL:  "https://cdn.example.com/img.png",
				Disposition: "enforce",
			},
		},
	}

	entry := SecurityReportEntryFromReport(sr)

	if entry.BlockedURI != "https://cdn.example.com/img.png" {
		t.Errorf("expected blocked_uri, got %q", entry.BlockedURI)
	}
	if entry.Message != "enforce" {
		t.Errorf("expected message 'enforce', got %q", entry.Message)
	}
}

func TestSecurityReportEntryFromCOOP(t *testing.T) {
	sr := &reporting.SecurityReport{
		ReportType: nullStr("coop"),
		RawJSON:    `{"type":"coop"}`,
		Service:    nullStr("mysite"),
		COOP: &reporting.COOPReport{
			URL: "https://example.com/",
			Body: reporting.COOPReportBody{
				EffectivePolicy: "same-origin",
			},
		},
	}

	entry := SecurityReportEntryFromReport(sr)

	if entry.Message != "same-origin" {
		t.Errorf("expected message 'same-origin', got %q", entry.Message)
	}
}

func TestSecurityReportEntryFromDocumentPolicy(t *testing.T) {
	sr := &reporting.SecurityReport{
		ReportType: nullStr("document-policy-violation"),
		RawJSON:    `{"type":"document-policy-violation"}`,
		Service:    nullStr("mysite"),
		DocumentPolicy: &reporting.DocumentPolicyReport{
			URL: "https://example.com/",
			Body: reporting.DocumentPolicyReportBody{
				FeatureId:  "oversized-images",
				Message:    "Image too large",
				SourceFile: "index.html",
				LineNumber: 15,
			},
		},
	}

	entry := SecurityReportEntryFromReport(sr)

	if entry.ReportType != "document-policy-violation" {
		t.Errorf("expected type, got %q", entry.ReportType)
	}
	if entry.Message != "Image too large" {
		t.Errorf("expected message, got %q", entry.Message)
	}
	if entry.SourceFile != "index.html" {
		t.Errorf("expected source_file 'index.html', got %q", entry.SourceFile)
	}
}

func TestSecurityReportEntryFromPermissionsPolicy(t *testing.T) {
	sr := &reporting.SecurityReport{
		ReportType: nullStr("permissions-policy-violation"),
		RawJSON:    `{"type":"permissions-policy-violation"}`,
		Service:    nullStr("mysite"),
		PermissionsPolicy: &reporting.PermissionsPolicyReport{
			URL: "https://example.com/",
			Body: reporting.PermissionsPolicyReportBody{
				FeatureId:  "geolocation",
				Message:    "geolocation not allowed",
				SourceFile: "app.js",
				LineNumber: 42,
			},
		},
	}

	entry := SecurityReportEntryFromReport(sr)

	if entry.Message != "geolocation not allowed" {
		t.Errorf("expected message, got %q", entry.Message)
	}
	if entry.LineNumber != 42 {
		t.Errorf("expected line 42, got %d", entry.LineNumber)
	}
}

func TestSecurityReportEntryFromIntervention(t *testing.T) {
	sr := &reporting.SecurityReport{
		ReportType: nullStr("intervention"),
		RawJSON:    `{"type":"intervention"}`,
		Service:    nullStr("mysite"),
		Intervention: &reporting.InterventionReport{
			URL: "https://example.com/",
			Body: reporting.InterventionReportBody{
				Id:         "HeavyAdIntervention",
				Message:    "Ad removed",
				SourceFile: "ads.js",
				LineNumber: 100,
			},
		},
	}

	entry := SecurityReportEntryFromReport(sr)

	if entry.Message != "Ad removed" {
		t.Errorf("expected message, got %q", entry.Message)
	}
	if entry.SourceFile != "ads.js" {
		t.Errorf("expected source_file 'ads.js', got %q", entry.SourceFile)
	}
}
