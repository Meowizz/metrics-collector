package main

import (
	"flag"
)

type ConfigAddr struct {
	Addr string
}

func ParseFlag() *ConfigAddr {
	cfg := &ConfigAddr{
		Addr: "localhost:8080",
	}
	flag.StringVar(&cfg.Addr, "a", cfg.Addr, "Net address host:port")

	flag.Parse()
	return cfg
}
