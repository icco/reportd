package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
	"github.com/icco/gutil/logging"
	"github.com/icco/reportd/lib"
	"github.com/namsral/flag"
	"go.uber.org/zap"
)

var (
	service = "reportd"
	log     = logging.Must(logging.NewLogger(service))
	project = flag.String("project", os.Getenv("PROJECT_ID"), "Project ID containing the bigquery dataset to upload to.")
	dataset = flag.String("dataset", os.Getenv("DATASET"), "The bigquery dataset to upload to.")
	aTable  = flag.String("analytics_table", os.Getenv("ANALYTICS_TABLE"), "The bigquery table to upload analytics to.")
	rTable  = flag.String("reports_table", os.Getenv("REPORTS_TABLE"), "The bigquery table to upload reports to.")
)

func main() {
	flag.Parse()

	port := "8080"
	if fromEnv := os.Getenv("PORT"); fromEnv != "" {
		port = fromEnv
	}
	log.Infow("Starting up", "host", fmt.Sprintf("http://localhost:%s", port))

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
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`hi! Please see <a href="https://github.com/icco/reportd">github.com/icco/reportd</a> for more information.`))
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

	r.Post("/report/{bucket}", func(w http.ResponseWriter, r *http.Request) {
		bucket := chi.URLParam(r, "bucket")

		buf := new(bytes.Buffer)
		buf.ReadFrom(r.Body)
		bodyStr := buf.String()
		ct := r.Header.Get("content-type")
		data, err := lib.ParseReport(ct, bodyStr)
		if err != nil {
			log.Errorw("error seen during parse", "content-type", ct, "user-agent", r.UserAgent(), "json", bodyStr, zap.Error(err))
			http.Error(w, "processing error", 500)
			return
		}

		// Log the report.
		log.Infow("report recieved", "content-type", ct, "bucket", bucket, "user-agent", r.UserAgent(), "report", data)

		if err := lib.WriteReportToBigQuery(r.Context(), *project, *dataset, *rTable, []*lib.Report{data}); err != nil {
			log.Errorw("error during upload", "dataset", *dataset, "project", *project, "table", *rTable, zap.Error(err))
			http.Error(w, "uploading error", 500)
			return
		}
	})

	r.Post("/analytics/{bucket}", func(w http.ResponseWriter, r *http.Request) {
		bucket := chi.URLParam(r, "bucket")
		ct := r.Header.Get("content-type")
		data, err := lib.ParseAnalytics(r.Body)
		if err != nil {
			log.Errorw("error seen during parse", zap.Error(err), "content-type", ct, "user-agent", r.UserAgent())
			http.Error(w, "processing error", 500)
			return
		}

		// Log the report.
		log.Infow("analytics recieved", "content-type", ct, "bucket", bucket, "user-agent", r.UserAgent(), "analytics", data)
		if err := lib.WriteAnalyticsToBigQuery(r.Context(), *project, *dataset, *aTable, []*lib.WebVital{data}); err != nil {
			log.Errorw("error during upload", "dataset", *dataset, "project", *project, "table", *aTable, zap.Error(err))
			http.Error(w, "uploading error", 500)
			return
		}
	})

	log.Fatal(http.ListenAndServe(":"+port, r))
}
