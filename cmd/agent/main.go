package main

import (
	"fmt"
	"log"
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
	s := sender.NewSender(address)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(time.Duration(cfg.PollInterval) * time.Second)
		defer ticker.Stop()

		c.Collect()
		fmt.Println("Метрики собраны")

		for {
			select {
			case <-quit:
				return
			case <-ticker.C:
				c.Collect()
				fmt.Println("Метрики собраны")
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if cfg.BatchSize > 0 {

			fmt.Printf("Запущен режим батчей (размер: %d, интервал: %dс)\n", cfg.BatchSize, cfg.ReportInterval)

			batchSender := agent.NewBatchSender(s, cfg.BatchSize, time.Duration(cfg.ReportInterval)*time.Second)
			defer batchSender.FlushAndStop()

			ticker := time.NewTicker(time.Duration(cfg.ReportInterval) * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-quit:
					return
				case <-ticker.C:
					metrics := c.GetMetrics()
					if len(metrics) > 0 {
						fmt.Printf("Добавляем %d метрик в батч...\n", len(metrics))
						batchSender.Add(metrics)
					}
				}
			}
		} else {

			fmt.Println("Запущен классический режим отправки (SendJSON)")

			ticker := time.NewTicker(time.Duration(cfg.ReportInterval) * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-quit:
					return
				case <-ticker.C:
					metrics := c.GetMetrics()
					if len(metrics) > 0 {
						fmt.Printf("Отправляем %d метрик...\n", len(metrics))
						err := s.SendJSON(metrics)
						if err != nil {
							log.Printf("Failed to send metrics: %v", err)
						} else {
							fmt.Println("Метрики успешно отправлены")
						}
					}
				}
			}
		}
	}()

	<-quit
	fmt.Println("\n Получен сигнал завершения. Корректная остановка агента..")

	wg.Wait()
	fmt.Println("Агент остановлен.")
}
