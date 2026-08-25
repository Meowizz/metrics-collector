package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Meowizz/metrics-collector/internal/agent"
	"github.com/Meowizz/metrics-collector/internal/collector"
	"github.com/Meowizz/metrics-collector/internal/sender"
)

func main() {
	cfg := ParseFlag()
	c := collector.NewCollector()

	address := "http://" + cfg.Addr
	s := sender.NewSender(address, cfg.Key)

	wp := agent.NewWorkerPool(s, cfg.RateLimiter)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	stop := make(chan struct{})
	var wg sync.WaitGroup

	wp.StartWorkingPool(&wg)

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(time.Duration(cfg.PollInterval) * time.Second)
		defer ticker.Stop()

		c.Collect()
		c.CollectGopsutil()
		fmt.Println("Метрики собраны")

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				c.Collect()
				c.CollectGopsutil()
				fmt.Println("Метрики собраны")
			}
		}
	}()

	wg.Add(1)
	go func() {
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
	fmt.Println("\n Получен сигнал завершения. Корректная остановка агента..")

	close(stop)

	time.Sleep(100 * time.Millisecond)

	wp.Close()

	done := make(chan struct{})

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("Агент остановлен.")
	case <-time.After(30 * time.Second):
		fmt.Println("Таймаут при остановке агента (30с). Принудительный выход.")

	}

}
