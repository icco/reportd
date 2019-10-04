package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
	sdLogging "github.com/icco/logrus-stackdriver-formatter"
	"github.com/sirupsen/logrus"
)

var (
	log = InitLogging()
)

// {"expect-ct-report":{"date-time":"2019-10-04T01:05:38.621Z","effective-expiration-date":"2019-10-04T01:05:38.621Z","hostname":"expect-ct-report.test","port":443,"scts":[],"served-certificate-chain":[],"validated-certificate-chain":[]}}
type Report struct {
	ExpectCTReport struct {
		DateTime                  time.Time `json:"date-time"`
		EffectiveExpirationDate   time.Time `json:"effective-expiration-date"`
		Hostname                  string    `json:"hostname"`
		Port                      int       `json:"port"`
		Scts                      []string  `json:"scts"`
		ServedCertificateChain    []string  `json:"served-certificate-chain"`
		ValidatedCertificateChain []string  `json:"validated-certificate-chain"`
	} `json:"expect-ct-report"`
}

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

		// TODO: Validate application/reports+json
		var data Report

		buf := new(bytes.Buffer)
		buf.ReadFrom(r.Body)
		bodyStr := buf.String()

		err := json.Unmarshal([]byte(bodyStr), &data)
		if err != nil {
			log.WithError(err).WithField("json", bodyStr).Error("Error seen during json decode")
			http.Error(w, "processing error", 500)
			return
		}

		log.WithFields(logrus.Fields{"bucket": bucket, "data": data}).Info("report")
	})

	log.Fatal(http.ListenAndServe(":"+port, r))
}
