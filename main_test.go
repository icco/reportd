package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/icco/reportd/pkg/analytics"
	"github.com/icco/reportd/pkg/db"
	"github.com/icco/reportd/pkg/reporting"
	"github.com/icco/reportd/pkg/reportto"
	"gorm.io/gorm"
)

// recordingWriters captures invocations of the BigQuery writer hooks so tests
// can assert that the post-handler goroutines fire without performing any
// real network I/O.
type recordingWriters struct {
	mu              sync.Mutex
	reports         []*reportto.Report
	analyticsRows   []*analytics.WebVital
	securityReports []*reporting.SecurityReport
	doneAnalytics   chan struct{}
	doneReport      chan struct{}
	doneSecurityRpt chan struct{}
}

func newRecordingWriters() *recordingWriters {
	return &recordingWriters{
		doneAnalytics:   make(chan struct{}, 8),
		doneReport:      make(chan struct{}, 8),
		doneSecurityRpt: make(chan struct{}, 8),
	}
}

func (w *recordingWriters) writeReport(_ context.Context, r *reportto.Report) {
	w.mu.Lock()
	w.reports = append(w.reports, r)
	w.mu.Unlock()
	w.doneReport <- struct{}{}
}

func (w *recordingWriters) writeAnalytics(_ context.Context, r *analytics.WebVital) {
	w.mu.Lock()
	w.analyticsRows = append(w.analyticsRows, r)
	w.mu.Unlock()
	w.doneAnalytics <- struct{}{}
}

func (w *recordingWriters) writeSecurityReport(_ context.Context, r *reporting.SecurityReport) {
	w.mu.Lock()
	w.securityReports = append(w.securityReports, r)
	w.mu.Unlock()
	w.doneSecurityRpt <- struct{}{}
}

// newTestRouter wires up the same router as main() but against a fresh
// per-test SQLite DB and recording BigQuery writers, so the test stays
// hermetic and isolated from other test functions.
func newTestRouter(t *testing.T) (http.Handler, *gorm.DB, *recordingWriters) {
	t.Helper()
	ctx := context.Background()
	dsn := "sqlite://" + filepath.Join(t.TempDir(), "reportd.db")
	pgDB, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("db.Connect: %v", err)
	}
	if err := db.AutoMigrate(ctx, pgDB); err != nil {
		t.Fatalf("db.AutoMigrate: %v", err)
	}
	rec := newRecordingWriters()
	router := newRouter(pgDB, rec.writeReport, rec.writeAnalytics, rec.writeSecurityReport)
	return router, pgDB, rec
}

