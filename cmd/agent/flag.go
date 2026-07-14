package main

import (
	"flag"
	"log"
	"os"
	"strconv"
)

type Config struct {
	Addr           string
	ReportInterval int
	PollInterval   int
}

func ParseFlag() *Config {
	cfg := &Config{
		Addr:           "localhost:8080",
		ReportInterval: 10,
		PollInterval:   2,
	}
	flag.StringVar(&cfg.Addr, "a", cfg.Addr, "Net address host:port")
	flag.IntVar(&cfg.ReportInterval, "r", cfg.ReportInterval, "report interval in seconds")
	flag.IntVar(&cfg.PollInterval, "p", cfg.PollInterval, "poll interval in seconds")

	flag.Parse()

	if envRunAddr := os.Getenv("ADDRESS"); envRunAddr != "" {
		cfg.Addr = envRunAddr
	}

	if envRunReportInterval := os.Getenv("REPORT_INTERVAL"); envRunReportInterval != "" {
		reportInterval, err := strconv.Atoi(envRunReportInterval)
		if err != nil {
			log.Fatal("Unkonwn parametr in env:REPORT_INTERVAL")
		}
		cfg.ReportInterval = reportInterval
	}

	if envPollInterval := os.Getenv("POLL_INTERVAL"); envPollInterval != "" {
		pollInterval, err := strconv.Atoi(envPollInterval)
		if err != nil {
			log.Fatal("Unkonwn parametr in env:POLL_INTERVAL")
		}
		cfg.PollInterval = pollInterval
	}
	return cfg
}
