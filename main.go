package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	chi "github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/icco/gutil/logging"
	"github.com/icco/reportd/pkg/analytics"
	"github.com/icco/reportd/pkg/db"
	"github.com/icco/reportd/pkg/lib"
	"github.com/icco/reportd/pkg/reporting"
	"github.com/icco/reportd/pkg/reportto"
	"github.com/icco/reportd/templates"
	"github.com/namsral/flag"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/unrolled/render"
	"github.com/unrolled/secure"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const serverName = "reportd"

var (
	service = "reportd"
	log     = logging.Must(logging.NewLogger(service))
)

func writeJSON(w http.ResponseWriter, data any) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(data)
}

// routeTag stamps the chi route pattern onto otelhttp metric labels.
func routeTag(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		labeler, ok := otelhttp.LabelerFromContext(r.Context())
		if !ok {
			return
		}
		if pattern := chi.RouteContext(r.Context()).RoutePattern(); pattern != "" {
			labeler.Add(semconv.HTTPRoute(pattern))
		}
	})
}

func main() {
	fs := flag.NewFlagSetWithEnvPrefix(os.Args[0], "REPORTD", 0)
	project := fs.String("project", "", "Project ID containing the bigquery dataset to upload to.")
	dataset := fs.String("dataset", "", "The bigquery dataset to upload to.")
	aTable := fs.String("analytics_table", "", "The bigquery table to upload analytics to.")
	rTable := fs.String("reports_table", "", "The bigquery table to upload reports to.")
	rv2Table := fs.String("reports_v2_table", "", "The bigquery table to upload reports to.")
	databaseURL := fs.String("database_url", "", "Database connection string (e.g. postgres://user:pass@host/reportd or sqlite:///tmp/reportd.db).")
	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatalw("error parsing flags", zap.Error(err))
	}

	port := "8080"
	if fromEnv := os.Getenv("PORT"); fromEnv != "" {
		port = fromEnv
	}
	log.Infow("Starting up", "host", fmt.Sprintf("http://localhost:%s", port))

	if *project == "" {
		log.Fatalw("project is required")
	}

	if *dataset == "" {
		log.Fatalw("dataset is required")
	}

	if *aTable == "" {
		log.Fatalw("analytics_table is required")
	}

	if *rTable == "" {
		log.Fatalw("reports_table is required")
	}

	if *databaseURL == "" {
		log.Fatalw("database_url is required")
	}

	ctx := context.Background()

	registry := prometheus.NewRegistry()
	exporter, err := otelprom.New(otelprom.WithRegisterer(registry))
	if err != nil {
		log.Fatalw("otel prometheus exporter", zap.Error(err))
	}
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter))
	otel.SetMeterProvider(mp)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := mp.Shutdown(shutdownCtx); err != nil {
			log.Warnw("meter provider shutdown", zap.Error(err))
		}
	}()

	pgDB, err := db.Connect(ctx, *databaseURL)
	if err != nil {
		log.Fatalw("could not connect to database", zap.Error(err))
	}
	if err := db.AutoMigrate(ctx, pgDB); err != nil {
		log.Fatalw("could not auto-migrate database", zap.Error(err))
	}
	log.Infow("Database connected and migrated")

	if err := reportto.UpdateReportsBQSchema(ctx, *project, *dataset, *rTable); err != nil {
		log.Errorw("report table update", zap.Error(err))
	}

	if err := analytics.UpdateAnalyticsBQSchema(ctx, *project, *dataset, *aTable); err != nil {
		log.Errorw("analytics table update", zap.Error(err))
	}

	if err := reporting.UpdateReportsBQSchema(ctx, *project, *dataset, *rv2Table); err != nil {
		log.Errorw("reporting table update", zap.Error(err))
	}

	r := chi.NewRouter()
	r.Use(logging.Middleware(log.Desugar()))
	r.Use(routeTag)
	r.Use(middleware.Compress(5))

	r.Use(cors.New(cors.Options{
		// AllowCredentials must stay false to keep the wildcard origin safe (CORS bypass).
		AllowCredentials:   false,
		OptionsPassthrough: true,
		AllowedOrigins:     []string{"*"},
		AllowedMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:     []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:     []string{"Link"},
		MaxAge:             300,
	}).Handler)

	r.Use(middleware.Timeout(30 * time.Second))

	r.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("report-to", `{"group":"default","max_age":10886400,"endpoints":[{"url":"https://reportd.natwelch.com/report/reportd"}]}`)
			w.Header().Set("reporting-endpoints", `default="https://reportd.natwelch.com/reporting/reportd"`)

			h.ServeHTTP(w, r)
		})
	})

	secureMiddleware := secure.New(secure.Options{
		SSLRedirect:          false,
		SSLProxyHeaders:      map[string]string{"X-Forwarded-Proto": "https"},
		STSSeconds:           63072000,
		STSIncludeSubdomains: true,
		STSPreload:           true,
		FrameDeny:            true,
		ContentTypeNosniff:   true,
		BrowserXssFilter:     true,
		ReferrerPolicy:       "no-referrer",
		PermissionsPolicy:    "geolocation=(), midi=(), sync-xhr=(), microphone=(), camera=(), magnetometer=(), gyroscope=(), fullscreen=(), payment=(), usb=()",
	})
	r.Use(secureMiddleware.Handler)

	re := render.New(render.Options{
		Directory:  ".",
		FileSystem: &render.EmbedFileSystem{FS: templates.FS},
		Extensions: []string{".tmpl"},
	})

	r.Get("/robots.txt", robotsTxtHandler())
	r.Get("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	r.Get("/", indexHandler(re, pgDB))
	r.Get("/view/{service}", viewHandler(re))
	r.Get("/healthz", healthzHandler())

	r.Options("/report/{service}", corsPreflightHandler())
	r.Options("/analytics/{service}", corsPreflightHandler())

	r.Get("/reports/{service}", getReportsHandler(pgDB))
	r.Post("/report/{service}", postReportHandler(pgDB, *project, *dataset, *rTable))

	r.Get("/services", getServicesHandler(pgDB))
	r.Get("/analytics/{service}", getAnalyticsHandler(pgDB))
	r.Post("/analytics/{service}", postAnalyticsHandler(pgDB, *project, *dataset, *aTable))

	r.Post("/reporting/{service}", postReportingHandler(pgDB, *project, *dataset, *rv2Table))

	r.Get("/api/vitals/{service}", apiVitalsHandler(pgDB))
	r.Get("/api/reports/{service}", apiReportsHandler(pgDB))

	r.Method(http.MethodGet, "/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	handler := otelhttp.NewHandler(r, serverName,
		otelhttp.WithFilter(func(req *http.Request) bool {
			return req.URL.Path != "/metrics"
		}),
	)

	srv := http.Server{
		Addr:              ":" + port,
		WriteTimeout:      30 * time.Second,
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
		Handler:           handler,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalw("Server failed", zap.Error(err))
		}
	}()

	log.Infow("Server ready", "addr", srv.Addr)
	<-done
	log.Infow("Shutting down")

	shutdownCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalw("Shutdown failed", zap.Error(err))
	}

	log.Infow("Server stopped")
}

