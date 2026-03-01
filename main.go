package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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
	"github.com/namsral/flag"
	"github.com/unrolled/render"
	"github.com/unrolled/secure"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	service = "reportd"
	log     = logging.Must(logging.NewLogger(service))
)

func main() {
	fs := flag.NewFlagSetWithEnvPrefix(os.Args[0], "REPORTD", 0)
	project := fs.String("project", "", "Project ID containing the bigquery dataset to upload to.")
	dataset := fs.String("dataset", "", "The bigquery dataset to upload to.")
	aTable := fs.String("analytics_table", "", "The bigquery table to upload analytics to.")
	rTable := fs.String("reports_table", "", "The bigquery table to upload reports to.")
	rv2Table := fs.String("reports_v2_table", "", "The bigquery table to upload reports to.")
	databaseURL := fs.String("database_url", "", "Postgres connection string (e.g. postgres://user:pass@host/reportd).")
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

	pgDB, err := db.Connect(ctx, *databaseURL)
	if err != nil {
		log.Fatalw("could not connect to postgres", zap.Error(err))
	}
	if err := db.AutoMigrate(ctx, pgDB); err != nil {
		log.Fatalw("could not auto-migrate postgres", zap.Error(err))
	}
	log.Infow("Postgres connected and migrated")

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
	r.Use(middleware.RealIP)
	r.Use(logging.Middleware(log.Desugar(), *project))

	r.Use(cors.New(cors.Options{
		AllowCredentials:   true,
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
		SSLRedirect:        false,
		SSLProxyHeaders:    map[string]string{"X-Forwarded-Proto": "https"},
		FrameDeny:          true,
		ContentTypeNosniff: true,
		BrowserXssFilter:   true,
		ReferrerPolicy:     "no-referrer",
		FeaturePolicy:      "geolocation 'none'; midi 'none'; sync-xhr 'none'; microphone 'none'; camera 'none'; magnetometer 'none'; gyroscope 'none'; fullscreen 'none'; payment 'none'; usb 'none'",
	})
	r.Use(secureMiddleware.Handler)

	r.Get("/", indexHandler(pgDB))
	r.Get("/view/{service}", viewHandler())
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

	srv := http.Server{
		Addr:         ":" + port,
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
		Handler:      r,
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalw("Server failed", zap.Error(err))
	}
}

func indexHandler(pgDB *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		re := render.New()

		services, err := db.GetServices(ctx, pgDB)
		if err != nil {
			log.Errorw("error getting services", zap.Error(err))
			http.Error(w, "could not get services", 500)
			return
		}

		health, err := db.GetAllServicesHealth(ctx, pgDB)
		if err != nil {
			log.Errorw("error getting services health", zap.Error(err))
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
			log.Errorw("error rendering index", zap.Error(err))
			http.Error(w, "could not render index", 500)
			return
		}
	}
}

func viewHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		service := chi.URLParam(r, "service")
		re := render.New()

		if err := lib.ValidateService(service); err != nil {
			log.Errorw("error validating service", zap.Error(err), "service", service)
			http.Error(w, "could not validate service", 400)
			return
		}

		if err := re.HTML(w, http.StatusOK, "view", struct {
			Service string
		}{
			Service: service,
		}); err != nil {
			log.Errorw("error rendering view", zap.Error(err), "service", service)
			http.Error(w, "could not render view", 500)
			return
		}
	}
}

func healthzHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("ok.")); err != nil {
			log.Errorw("error writing healthz", zap.Error(err))
		}
	}
}

func corsPreflightHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		service := chi.URLParam(r, "service")
		if err := lib.ValidateService(service); err != nil {
			log.Errorw("error validating service", zap.Error(err), "service", service)
			http.Error(w, "could not validate service", 400)
			return
		}
		if _, err := w.Write([]byte("")); err != nil {
			log.Errorw("error writing options", zap.Error(err))
		}
	}
}

