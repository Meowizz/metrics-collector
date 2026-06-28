package main

import (
	"fmt"
	"log"
	"time"

	"github.com/Meowizz/metrics-collector/internal/collector"
	"github.com/Meowizz/metrics-collector/internal/sender"
)

func main() {
	c := collector.NewCollector()
	s := sender.NewSender("http://localhost:8080")

	c.Collect()

	go func() {
		for {
			c.Collect()
			fmt.Println("Метрики собраны")
			time.Sleep(2 * time.Second)
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
			time.Sleep(10 * time.Second)
		}
	}()

	select {}

}