func indexHandler(re *render.Render, pgDB *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logging.FromContext(ctx)

		services, err := db.GetServices(ctx, pgDB)
		if err != nil {
			l.Errorw("error getting services", zap.Error(err))
			http.Error(w, "could not get services", 500)
			return
		}

		health, err := db.GetAllServicesHealth(ctx, pgDB)
		if err != nil {
			l.Errorw("error getting services health", zap.Error(err))
			health = make(map[string][]db.ServiceHealth)
		}

		healthJSON, _ := json.Marshal(health)

		if err := re.HTML(w, http.StatusOK, "index", struct {
			Services   []string
			HealthJSON string
		}{
			Services:   services,
			HealthJSON: string(healthJSON),
		}); err != nil {
			l.Errorw("error rendering index", zap.Error(err))
			http.Error(w, "could not render index", 500)
			return
		}
	}
}

func viewHandler(re *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logging.FromContext(r.Context())
		service := chi.URLParam(r, "service")

		if err := lib.ValidateService(service); err != nil {
			l.Errorw("error validating service", zap.Error(err), "service", service)
			http.Error(w, "could not validate service", 400)
			return
		}

		if err := re.HTML(w, http.StatusOK, "view", struct {
			Service string
		}{
			Service: service,
		}); err != nil {
			l.Errorw("error rendering view", zap.Error(err), "service", service)
			http.Error(w, "could not render view", 500)
			return
		}
	}
}

func healthzHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("ok.")); err != nil {
			logging.FromContext(r.Context()).Errorw("error writing healthz", zap.Error(err))
		}
	}
}

func robotsTxtHandler() http.HandlerFunc {
	body, err := fs.ReadFile(templates.FS, "robots.txt")
	if err != nil {
		log.Fatalw("could not read embedded robots.txt", zap.Error(err))
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		if _, err := w.Write(body); err != nil {
			logging.FromContext(r.Context()).Errorw("error writing robots.txt", zap.Error(err))
		}
	}
}

func corsPreflightHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logging.FromContext(r.Context())
		service := chi.URLParam(r, "service")
		if err := lib.ValidateService(service); err != nil {
			l.Errorw("error validating service", zap.Error(err), "service", service)
			http.Error(w, "could not validate service", 400)
			return
		}
		if _, err := w.Write([]byte("")); err != nil {
			l.Errorw("error writing options", zap.Error(err))
		}
	}
}

func getReportsHandler(pgDB *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logging.FromContext(ctx)
		service := chi.URLParam(r, "service")

		if err := lib.ValidateService(service); err != nil {
			l.Errorw("error validating service", zap.Error(err), "service", service)
			http.Error(w, "could not validate service", 400)
			return
		}

		data, err := db.GetReportCounts(ctx, pgDB, service)
		if err != nil {
			l.Errorw("error getting report counts from postgres", zap.Error(err), "service", service)
			http.Error(w, "processing error", 500)
			return
		}

		if err := writeJSON(w, data); err != nil {
			l.Errorw("error writing reports", zap.Error(err), "service", service)
		}
	}
}

func postReportHandler(pgDB *gorm.DB, project, dataset, rTable string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logging.FromContext(ctx)
		service := chi.URLParam(r, "service")

		if err := lib.ValidateService(service); err != nil {
			l.Errorw("error validating service", zap.Error(err), "service", service)
			http.Error(w, "could not validate service", 400)
			return
		}

		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(r.Body); err != nil {
			l.Errorw("error reading body", zap.Error(err), "service", service)
			http.Error(w, "uploading error", 500)
			return
		}
		bodyStr := buf.String()
		ct := r.Header.Get("content-type")

		data, err := reportto.ParseReport(ct, bodyStr, service)
		if err != nil {
			l.Errorw("error seen during report parse", "content-type", ct, "user-agent", r.UserAgent(), "bodyJson", bodyStr, zap.Error(err), "service", service)
			http.Error(w, "processing error", 500)
			return
		}

		l.Infow("report received", "content-type", ct, "service", service, "user-agent", r.UserAgent(), "report", data)

		entries := db.ReportToEntriesFromReport(data)
		if err := pgDB.WithContext(ctx).Create(&entries).Error; err != nil {
			l.Errorw("error writing report to postgres", zap.Error(err), "service", service)
			http.Error(w, "storage error", 500)
			return
		}

		w.WriteHeader(http.StatusNoContent)

		// Background goroutine outlives the request; use the package logger.
		go func() {
			bgCtx := context.WithoutCancel(ctx)
			if err := reportto.WriteReportToBigQuery(bgCtx, project, dataset, rTable, []*reportto.Report{data}); err != nil {
				log.Errorw("error during report upload to bigquery", "dataset", dataset, "project", project, "table", rTable, "bodyJson", bodyStr, zap.Error(err), "service", service)
			}
		}()
	}
}

