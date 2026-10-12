package main

import (
	"bufio"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.icco.me/reportd/pkg/db"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"gorm.io/gorm"
)

// dropModel removes a table from a migrated database so a handler's query
// fails — the storage-error branches without an actual outage.
func dropModel(t *testing.T, pgDB *gorm.DB, model any) {
	t.Helper()
	if err := pgDB.Migrator().DropTable(model); err != nil {
		t.Fatalf("DropTable(%T): %v", model, err)
	}
}

// Each read handler should surface a 500 when its table is gone.
func TestHandlerStorageErrors(t *testing.T) {
	for _, tt := range []struct {
		name   string
		model  any
		method string
		target string
	}{
		{"index", &db.WebVital{}, http.MethodGet, "/"},
		{"getReports", &db.SecurityReportEntry{}, http.MethodGet, "/reports/svc"},
		{"getServices", &db.WebVital{}, http.MethodGet, "/services"},
		{"getAnalytics", &db.WebVital{}, http.MethodGet, "/analytics/svc"},
		{"apiVitals", &db.WebVital{}, http.MethodGet, "/api/vitals/svc"},
		{"apiReports", &db.SecurityReportEntry{}, http.MethodGet, "/api/reports/svc"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, pgDB, _ := newTestRouter(t)
			dropModel(t, pgDB, tt.model)

			rr := do(t, h, tt.method, tt.target, nil, "")
			if rr.Code != http.StatusInternalServerError {
				t.Errorf("status = %d, want 500 (body=%q)", rr.Code, rr.Body.String())
			}
		})
	}
}

// The insert-failure branch: the payload parses, then the write has nowhere to go.
func TestIngestStorageErrors(t *testing.T) {
	const (
		reportToBody  = `{"csp-report":{"document-uri":"https://example.com/","blocked-uri":"https://evil.com/","violated-directive":"script-src"}}`
		analyticsBody = `{"id":"v1-abc","name":"LCP","value":2500,"delta":100,"label":"web-vital"}`
		reportingBody = `{"type":"csp-violation","url":"https://example.com/","body":{"document_uri":"https://example.com/","blocked_uri":"https://evil.com/","effective_directive":"script-src"}}`
	)

	for _, tt := range []struct {
		name        string
		model       any
		target      string
		body        string
		contentType string
	}{
		{"postReport", &db.ReportToEntry{}, "/report/svc", reportToBody, "application/csp-report"},
		{"postAnalytics", &db.WebVital{}, "/analytics/svc", analyticsBody, "application/json"},
		{"postReporting", &db.SecurityReportEntry{}, "/reporting/svc", reportingBody, "application/reports+json"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, pgDB, _ := newTestRouter(t)
			dropModel(t, pgDB, tt.model)

			rr := do(t, h, http.MethodPost, tt.target, strings.NewReader(tt.body), tt.contentType)
			if rr.Code != http.StatusInternalServerError {
				t.Errorf("status = %d, want 500 (body=%q)", rr.Code, rr.Body.String())
			}
		})
	}
}

// errReader fails on the first Read, reaching the body-read error branches.
// A client hanging up mid-upload produces the same shape.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("simulated read failure") }

func TestIngestBodyReadErrors(t *testing.T) {
	for _, tt := range []struct {
		name        string
		target      string
		contentType string
	}{
		{"postReport", "/report/svc", "application/csp-report"},
		{"postAnalytics", "/analytics/svc", "application/json"},
		{"postReporting", "/reporting/svc", "application/reports+json"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, _, _ := newTestRouter(t)

			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, tt.target, errReader{})
			req.Header.Set("Content-Type", tt.contentType)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			if rr.Code != http.StatusInternalServerError {
				t.Errorf("status = %d, want 500 (body=%q)", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestIngestTruncatedBodies(t *testing.T) {
	previousLog := log
	core, logs := observer.New(zap.WarnLevel)
	log = zap.New(core).Sugar()
	t.Cleanup(func() { log = previousLog })
	h := newRouter(nil, nil, nil, nil)

	for _, tc := range []struct {
		target      string
		contentType string
	}{
		{"/report/svc", "application/csp-report"},
		{"/analytics/svc", "application/json"},
		{"/reporting/svc", "application/reports+json"},
	} {
		t.Run(tc.target, func(t *testing.T) {
			logs.TakeAll()
			raw := "POST " + tc.target + " HTTP/1.1\r\nHost: example.com\r\nContent-Type: " + tc.contentType + "\r\nContent-Length: 100\r\n\r\n{"
			req, err := http.ReadRequest(bufio.NewReader(strings.NewReader(raw)))
			if err != nil {
				t.Fatal(err)
			}
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req.WithContext(t.Context()))
			if rr.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rr.Code)
			}
			if got := logs.FilterLevelExact(zap.ErrorLevel).Len(); got != 0 {
				t.Errorf("error logs = %d, want 0", got)
			}
			if got := logs.FilterLevelExact(zap.WarnLevel).FilterField(zap.Error(io.ErrUnexpectedEOF)).Len(); got != 1 {
				t.Errorf("incomplete upload warnings = %d, want 1", got)
			}
		})
	}
}

// The two Content-Type guards: an unparseable header, and an unaccepted type.
func TestPostReportingContentTypeRejections(t *testing.T) {
	for _, tt := range []struct {
		name        string
		contentType string
	}{
		{"unparseable", "application/"},
		{"missing", ""},
		{"unsupported media type", "text/plain"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, _, _ := newTestRouter(t)

			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/reporting/svc", strings.NewReader("{}"))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400 (body=%q)", rr.Code, rr.Body.String())
			}
		})
	}
}

// The ParseLegacyCSPReport error arm only runs for application/csp-report.
func TestPostReportingLegacyCSPParseError(t *testing.T) {
	h, _, _ := newTestRouter(t)

	rr := do(t, h, http.MethodPost, "/reporting/svc", strings.NewReader("not json"), "application/csp-report")
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 (body=%q)", rr.Code, rr.Body.String())
	}
}

// 33 characters exceeds the 32-character limit while staying a legal path
// segment, hitting the validation branch on every {service} handler.
func TestServiceValidationRejectsOverlongName(t *testing.T) {
	long := strings.Repeat("a", 33)

	for _, tt := range []struct {
		name   string
		method string
		target string
	}{
		{"view", http.MethodGet, "/view/" + long},
		{"corsPreflight report", http.MethodOptions, "/report/" + long},
		{"corsPreflight analytics", http.MethodOptions, "/analytics/" + long},
		{"getReports", http.MethodGet, "/reports/" + long},
		{"getAnalytics", http.MethodGet, "/analytics/" + long},
		{"apiVitals", http.MethodGet, "/api/vitals/" + long},
		{"apiReports", http.MethodGet, "/api/reports/" + long},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, _, _ := newTestRouter(t)

			rr := do(t, h, tt.method, tt.target, nil, "")
			if rr.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400 (body=%q)", rr.Code, rr.Body.String())
			}
		})
	}
}
