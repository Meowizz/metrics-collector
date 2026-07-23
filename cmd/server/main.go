package main

import (
	"net/http"

	"github.com/Meowizz/metrics-collector/internal/handler"
	"github.com/Meowizz/metrics-collector/internal/logger"
	appMiddleware "github.com/Meowizz/metrics-collector/internal/middleware"
	"github.com/Meowizz/metrics-collector/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

func main() {
	storage := repository.NewMemStorage()
	handler := handler.NewMetricsHandler(storage)
	cfg := ParseFlag()

	if err := logger.Initialize(cfg.LogLevel); err != nil {
		panic("Failed to initialized logger: " + err.Error())
	}

	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(logger.WithLogging)

	r.Post("/update/{type}/{name}/{value}", handler.UpdatePage)
	r.Get("/value/{type}/{name}", handler.GetMetricValue)
	r.With(appMiddleware.GzipMiddleware).Post("/update", handler.UpdateMetricJSON)
	r.With(appMiddleware.GzipMiddleware).Post("/value", handler.ValueMetricJSON)
	r.With(appMiddleware.GzipMiddleware).Post("/update/", handler.UpdateMetricJSON)
	r.With(appMiddleware.GzipMiddleware).Post("/value/", handler.ValueMetricJSON)

	logger.Log.Info("Starting server", zap.String("address", cfg.Addr))

	err := http.ListenAndServe(cfg.Addr, r)

	if err != nil {
		logger.Log.Fatal("Server failed", zap.Error(err))
	}
}
