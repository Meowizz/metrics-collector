package main

import (
	"context"
	"crypto/rsa"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Meowizz/metrics-collector/internal/audit"
	"github.com/Meowizz/metrics-collector/internal/crypto"
	"github.com/Meowizz/metrics-collector/internal/handler"
	"github.com/Meowizz/metrics-collector/internal/logger"
	appMiddleware "github.com/Meowizz/metrics-collector/internal/middleware"
	"github.com/Meowizz/metrics-collector/internal/repository"
	"github.com/Meowizz/metrics-collector/pkg/build"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func main() {
	//storage := repository.NewMemStorage()
	//handler := handler.NewMetricsHandler(storage)

	build.PrintBuildInfo(os.Stdout, buildVersion, buildDate, buildCommit)

	cfg := ParseFlag()

	auditSubj := audit.NewSubject()

	if err := logger.Initialize(cfg.LogLevel); err != nil {
		panic("Failed to initialized logger: " + err.Error())
	}

	// Регистрируется первым, чтобы выполниться после всех остальных defer.
	var runErr error
	defer func() {
		if runErr != nil {
			logger.Log.Fatal("Server stopped due to error")
		}
	}()

	var storage repository.Storage

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

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
	var privateKey *rsa.PrivateKey
	if cfg.CryptoKey != "" {
		var err error
		privateKey, err = crypto.LoadPrivateKey(cfg.CryptoKey)
		if err != nil {
			log.Fatalf("failed to load private key: %v", err)
		}

	}
	if cfg.AuditFile != "" {
		fileObs, err := audit.NewFileAuditObserver(cfg.AuditFile, logger.Log)
		if err != nil {
			log.Fatal("Can`t work with file [AUDIT]")
		}
		auditSubj.Attach(fileObs)
		defer fileObs.Close()
	}

	if cfg.AuditURL != "" {
		httpObs := audit.NewHTTPObserver(cfg.AuditURL, logger.Log)
		auditSubj.Attach(httpObs)
	}

	handler := handler.NewMetricsHandler(storage, auditSubj)

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

	var saverWG sync.WaitGroup
	if cfg.StoreInterval > 0 {
		if memStore, ok := storage.(*repository.MemStorage); ok {
			saverWG.Add(1)
			go func() {
				defer saverWG.Done()
				ticker := time.NewTicker(time.Duration(cfg.StoreInterval) * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ctx.Done():
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
	hashMW := func(next http.Handler) http.Handler {
		return appMiddleware.HashMiddleware(next, cfg.Key)
	}
	r.Use(middleware.Recoverer)
	r.Use(logger.WithLogging)
	r.Use(appMiddleware.DecryptMiddleware(privateKey))

	r.With(appMiddleware.GzipMiddleware, hashMW).Get("/", handler.MainPage)
	r.With(hashMW).Post("/update/{type}/{name}/{value}", handler.UpdatePage)
	r.With(hashMW).Get("/value/{type}/{name}", handler.GetMetricValue)
	r.With(appMiddleware.GzipMiddleware, hashMW).Post("/update", handler.UpdateMetricJSON)
	r.With(appMiddleware.GzipMiddleware, hashMW).Post("/value", handler.ValueMetricJSON)
	r.With(appMiddleware.GzipMiddleware, hashMW).Post("/update/", handler.UpdateMetricJSON)
	r.With(appMiddleware.GzipMiddleware, hashMW).Post("/value/", handler.ValueMetricJSON)
	r.With(appMiddleware.GzipMiddleware, hashMW).Post("/updates/", handler.UpdatesHandler)
	r.Get("/ping", handler.Ping)

	logger.Log.Info("Starting server", zap.String("address", cfg.Addr))

	server := &http.Server{
		Addr:    cfg.Addr,
		Handler: r,
	}

	serverErr := make(chan error, 1)

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Log.Info("Shutting down server")
	case runErr = <-serverErr:
		logger.Log.Error("Server exited with error", zap.Error(runErr))

	}
	stop()

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		logger.Log.Error("Server shutdown failed", zap.Error(err))
	}
	auditSubj.Shutdown()
	saverWG.Wait()
	if memStore, ok := storage.(*repository.MemStorage); ok && cfg.FileStoragePath != "" {
		if err := memStore.SaveToFile(cfg.FileStoragePath); err != nil {
			logger.Log.Error("Failed to save metrics on shutdown", zap.Error(err))
		} else {
			logger.Log.Info("Metrics saved on shutdown", zap.String("path", cfg.FileStoragePath))
		}
	}
	logger.Log.Info("Server stopped")
}
