package main

import (
	"context"
	"log"
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
	//storage := repository.NewMemStorage()
	//handler := handler.NewMetricsHandler(storage)
	cfg := ParseFlag()

	if err := logger.Initialize(cfg.LogLevel); err != nil {
		panic("Failed to initialized logger: " + err.Error())
	}

	var storage repository.Storage

	ctx := context.Background()

	if cfg.DatabaseDSN != "" {

		if err := repository.RunMigrations(cfg.DatabaseDSN, "./migrations"); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}

		log.Print("Initializing PostgreSQL storage")
		pgStore, err := repository.NewPostgresStorage(cfg.DatabaseDSN)
		if err != nil {
			log.Fatalf("Failed to connect to PostgreSQL %v", err)
		}
		storage = pgStore
		defer pgStore.Close()
	} else {
		log.Println("Using in-memory storage")
		storage = repository.NewMemStorage()
	}
	handler := handler.NewMetricsHandler(storage)

	if cfg.Restore {
		if memStore, ok := storage.(*repository.MemStorage); ok {
			if err := memStore.LoadFromFile(cfg.FileStoragePath); err != nil {
				logger.Log.Error("Ошибка при загрузке из файла", zap.Error(err))
			} else {
				logger.Log.Info("Метрики успешно загружены из файла")
			}
		} else {
			logger.Log.Info("Восстановление из файла пропущено: используется PostgreSQL")
		}
	}

	if cfg.StoreInterval > 0 {
		if memStore, ok := storage.(*repository.MemStorage); ok {
			go func() {
				ticker := time.NewTicker(time.Duration(cfg.StoreInterval) * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ctx.Done():
						if err := memStore.SaveToFile(cfg.FileStoragePath); err != nil {
							logger.Log.Error("Ошибка финального сохранения при остановке", zap.Error(err))
						} else {
							logger.Log.Info("Метрики успешно сохранены перед остановкой")
						}
						logger.Log.Info("Горутина фонового сохранения завершена")
						return

					case <-ticker.C:
						if err := memStore.SaveToFile(cfg.FileStoragePath); err != nil {
							logger.Log.Error("Ошибка фонового сохранения", zap.Error(err))
						}
					}
				}
			}()
		}
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
	r.With(appMiddleware.GzipMiddleware).Post("/updates/", handler.UpdatesHandler)
	r.Get("/ping", handler.Ping)

	logger.Log.Info("Starting server", zap.String("address", cfg.Addr))

	err := http.ListenAndServe(cfg.Addr, r)

	if err != nil {
		logger.Log.Fatal("Server failed", zap.Error(err))
	}
}