func do(t *testing.T, h http.Handler, method, target string, body io.Reader, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// waitForSignal returns true if a value is received on ch within timeout.
func waitForSignal(ch chan struct{}, timeout time.Duration) bool {
	select {
	case <-ch:
		return true
	case <-time.After(timeout):
		return false
	}
}

func TestRobotsTxtHandler(t *testing.T) {
	h, _, _ := newTestRouter(t)
	rr := do(t, h, http.MethodGet, "/robots.txt", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if got := rr.Body.String(); !strings.Contains(got, "User-agent") {
		t.Errorf("body should contain a robots directive, got %q", got)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("content-type = %q, want text/plain prefix", ct)
	}
}

func TestHealthzHandler(t *testing.T) {
	h, _, _ := newTestRouter(t)
	rr := do(t, h, http.MethodGet, "/healthz", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if got := rr.Body.String(); got != "ok." {
		t.Errorf("body = %q, want %q", got, "ok.")
	}
}

func TestFaviconHandler(t *testing.T) {
	h, _, _ := newTestRouter(t)
	rr := do(t, h, http.MethodGet, "/favicon.ico", nil, "")
	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rr.Code)
	}
}

func TestIndexHandler(t *testing.T) {
	h, _, _ := newTestRouter(t)
	rr := do(t, h, http.MethodGet, "/", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Body.String(); !strings.Contains(got, "ReportD") {
		t.Errorf("body should contain 'ReportD', got %d bytes", len(got))
	}
}

func TestViewHandler(t *testing.T) {
	h, _, _ := newTestRouter(t)

	rr := do(t, h, http.MethodGet, "/view/mysite", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if got := rr.Body.String(); !strings.Contains(got, "mysite") {
		t.Errorf("body should mention service name, got %d bytes", len(got))
	}

	rr = do(t, h, http.MethodGet, "/view/bad%20service", nil, "")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("invalid service: status = %d, want 400", rr.Code)
	}
}

func TestCorsPreflightHandler(t *testing.T) {
	h, _, _ := newTestRouter(t)

	rr := do(t, h, http.MethodOptions, "/report/svc", nil, "")
	if rr.Code != http.StatusOK {
		t.Errorf("valid service options: status = %d, want 200", rr.Code)
	}

	rr = do(t, h, http.MethodOptions, "/analytics/svc", nil, "")
	if rr.Code != http.StatusOK {
		t.Errorf("valid analytics options: status = %d, want 200", rr.Code)
	}

	rr = do(t, h, http.MethodOptions, "/report/bad.service", nil, "")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("invalid service: status = %d, want 400", rr.Code)
	}
}

func TestGetServicesHandler(t *testing.T) {
	h, pgDB, _ := newTestRouter(t)

	rr := do(t, h, http.MethodGet, "/services", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("empty: status = %d, want 200", rr.Code)
	}
	var got []string
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("json: %v body=%s", err, rr.Body.String())
	}
	if len(got) != 0 {
		t.Errorf("empty DB should yield no services, got %v", got)
	}

	if err := pgDB.Create(&db.WebVital{
		CreatedAt: time.Now(),
		Service:   "alpha",
		Name:      "LCP",
		Value:     1,
	}).Error; err != nil {
		t.Fatalf("seed web vital: %v", err)
	}

	rr = do(t, h, http.MethodGet, "/services", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("json: %v", err)
	}
	if len(got) != 1 || got[0] != "alpha" {
		t.Errorf("services = %v, want [alpha]", got)
	}
}

func TestGetReportsHandler(t *testing.T) {
	h, pgDB, _ := newTestRouter(t)

	rr := do(t, h, http.MethodGet, "/reports/bad.service", nil, "")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("invalid service: status = %d, want 400", rr.Code)
	}

	if err := pgDB.Create(&db.ReportToEntry{
		CreatedAt:         time.Now(),
		Service:           "svc",
		ReportType:        "csp",
		ViolatedDirective: "script-src",
		RawJSON:           "{}",
	}).Error; err != nil {
		t.Fatalf("seed report-to: %v", err)
	}

	rr = do(t, h, http.MethodGet, "/reports/svc", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	// Unmarshal with day as string because db.Day implements MarshalJSON but
	// not UnmarshalJSON; we just need to confirm the wire shape.
	var got []struct {
		Day        string `json:"day"`
		ReportType string `json:"report_type"`
		Count      int64  `json:"count"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("json: %v body=%s", err, rr.Body.String())
	}
	if len(got) == 0 {
		t.Errorf("expected non-empty counts, got %v", got)
	}
}

func TestGetAnalyticsHandler(t *testing.T) {
	h, pgDB, _ := newTestRouter(t)

	rr := do(t, h, http.MethodGet, "/analytics/bad.service", nil, "")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("invalid service: status = %d, want 400", rr.Code)
	}

	if err := pgDB.Create(&db.WebVital{
		CreatedAt: time.Now(),
		Service:   "svc",
		Name:      "LCP",
		Value:     1234,
	}).Error; err != nil {
		t.Fatalf("seed web vital: %v", err)
	}

	rr = do(t, h, http.MethodGet, "/analytics/svc", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var got []struct {
		Day     string  `json:"day"`
		Service string  `json:"service"`
		Name    string  `json:"name"`
		Value   float64 `json:"value"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("json: %v body=%s", err, rr.Body.String())
	}
	if len(got) == 0 {
		t.Errorf("expected non-empty summaries, got %v", got)
	}
}

