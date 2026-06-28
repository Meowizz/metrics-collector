package main

import (
	"log"
	"time"

	"github.com/Meowizz/metrics-collector/internal/collector"
	"github.com/Meowizz/metrics-collector/internal/sender"
)

func main() {
	c := collector.NewCollector()
	s := sender.NewSender("http://localhost:8080")

	go func() {
		for {
			c.Collect()
			time.Sleep(2 * time.Second)
		}
	}()

	go func() {
		for {
			metrics := c.GetMetrics()
			err := s.Send(metrics)
			if err != nil {
				log.Printf("Failed to send metrics:%v", err)
			}
			time.Sleep(10 * time.Second)
		}
	}()

	select {}

}
