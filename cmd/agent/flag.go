package main

import (
	"flag"
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
	return cfg
}