func TestApiVitalsHandler(t *testing.T) {
	h, pgDB, _ := newTestRouter(t)

	rr := do(t, h, http.MethodGet, "/api/vitals/bad.service", nil, "")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("invalid service: status = %d, want 400", rr.Code)
	}

	if err := pgDB.Create(&db.WebVital{
		CreatedAt: time.Now(),
		Service:   "svc",
		Name:      "LCP",
		Value:     500,
	}).Error; err != nil {
		t.Fatalf("seed web vital: %v", err)
	}

	rr = do(t, h, http.MethodGet, "/api/vitals/svc", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var got struct {
		Averages []struct {
			Name  string  `json:"name"`
			Value float64 `json:"value"`
		} `json:"averages"`
		Summaries []struct {
			Day     string  `json:"day"`
			Service string  `json:"service"`
			Name    string  `json:"name"`
			Value   float64 `json:"value"`
		} `json:"summaries"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("json: %v", err)
	}
	if len(got.Averages) != 1 || got.Averages[0].Name != "LCP" {
		t.Errorf("averages = %+v, want one LCP", got.Averages)
	}
	if len(got.Summaries) == 0 {
		t.Errorf("summaries should be non-empty")
	}
}

func TestApiReportsHandler(t *testing.T) {
	h, pgDB, _ := newTestRouter(t)

	rr := do(t, h, http.MethodGet, "/api/reports/bad.service", nil, "")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("invalid service: status = %d, want 400", rr.Code)
	}

	if err := pgDB.Create(&db.SecurityReportEntry{
		CreatedAt:         time.Now(),
		Service:           "svc",
		ReportType:        "csp-violation",
		ViolatedDirective: "script-src",
		RawJSON:           "{}",
	}).Error; err != nil {
		t.Fatalf("seed security: %v", err)
	}
	if err := pgDB.Create(&db.ReportToEntry{
		CreatedAt:         time.Now(),
		Service:           "svc",
		ReportType:        "csp",
		ViolatedDirective: "img-src",
		RawJSON:           "{}",
	}).Error; err != nil {
		t.Fatalf("seed report-to: %v", err)
	}

	rr = do(t, h, http.MethodGet, "/api/reports/svc", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rr.Code, rr.Body.String())
	}
	var got struct {
		Counts []struct {
			Day        string `json:"day"`
			ReportType string `json:"report_type"`
			Count      int64  `json:"count"`
		} `json:"counts"`
		RecentReports  []db.SecurityReportEntry `json:"recent_reports"`
		RecentReportTo []db.ReportToEntry       `json:"recent_report_to"`
		TopDirectives  []db.DirectiveCount      `json:"top_directives"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("json: %v body=%s", err, rr.Body.String())
	}
	if len(got.RecentReports) == 0 || len(got.RecentReportTo) == 0 {
		t.Errorf("recent_reports=%d recent_report_to=%d", len(got.RecentReports), len(got.RecentReportTo))
	}
	if len(got.TopDirectives) == 0 {
		t.Errorf("top_directives should be non-empty")
	}
	if len(got.Counts) == 0 {
		t.Errorf("counts should be non-empty")
	}
}

func TestPostReportHandler(t *testing.T) {
	h, pgDB, rec := newTestRouter(t)

	rr := do(t, h, http.MethodPost, "/report/bad.service", strings.NewReader(`{"csp-report":{}}`), "application/csp-report")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("invalid service: status = %d, want 400", rr.Code)
	}

	rr = do(t, h, http.MethodPost, "/report/svc", strings.NewReader("not json"), "application/csp-report")
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("malformed body: status = %d, want 500", rr.Code)
	}

	body := `{"csp-report":{"document-uri":"https://example.com/","blocked-uri":"https://evil.com/","violated-directive":"script-src"}}`
	rr = do(t, h, http.MethodPost, "/report/svc", strings.NewReader(body), "application/csp-report")
	if rr.Code != http.StatusNoContent {
		t.Fatalf("valid: status = %d, want 204, body=%s", rr.Code, rr.Body.String())
	}

	var count int64
	if err := pgDB.Model(&db.ReportToEntry{}).Where("service = ?", "svc").Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 report-to row, got %d", count)
	}

	if !waitForSignal(rec.doneReport, 2*time.Second) {
		t.Error("expected BQ writer to be invoked")
	}
}

