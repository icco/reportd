package main

import (
	"bytes"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
	sdLogging "github.com/icco/logrus-stackdriver-formatter"
	"github.com/icco/reportd/lib"
	"github.com/sirupsen/logrus"
)

var (
	log = InitLogging()
)

func main() {
	port := "8080"
	if fromEnv := os.Getenv("PORT"); fromEnv != "" {
		port = fromEnv
	}
	log.Printf("Starting up on http://localhost:%s", port)

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(sdLogging.LoggingMiddleware(log))
	r.Use(middleware.Recoverer)

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
		w.Write([]byte("hi"))
	})

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok."))
	})

	r.Options("/report/{bucket}", func(w http.ResponseWriter, r *http.Request) {
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
			log.WithError(err).WithFields(logrus.Fields{"content-type": ct, "user-agent": r.UserAgent(), "json": bodyStr}).Error("error seen during parse")
			http.Error(w, "processing error", 500)
			return
		}
		log.WithFields(logrus.Fields{"bucket": bucket, "data": data}).Info("report")
	})

	log.Fatal(http.ListenAndServe(":"+port, r))
}