func getReportsHandler(pgDB *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		service := chi.URLParam(r, "service")

		if err := lib.ValidateService(service); err != nil {
			log.Errorw("error validating service", zap.Error(err), "service", service)
			http.Error(w, "could not validate service", 400)
			return
		}

		data, err := db.GetReportCounts(ctx, pgDB, service)
		if err != nil {
			log.Errorw("error getting report counts from postgres", zap.Error(err), "service", service)
			http.Error(w, "processing error", 500)
			return
		}

		resp, err := json.Marshal(data)
		if err != nil {
			log.Errorw("error seen during reports marshal", zap.Error(err), "service", service)
			http.Error(w, "processing error", 500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(resp); err != nil {
			log.Errorw("error writing reports", zap.Error(err), "service", service)
		}
	}
}

func postReportHandler(pgDB *gorm.DB, project, dataset, rTable string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		service := chi.URLParam(r, "service")

		if err := lib.ValidateService(service); err != nil {
			log.Errorw("error validating service", zap.Error(err), "service", service)
			http.Error(w, "could not validate service", 400)
			return
		}

		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(r.Body); err != nil {
			log.Errorw("error reading body", zap.Error(err), "service", service)
			http.Error(w, "uploading error", 500)
			return
		}
		bodyStr := buf.String()
		ct := r.Header.Get("content-type")

		data, err := reportto.ParseReport(ct, bodyStr, service)
		if err != nil {
			log.Errorw("error seen during report parse", "content-type", ct, "user-agent", r.UserAgent(), "bodyJson", bodyStr, zap.Error(err), "service", service)
			http.Error(w, "processing error", 500)
			return
		}

		log.Infow("report received", "content-type", ct, "service", service, "user-agent", r.UserAgent(), "report", data)

		entry := db.ReportToEntryFromReport(data)
		if err := pgDB.WithContext(ctx).Create(entry).Error; err != nil {
			log.Errorw("error writing report to postgres", zap.Error(err), "service", service)
		}

		go func() {
			bgCtx := context.Background()
			if err := reportto.WriteReportToBigQuery(bgCtx, project, dataset, rTable, []*reportto.Report{data}); err != nil {
				log.Errorw("error during report upload to bigquery", "dataset", dataset, "project", project, "table", rTable, "bodyJson", bodyStr, zap.Error(err), "service", service)
			}
		}()
	}
}

func getServicesHandler(pgDB *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		data, err := db.GetServices(ctx, pgDB)
		if err != nil {
			log.Errorw("error seen during services get", zap.Error(err))
			http.Error(w, "processing error", 500)
			return
		}

		resp, err := json.Marshal(data)
		if err != nil {
			log.Errorw("error seen during services marshal", zap.Error(err))
			http.Error(w, "processing error", 500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(resp); err != nil {
			log.Errorw("error writing services", zap.Error(err))
		}
	}
}

func getAnalyticsHandler(pgDB *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		service := chi.URLParam(r, "service")

		if err := lib.ValidateService(service); err != nil {
			log.Errorw("error validating service", zap.Error(err), "service", service)
			http.Error(w, "could not validate service", 400)
			return
		}

		data, err := db.GetWebVitalSummaries(ctx, pgDB, service)
		if err != nil {
			log.Errorw("error getting analytics from postgres", zap.Error(err), "service", service)
			http.Error(w, "processing error", 500)
			return
		}

		resp, err := json.Marshal(data)
		if err != nil {
			log.Errorw("error seen during analytics marshal", zap.Error(err), "service", service)
			http.Error(w, "processing error", 500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(resp); err != nil {
			log.Errorw("error writing analytics", zap.Error(err), "service", service)
		}
	}
}

func postAnalyticsHandler(pgDB *gorm.DB, project, dataset, aTable string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		service := chi.URLParam(r, "service")
		ct := r.Header.Get("content-type")

		if err := lib.ValidateService(service); err != nil {
			log.Errorw("error validating service", zap.Error(err), "service", service)
			http.Error(w, "could not validate service", 400)
			return
		}

		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(r.Body); err != nil {
			log.Errorw("error reading body", zap.Error(err), "service", service)
			http.Error(w, "uploading error", 500)
			return
		}
		bodyStr := buf.String()
		data, err := analytics.ParseAnalytics(bodyStr, service)
		if err != nil {
			log.Errorw("error seen during analytics parse", zap.Error(err), "content-type", ct, "user-agent", r.UserAgent(), "bodyJson", bodyStr, "service", service)
			http.Error(w, "processing error", 500)
			return
		}

		log.Infow("analytics received", "content-type", ct, "service", service, "user-agent", r.UserAgent(), "analytics", data)

		entry := db.WebVitalFromAnalytics(data)
		if err := pgDB.WithContext(ctx).Create(entry).Error; err != nil {
			log.Errorw("error writing analytics to postgres", zap.Error(err), "service", service)
		}

		go func() {
			bgCtx := context.Background()
			if err := analytics.WriteAnalyticsToBigQuery(bgCtx, project, dataset, aTable, []*analytics.WebVital{data}); err != nil {
				log.Errorw("error during analytics upload to bigquery", "dataset", dataset, "project", project, "table", aTable, "bodyJson", bodyStr, zap.Error(err), "service", service)
			}
		}()
	}
}

func postReportingHandler(pgDB *gorm.DB, project, dataset, rv2Table string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		service := chi.URLParam(r, "service")
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/reports+json" {
			log.Errorw("Content-Type header is not application/reports+json", "service", service, "content-type", contentType)
			http.Error(w, "uploading error", 400)
			return
		}

		if err := lib.ValidateService(service); err != nil {
			log.Errorw("error validating service", zap.Error(err), "service", service)
			http.Error(w, "could not validate service", 400)
			return
		}

		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(r.Body); err != nil {
			log.Errorw("error reading body", zap.Error(err), "service", service, "content-type", contentType)
			http.Error(w, "uploading error", 500)
			return
		}
		bodyStr := buf.String()

		log.Infow("reporting received", "content-type", contentType, "service", service, "user-agent", r.UserAgent())
		reports, err := reporting.ParseReport(bodyStr, service)
		if err != nil {
			log.Errorw("error on parsing reporting data", zap.Error(err), "service", service, "content-type", contentType, "body", bodyStr)
			http.Error(w, "uploading error", 500)
			return
		}

		log.Infow("reporting parsed", "reports", reports, "service", service, "content-type", contentType, "user-agent", r.UserAgent())

		entry := db.SecurityReportEntryFromReport(reports)
		if err := pgDB.WithContext(ctx).Create(entry).Error; err != nil {
			log.Errorw("error writing reporting to postgres", zap.Error(err), "service", service)
		}

		go func() {
			bgCtx := context.Background()
			if err := reporting.WriteReportsToBigQuery(bgCtx, project, dataset, rv2Table, reports); err != nil {
				log.Errorw("error during reporting upload to bigquery", "dataset", dataset, "project", project, "table", rv2Table, zap.Error(err))
			}
		}()
	}
}

