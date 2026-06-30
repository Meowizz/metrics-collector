package main

import (
	"net/http"

	"github.com/Meowizz/metrics-collector/internal/handler"
	"github.com/Meowizz/metrics-collector/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	storage := repository.NewMemStorage()
	handler := handler.NewMetricsHandler(storage)

	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Post("/update/{type}/{name}/{value}", handler.UpdatePage)
	r.Get("/value/{type}/{name}", handler.GetMetricValue)

	err := http.ListenAndServe(":8080", r)

	if err != nil {
		panic(err)
	}
}