func getServicesHandler(pgDB *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logging.FromContext(ctx)

		data, err := db.GetServices(ctx, pgDB)
		if err != nil {
			l.Errorw("error seen during services get", zap.Error(err))
			http.Error(w, "processing error", 500)
			return
		}

		if err := writeJSON(w, data); err != nil {
			l.Errorw("error writing services", zap.Error(err))
		}
	}
}

func getAnalyticsHandler(pgDB *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logging.FromContext(ctx)
		service := chi.URLParam(r, "service")

		if err := lib.ValidateService(service); err != nil {
			l.Errorw("error validating service", zap.Error(err), "service", service)
			http.Error(w, "could not validate service", 400)
			return
		}

		data, err := db.GetWebVitalSummaries(ctx, pgDB, service)
		if err != nil {
			l.Errorw("error getting analytics from postgres", zap.Error(err), "service", service)
			http.Error(w, "processing error", 500)
			return
		}

		if err := writeJSON(w, data); err != nil {
			l.Errorw("error writing analytics", zap.Error(err), "service", service)
		}
	}
}

func postAnalyticsHandler(pgDB *gorm.DB, project, dataset, aTable string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logging.FromContext(ctx)
		service := chi.URLParam(r, "service")
		ct := r.Header.Get("content-type")

		if err := lib.ValidateService(service); err != nil {
			l.Errorw("error validating service", zap.Error(err), "service", service)
			http.Error(w, "could not validate service", 400)
			return
		}

		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(r.Body); err != nil {
			l.Errorw("error reading body", zap.Error(err), "service", service)
			http.Error(w, "uploading error", 500)
			return
		}
		bodyStr := buf.String()
		data, err := analytics.ParseAnalytics(bodyStr, service)
		if err != nil {
			l.Errorw("error seen during analytics parse", zap.Error(err), "content-type", ct, "user-agent", r.UserAgent(), "bodyJson", bodyStr, "service", service)
			http.Error(w, "processing error", 500)
			return
		}

		l.Infow("analytics received", "content-type", ct, "service", service, "user-agent", r.UserAgent(), "analytics", data)

		entry := db.WebVitalFromAnalytics(data)
		if err := pgDB.WithContext(ctx).Create(entry).Error; err != nil {
			l.Errorw("error writing analytics to postgres", zap.Error(err), "service", service)
			http.Error(w, "storage error", 500)
			return
		}

		w.WriteHeader(http.StatusNoContent)

		go func() {
			bgCtx := context.WithoutCancel(ctx)
			if err := analytics.WriteAnalyticsToBigQuery(bgCtx, project, dataset, aTable, []*analytics.WebVital{data}); err != nil {
				log.Errorw("error during analytics upload to bigquery", "dataset", dataset, "project", project, "table", aTable, "bodyJson", bodyStr, zap.Error(err), "service", service)
			}
		}()
	}
}

