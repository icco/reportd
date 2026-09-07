package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// failingWriter accepts headers but fails every Write, reaching the "error
// writing response" branches. Same as a client disconnecting after the status.
type failingWriter struct {
	header http.Header
	code   int
}

func newFailingWriter() *failingWriter {
	return &failingWriter{header: make(http.Header)}
}

func (f *failingWriter) Header() http.Header       { return f.header }
func (f *failingWriter) WriteHeader(code int)      { f.code = code }
func (f *failingWriter) Write([]byte) (int, error) { return 0, errors.New("simulated write failure") }

// requestWithService carries a chi route parameter so a handler can be called
// directly. Required: the router owns its ResponseWriter, so a failing one can
// only be injected by invoking the handler.
func requestWithService(t *testing.T, service string) *http.Request {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("service", service)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// The handlers log and return on a write failure, so the assertion is that
// they neither panic nor report success through the writer.
func TestHandlerWriteErrors(t *testing.T) {
	_, pgDB, _ := newTestRouter(t)

	for _, tt := range []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"healthz", healthzHandler()},
		{"robotsTxt", robotsTxtHandler()},
		{"corsPreflight", corsPreflightHandler()},
		{"getReports", getReportsHandler(pgDB)},
		{"getServices", getServicesHandler(pgDB)},
		{"getAnalytics", getAnalyticsHandler(pgDB)},
		{"apiVitals", apiVitalsHandler(pgDB)},
		{"apiReports", apiReportsHandler(pgDB)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := newFailingWriter()
			tt.handler(w, requestWithService(t, "svc"))

			if w.code >= 500 {
				t.Errorf("status = %d, want the write failure to be logged, not a 5xx", w.code)
			}
		})
	}
}

// routeTag's labeler branch is skipped unless an otelhttp labeler is present.
func TestRouteTagAddsRoutePattern(t *testing.T) {
	var called bool
	inner := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })

	r := chi.NewRouter()
	r.Use(routeTag)
	r.Get("/view/{service}", inner)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/view/svc", nil)
	ctx, _ := otelhttp.LabelerFromContext(req.Context())
	req = req.WithContext(otelhttp.ContextWithLabeler(req.Context(), ctx))

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if !called {
		t.Fatal("inner handler was not reached")
	}
}
