package main

import (
	"net/http"
	"time"

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

	if cfg.Restore {
		if err := storage.LoadFromFile(cfg.FileStoragePath); err != nil {
			logger.Log.Error("Ошибка фонового сохранения", zap.Error(err))
		}
	}

	if cfg.StoreInterval > 0 {
		go func() {
			for {
				time.Sleep(time.Duration(cfg.StoreInterval) * time.Second)

				if err := storage.SaveToFile(cfg.FileStoragePath); err != nil {
					logger.Log.Error("Ошибка фонового сохранения", zap.Error(err))
				}
			}
		}()
	}

	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(logger.WithLogging)

	r.With(appMiddleware.GzipMiddleware).Get("/", handler.MainPage)
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
