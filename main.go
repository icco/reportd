package main

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
	sdLogging "github.com/icco/logrus-stackdriver-formatter"
)

var (
	log = InitLogging()
)

func main() {
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

	r.Post("/report/{bucket}", func(w http.ResponseWriter, r *http.Request) {
		bucket := chi.URLParam(r, "bucket")

		var data []map[string]string
		decoder := json.NewDecoder(r.Body)
		err = decoder.Decode(&data)
		if err != nil {
			log.WithError(err).Error("Error seen during json decode")
			Renderer.JSON(w, 500, map[string]string{"error": err.Error()})
			return
		}

		log.WithFields(logrus.Fields{"bucket": bucket, "data": data}).Info("report")
	})
}
