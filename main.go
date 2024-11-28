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
	"github.com/icco/reportd/lib"
	"github.com/icco/reportd/static"
	"github.com/namsral/flag"
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
	if err := lib.UpdateReportsBQSchema(ctx, *project, *dataset, *rTable); err != nil {
		log.Errorw("report table update", zap.Error(err))
	}

	if err := lib.UpdateAnalyticsBQSchema(ctx, *project, *dataset, *aTable); err != nil {
		log.Errorw("analytics table update", zap.Error(err))
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

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		index, err := static.Get("index.html")
		if err != nil {
			http.Error(w, "could not load index", 500)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		w.Write(index)
	})

	r.Get("/sparklines.js", func(w http.ResponseWriter, r *http.Request) {
		js, err := static.Get("sparklines.js")
		if err != nil {
			http.Error(w, "could not load sparklines.js", 500)
			return
		}

		w.Header().Set("Content-Type", "application/javascript")
		w.Write(js)
	})

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok."))
	})

	// Needed because some browsers fire off an OPTIONS request before sending a
	// POST to validate CORS.
	r.Options("/report/{bucket}", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(""))
	})

	r.Options("/analytics/{bucket}", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(""))
	})

	r.Get("/report/{bucket}", func(w http.ResponseWriter, r *http.Request) {
		bucket := chi.URLParam(r, "bucket")
		newURL := fmt.Sprintf("/view/%s", bucket)

		http.Redirect(w, r, newURL, http.StatusPermanentRedirect)
	})

	r.Post("/report/{bucket}", func(w http.ResponseWriter, r *http.Request) {
		bucket := chi.URLParam(r, "bucket")

		buf := new(bytes.Buffer)
		buf.ReadFrom(r.Body)
		bodyStr := buf.String()
		ct := r.Header.Get("content-type")

		data, err := lib.ParseReport(ct, bodyStr, bucket)
		if err != nil {
			log.Errorw("error seen during report parse", "content-type", ct, "user-agent", r.UserAgent(), "bodyJson", bodyStr, zap.Error(err))
			http.Error(w, "processing error", 500)
			return
		}

		// Log the report.
		log.Infow("report recieved", "content-type", ct, "bucket", bucket, "user-agent", r.UserAgent(), "report", data)

		if err := lib.WriteReportToBigQuery(r.Context(), *project, *dataset, *rTable, []*lib.Report{data}); err != nil {
			log.Errorw("error during report upload", "dataset", *dataset, "project", *project, "table", *rTable, "bodyJson", bodyStr, zap.Error(err))
			http.Error(w, "uploading error", 500)
			return
		}
	})

	r.Get("/analytics", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		data, err := lib.GetAnalyticsServices(ctx, *project, *dataset, *aTable)
		if err != nil {
			log.Errorw("error seen during analytics services get", zap.Error(err))
			http.Error(w, "processing error", 500)
			return
		}

		resp, err := json.Marshal(data)
		if err != nil {
			log.Errorw("error seen during analytics marshal", zap.Error(err))
			http.Error(w, "processing error", 500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(resp)
	})

	r.Get("/analytics/{bucket}", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		bucket := chi.URLParam(r, "bucket")

		data, err := lib.GetAnalytics(ctx, bucket, *project, *dataset, *aTable)
		if err != nil {
			log.Errorw("error seen during analytics get", zap.Error(err), "bucket", bucket)
			http.Error(w, "processing error", 500)
			return
		}

		resp, err := json.Marshal(data)
		if err != nil {
			log.Errorw("error seen during analytics marshal", zap.Error(err), "bucket", bucket)
			http.Error(w, "processing error", 500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(resp)
	})

	r.Post("/analytics/{bucket}", func(w http.ResponseWriter, r *http.Request) {
		bucket := chi.URLParam(r, "bucket")
		ct := r.Header.Get("content-type")
		buf := new(bytes.Buffer)
		buf.ReadFrom(r.Body)
		bodyStr := buf.String()
		data, err := lib.ParseAnalytics(bodyStr, bucket)
		if err != nil {
			log.Errorw("error seen during analytics parse", zap.Error(err), "content-type", ct, "user-agent", r.UserAgent(), "bodyJson", bodyStr)
			http.Error(w, "processing error", 500)
			return
		}

		// Log the report.
		log.Infow("analytics recieved", "content-type", ct, "bucket", bucket, "user-agent", r.UserAgent(), "analytics", data)
		if err := lib.WriteAnalyticsToBigQuery(r.Context(), *project, *dataset, *aTable, []*lib.WebVital{data}); err != nil {
			log.Errorw("error during analytics upload", "dataset", *dataset, "project", *project, "table", *aTable, "bodyJson", bodyStr, zap.Error(err))
			http.Error(w, "uploading error", 500)
			return
		}
	})

	log.Fatal(http.ListenAndServe(":"+port, r))
}
