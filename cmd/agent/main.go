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
	"github.com/Meowizz/metrics-collector/internal/crypto"
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

	if cfg.CryptoKey != "" {
		publicKey, err := crypto.LoadPublicKey(cfg.CryptoKey)
		if err != nil {
			log.Fatalf("failed to load public key: %v", err)
		}
		s.EnableEncryption(publicKey)
	}

	wp := agent.NewWorkerPool(s, cfg.RateLimiter, logger.Log)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

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
				reportMetrics(c, wp, cfg.BatchSize)
				return
			case <-ticker.C:
				reportMetrics(c, wp, cfg.BatchSize)
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

// reportMetrics передаёт текущие метрики в пул воркеров, разбивая их на батчи по batchSize.
func reportMetrics(c *collector.Collector, wp *agent.WorkerPool, batchSize int) {
	metrics := c.GetMetrics()
	if len(metrics) == 0 {
		return
	}
	if batchSize <= 0 {
		wp.Ingest(metrics)
		return
	}
	for i := 0; i < len(metrics); i += batchSize {
		end := min(i+batchSize, len(metrics))
		wp.Ingest(metrics[i:end])
	}
}
