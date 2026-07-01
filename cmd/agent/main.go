package main

import (
	"fmt"
	"log"
	"time"

	"github.com/Meowizz/metrics-collector/internal/collector"
	"github.com/Meowizz/metrics-collector/internal/sender"
)

func main() {
	cfg := ParseFlag()
	c := collector.NewCollector()

	s := sender.NewSender(cfg.Addr)

	c.Collect()

	go func() {
		for {
			c.Collect()
			fmt.Println("Метрики собраны")
			time.Sleep(time.Duration(cfg.PollInterval) * time.Second)
		}
	}()

	go func() {
		for {
			metrics := c.GetMetrics()
			fmt.Printf("Отправляем %d метрик...\n", len(metrics))
			err := s.Send(metrics)
			if err != nil {
				log.Printf("Failed to send metrics:%v", err)
			} else {
				fmt.Println("Метрики отправлены")
			}
			time.Sleep(time.Duration(cfg.ReportInterval) * time.Second)
		}
	}()

	select {}

}
