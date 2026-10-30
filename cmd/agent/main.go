package main

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Meowizz/metrics-collector/internal/agent"
	"github.com/Meowizz/metrics-collector/internal/collector"
	"github.com/Meowizz/metrics-collector/internal/logger"
	"github.com/Meowizz/metrics-collector/internal/sender"
	"github.com/Meowizz/metrics-collector/pkg/build"
)

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func main() {
	build.PrintBuildInfo(os.Stdout, buildVersion, buildDate, buildCommit)

	cfg := ParseFlag()

	if err := logger.Initialize("info"); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	c := collector.NewCollector()

	address := "http://" + cfg.Addr
	s := sender.NewSender(address, cfg.Key)

	wp := agent.NewWorkerPool(s, cfg.RateLimiter, logger.Log)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	stop := make(chan struct{})
	var wg sync.WaitGroup

	wp.StartWorkingPool()

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(time.Duration(cfg.PollInterval) * time.Second)
		defer ticker.Stop()

		c.Collect()
		c.CollectGopsutil()
		logger.Log.Info("Метрики собраны")

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				c.Collect()
				c.CollectGopsutil()
				logger.Log.Info("Метрики собраны")
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(time.Duration(cfg.ReportInterval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				metrics := c.GetMetrics()
				if len(metrics) > 0 {
					if cfg.BatchSize > 0 {
						for i := 0; i < len(metrics); i += cfg.BatchSize {
							end := i + cfg.BatchSize
							if end > len(metrics) {
								end = len(metrics)
							}
							chunk := metrics[i:end]
							wp.Ingest(chunk)
						}
					} else {
						wp.Ingest(metrics)
					}
				}
			}
		}
	}()

	<-quit
	logger.Log.Warn("Получен сигнал завершения. Корректная остановка агента..")

	close(stop)

	done := make(chan struct{})

	go func() {
		wg.Wait()
		wp.Close()
		close(done)
	}()

	select {
	case <-done:
		logger.Log.Info("Агент остановлен.")
	case <-time.After(30 * time.Second):
		logger.Log.Warn("Таймаут при остановке агента (30с). Принудительный выход.")

	}

}