func TestPostAnalyticsHandler(t *testing.T) {
	h, pgDB, rec := newTestRouter(t)

	rr := do(t, h, http.MethodPost, "/analytics/bad.service", strings.NewReader("{}"), "application/json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("invalid service: status = %d, want 400", rr.Code)
	}

	rr = do(t, h, http.MethodPost, "/analytics/svc", strings.NewReader("not json"), "application/json")
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("malformed body: status = %d, want 500", rr.Code)
	}

	body := `{"id":"v1-abc","name":"LCP","value":2500,"delta":100,"label":"web-vital"}`
	rr = do(t, h, http.MethodPost, "/analytics/svc", strings.NewReader(body), "application/json")
	if rr.Code != http.StatusNoContent {
		t.Fatalf("valid: status = %d, want 204, body=%s", rr.Code, rr.Body.String())
	}

	var count int64
	if err := pgDB.Model(&db.WebVital{}).Where("service = ?", "svc").Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 web vital row, got %d", count)
	}

	if !waitForSignal(rec.doneAnalytics, 2*time.Second) {
		t.Error("expected BQ writer to be invoked")
	}
}

func TestPostReportingHandler(t *testing.T) {
	h, pgDB, rec := newTestRouter(t)

	rr := do(t, h, http.MethodPost, "/reporting/svc", strings.NewReader("{}"), "application/json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("wrong content-type: status = %d, want 400", rr.Code)
	}

	rr = do(t, h, http.MethodPost, "/reporting/bad.service", strings.NewReader("{}"), "application/reports+json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("invalid service: status = %d, want 400", rr.Code)
	}

	rr = do(t, h, http.MethodPost, "/reporting/svc", strings.NewReader("not json"), "application/reports+json")
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("malformed body: status = %d, want 500", rr.Code)
	}

	body := `{"type":"csp-violation","url":"https://example.com/","body":{"document_uri":"https://example.com/","blocked_uri":"https://evil.com/","effective_directive":"script-src","line_number":10,"column_number":5}}`
	rr = do(t, h, http.MethodPost, "/reporting/svc", strings.NewReader(body), "application/reports+json")
	if rr.Code != http.StatusNoContent {
		t.Fatalf("valid: status = %d, want 204, body=%s", rr.Code, rr.Body.String())
	}

	var count int64
	if err := pgDB.Model(&db.SecurityReportEntry{}).Where("service = ?", "svc").Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 security report row, got %d", count)
	}

	if !waitForSignal(rec.doneSecurityRpt, 2*time.Second) {
		t.Error("expected BQ writer to be invoked")
	}
}

func TestWriteJSON(t *testing.T) {
	rr := httptest.NewRecorder()
	if err := writeJSON(rr, map[string]any{"hello": "world"}); err != nil {
		t.Fatalf("writeJSON: %v", err)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}
	got := strings.TrimSpace(rr.Body.String())
	if got != `{"hello":"world"}` {
		t.Errorf("body = %s, want JSON object", got)
	}
}

func TestRouteTagMiddleware(t *testing.T) {
	h, _, _ := newTestRouter(t)
	// routeTag is wired into the router; we just check that hitting a routed
	// path with an otelhttp labeler in the context does not panic. Since the
	// router's middleware stack wraps requests anyway, a normal request is a
	// sufficient smoke test.
	rr := do(t, h, http.MethodGet, "/healthz", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("routed request failed: %d", rr.Code)
	}

	// And confirm the post-handler "report-to" header is set by the inline
	// middleware, since that's a non-trivial code path.
	if rr.Header().Get("report-to") == "" {
		t.Error("report-to header should be set on every response")
	}
	if rr.Header().Get("reporting-endpoints") == "" {
		t.Error("reporting-endpoints header should be set on every response")
	}
}