func apiVitalsHandler(pgDB *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		service := chi.URLParam(r, "service")

		if err := lib.ValidateService(service); err != nil {
			log.Errorw("error validating service", zap.Error(err), "service", service)
			http.Error(w, "could not validate service", 400)
			return
		}

		p75s, err := db.GetWebVitalP75s(ctx, pgDB, service)
		if err != nil {
			log.Errorw("error getting p75s", zap.Error(err), "service", service)
			http.Error(w, "processing error", 500)
			return
		}

		summaries, err := db.GetWebVitalSummaries(ctx, pgDB, service)
		if err != nil {
			log.Errorw("error getting summaries", zap.Error(err), "service", service)
			http.Error(w, "processing error", 500)
			return
		}

		out := struct {
			P75s      []db.WebVitalP75         `json:"p75s"`
			Summaries []db.WebVitalDailySummary `json:"summaries"`
		}{
			P75s:      p75s,
			Summaries: summaries,
		}

		resp, err := json.Marshal(out)
		if err != nil {
			log.Errorw("error marshaling vitals", zap.Error(err), "service", service)
			http.Error(w, "processing error", 500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(resp); err != nil {
			log.Errorw("error writing vitals", zap.Error(err), "service", service)
		}
	}
}

func apiReportsHandler(pgDB *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		service := chi.URLParam(r, "service")

		if err := lib.ValidateService(service); err != nil {
			log.Errorw("error validating service", zap.Error(err), "service", service)
			http.Error(w, "could not validate service", 400)
			return
		}

		counts, err := db.GetReportCounts(ctx, pgDB, service)
		if err != nil {
			log.Errorw("error getting report counts", zap.Error(err), "service", service)
			http.Error(w, "processing error", 500)
			return
		}

		recent, err := db.GetRecentReports(ctx, pgDB, service, 50)
		if err != nil {
			log.Errorw("error getting recent reports", zap.Error(err), "service", service)
			http.Error(w, "processing error", 500)
			return
		}

		recentRT, err := db.GetRecentReportToEntries(ctx, pgDB, service, 50)
		if err != nil {
			log.Errorw("error getting recent report-to entries", zap.Error(err), "service", service)
			http.Error(w, "processing error", 500)
			return
		}

		topDirectives, err := db.GetTopViolatedDirectives(ctx, pgDB, service, 10)
		if err != nil {
			log.Errorw("error getting top violated directives", zap.Error(err), "service", service)
			http.Error(w, "processing error", 500)
			return
		}

		out := struct {
			Counts         []db.ReportDailyCount     `json:"counts"`
			RecentReports  []db.SecurityReportEntry   `json:"recent_reports"`
			RecentReportTo []db.ReportToEntry         `json:"recent_report_to"`
			TopDirectives  []db.DirectiveCount        `json:"top_directives"`
		}{
			Counts:         counts,
			RecentReports:  recent,
			RecentReportTo: recentRT,
			TopDirectives:  topDirectives,
		}

		resp, err := json.Marshal(out)
		if err != nil {
			log.Errorw("error marshaling reports", zap.Error(err), "service", service)
			http.Error(w, "processing error", 500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(resp); err != nil {
			log.Errorw("error writing reports", zap.Error(err), "service", service)
		}
	}
}
