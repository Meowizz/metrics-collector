package main

import (
	"flag"
	"log"
	"os"

	"github.com/caarlos0/env/v6"
)

type ConfigAddr struct {
	Addr string `env:"ADDRESS" envDefault:"localhost:8080"`
}

func ParseFlag() *ConfigAddr {
	var flagAddr string
	cfg := &ConfigAddr{}
	err := env.Parse(cfg)

	if err != nil {
		log.Fatal("Error parsing server argumnet for start")
	}

	flag.StringVar(&flagAddr, "a", "", "Net address host:port")
	flag.Parse()

	if os.Getenv("ADDRESS") == "" && flagAddr != "" {
		cfg.Addr = flagAddr
	}

	return cfg
}
