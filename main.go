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
	"github.com/icco/reportd/pkg/lib"
	"github.com/icco/reportd/pkg/reporting"
	"github.com/icco/reportd/pkg/reportto"
	"github.com/namsral/flag"
	"github.com/unrolled/render"
	"github.com/unrolled/secure"
	"go.uber.org/zap"
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
	fs.Parse(os.Args[1:])

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

	ctx := context.Background()
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
		MaxAge:             300, // Maximum value not ignored by any of major browsers
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

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		re := render.New()
		services, err := lib.GetServices(r.Context(), *project, *dataset, *aTable, *rTable)
		if err != nil {
			log.Errorw("error getting services", zap.Error(err))
			http.Error(w, "could not get services", 500)
			return
		}
		if err := re.HTML(w, http.StatusOK, "index", struct {
			Services []string
		}{
			Services: services,
		}); err != nil {
			log.Errorw("error rendering index", zap.Error(err))
			http.Error(w, "could not render index", 500)
			return
		}
	})

	r.Get("/view/{service}", func(w http.ResponseWriter, r *http.Request) {
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
	})

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok."))
	})

	// Needed because some browsers fire off an OPTIONS request before sending a
	// POST to validate CORS.
	r.Options("/report/{service}", func(w http.ResponseWriter, r *http.Request) {
		service := chi.URLParam(r, "service")

		if err := lib.ValidateService(service); err != nil {
			log.Errorw("error validating service", zap.Error(err), "service", service)
			http.Error(w, "could not validate service", 400)
			return
		}
		w.Write([]byte(""))
	})

	r.Options("/analytics/{service}", func(w http.ResponseWriter, r *http.Request) {
		service := chi.URLParam(r, "service")

		if err := lib.ValidateService(service); err != nil {
			log.Errorw("error validating service", zap.Error(err), "service", service)
			http.Error(w, "could not validate service", 400)
			return
		}
		w.Write([]byte(""))
	})

	r.Get("/reports/{service}", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		service := chi.URLParam(r, "service")

		if err := lib.ValidateService(service); err != nil {
			log.Errorw("error validating service", zap.Error(err), "service", service)
			http.Error(w, "could not validate service", 400)
			return
		}

		data, err := reportto.GetReportCounts(ctx, service, *project, *dataset, *rTable)
		if err != nil {
			log.Errorw("error seen during reports get", zap.Error(err), "service", service)
			http.Error(w, "processing error", 500)
			return
		}

		data2, err := reporting.GetReportCounts(ctx, service, *project, *dataset, *rv2Table)
		if err != nil {
			log.Errorw("error seen during reports get", zap.Error(err), "service", service)
			http.Error(w, "processing error", 500)
			return
		}

		out := append(data, data2...)

		resp, err := json.Marshal(out)
		if err != nil {
			log.Errorw("error seen during reports marshal", zap.Error(err), "service", service)
			http.Error(w, "processing error", 500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(resp)
	})

	r.Post("/report/{service}", func(w http.ResponseWriter, r *http.Request) {
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

		// Log the report.
		log.Infow("report received", "content-type", ct, "service", service, "user-agent", r.UserAgent(), "report", data)

		if err := reportto.WriteReportToBigQuery(ctx, *project, *dataset, *rTable, []*reportto.Report{data}); err != nil {
			log.Errorw("error during report upload", "dataset", *dataset, "project", *project, "table", *rTable, "bodyJson", bodyStr, zap.Error(err), "service", service)
			http.Error(w, "uploading error", 500)
			return
		}
	})

	r.Get("/services", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		data, err := lib.GetServices(ctx, *project, *dataset, *aTable, *rTable)
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
		w.Write(resp)
	})

	r.Get("/analytics/{service}", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		service := chi.URLParam(r, "service")

		if err := lib.ValidateService(service); err != nil {
			log.Errorw("error validating service", zap.Error(err), "service", service)
			http.Error(w, "could not validate service", 400)
			return
		}

		data, err := analytics.GetAnalytics(ctx, service, *project, *dataset, *aTable)
		if err != nil {
			log.Errorw("error seen during analytics get", zap.Error(err), "service", service)
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
		w.Write(resp)
	})

	r.Post("/analytics/{service}", func(w http.ResponseWriter, r *http.Request) {
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

		// Log the report.
		log.Infow("analytics received", "content-type", ct, "service", service, "user-agent", r.UserAgent(), "analytics", data)
		if err := analytics.WriteAnalyticsToBigQuery(ctx, *project, *dataset, *aTable, []*analytics.WebVital{data}); err != nil {
			log.Errorw("error during analytics upload", "dataset", *dataset, "project", *project, "table", *aTable, "bodyJson", bodyStr, zap.Error(err), "service", service)
			http.Error(w, "uploading error", 500)
			return
		}
	})

	r.Post("/reporting/{service}", func(w http.ResponseWriter, r *http.Request) {
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
		if err := reporting.WriteReportsToBigQuery(ctx, *project, *dataset, *rv2Table, reports); err != nil {
			log.Errorw("error during reporting upload", "dataset", *dataset, "project", *project, "table", *rTable, zap.Error(err))
			http.Error(w, "uploading error", 500)
			return
		}
	})

	srv := http.Server{
		Addr:         ":" + port,
		WriteTimeout: 1 * time.Second,
		ReadTimeout:  1 * time.Second,
		Handler:      r,
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalw("Server failed", zap.Error(err))
	}
}