func postReportingHandler(pgDB *gorm.DB, project, dataset, rv2Table string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logging.FromContext(ctx)
		service := chi.URLParam(r, "service")
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/reports+json" {
			l.Errorw("Content-Type header is not application/reports+json", "service", service, "content-type", contentType)
			http.Error(w, "uploading error", 400)
			return
		}

		if err := lib.ValidateService(service); err != nil {
			l.Errorw("error validating service", zap.Error(err), "service", service)
			http.Error(w, "could not validate service", 400)
			return
		}

		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(r.Body); err != nil {
			l.Errorw("error reading body", zap.Error(err), "service", service, "content-type", contentType)
			http.Error(w, "uploading error", 500)
			return
		}
		bodyStr := buf.String()

		l.Infow("reporting received", "content-type", contentType, "service", service, "user-agent", r.UserAgent())
		reports, err := reporting.ParseReport(bodyStr, service)
		if err != nil {
			l.Errorw("error on parsing reporting data", zap.Error(err), "service", service, "content-type", contentType, "body", bodyStr)
			http.Error(w, "uploading error", 500)
			return
		}

		l.Infow("reporting parsed", "reports", reports, "service", service, "content-type", contentType, "user-agent", r.UserAgent())

		entry := db.SecurityReportEntryFromReport(reports)
		if err := pgDB.WithContext(ctx).Create(entry).Error; err != nil {
			l.Errorw("error writing reporting to postgres", zap.Error(err), "service", service)
			http.Error(w, "storage error", 500)
			return
		}

		w.WriteHeader(http.StatusNoContent)

		go func() {
			bgCtx := context.WithoutCancel(ctx)
			if err := reporting.WriteReportsToBigQuery(bgCtx, project, dataset, rv2Table, reports); err != nil {
				log.Errorw("error during reporting upload to bigquery", "dataset", dataset, "project", project, "table", rv2Table, zap.Error(err))
			}
		}()
	}
}

func apiVitalsHandler(pgDB *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logging.FromContext(ctx)
		service := chi.URLParam(r, "service")

		if err := lib.ValidateService(service); err != nil {
			l.Errorw("error validating service", zap.Error(err), "service", service)
			http.Error(w, "could not validate service", 400)
			return
		}

		averages, err := db.GetWebVitalAverages(ctx, pgDB, service)
		if err != nil {
			l.Errorw("error getting averages", zap.Error(err), "service", service)
			http.Error(w, "processing error", 500)
			return
		}

		summaries, err := db.GetWebVitalSummaries(ctx, pgDB, service)
		if err != nil {
			l.Errorw("error getting summaries", zap.Error(err), "service", service)
			http.Error(w, "processing error", 500)
			return
		}

		out := struct {
			Averages  []db.WebVitalAverage      `json:"averages"`
			Summaries []db.WebVitalDailySummary `json:"summaries"`
		}{
			Averages:  averages,
			Summaries: summaries,
		}

		if err := writeJSON(w, out); err != nil {
			l.Errorw("error writing vitals", zap.Error(err), "service", service)
		}
	}
}

func apiReportsHandler(pgDB *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logging.FromContext(ctx)
		service := chi.URLParam(r, "service")

		if err := lib.ValidateService(service); err != nil {
			l.Errorw("error validating service", zap.Error(err), "service", service)
			http.Error(w, "could not validate service", 400)
			return
		}

		counts, err := db.GetReportCounts(ctx, pgDB, service)
		if err != nil {
			l.Errorw("error getting report counts", zap.Error(err), "service", service)
			http.Error(w, "processing error", 500)
			return
		}

		recent, err := db.GetRecentReports(ctx, pgDB, service, 50)
		if err != nil {
			l.Errorw("error getting recent reports", zap.Error(err), "service", service)
			http.Error(w, "processing error", 500)
			return
		}

		recentRT, err := db.GetRecentReportToEntries(ctx, pgDB, service, 50)
		if err != nil {
			l.Errorw("error getting recent report-to entries", zap.Error(err), "service", service)
			http.Error(w, "processing error", 500)
			return
		}

		topDirectives, err := db.GetTopViolatedDirectives(ctx, pgDB, service, 10)
		if err != nil {
			l.Errorw("error getting top violated directives", zap.Error(err), "service", service)
			http.Error(w, "processing error", 500)
			return
		}

		out := struct {
			Counts         []db.ReportDailyCount    `json:"counts"`
			RecentReports  []db.SecurityReportEntry `json:"recent_reports"`
			RecentReportTo []db.ReportToEntry       `json:"recent_report_to"`
			TopDirectives  []db.DirectiveCount      `json:"top_directives"`
		}{
			Counts:         counts,
			RecentReports:  recent,
			RecentReportTo: recentRT,
			TopDirectives:  topDirectives,
		}

		if err := writeJSON(w, out); err != nil {
			l.Errorw("error writing reports", zap.Error(err), "service", service)
		}
	}
}
