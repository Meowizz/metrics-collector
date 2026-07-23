package main

import (
	"flag"
	"log"
	"os"

	"github.com/caarlos0/env/v6"
)

type ConfigAddr struct {
	Addr            string `env:"ADDRESS" envDefault:"localhost:8080"`
	LogLevel        string `env:"LOG_LEVEL" envDefault:"info"`
	StoreInterval   int    `env:"STORE_INTERVAL" envDefault:"300"`
	FileStoragePath string `env:"FILE_STORAGE_PATH" envDefault:"./home/storage.json"`
	Restore         bool   `env:"RESTORE" envDefault:"false"`
}

func ParseFlag() *ConfigAddr {
	cfg := &ConfigAddr{}

	if err := env.Parse(cfg); err != nil {
		log.Fatalf("Error parsing environment variables: %v", err)
	}

	var flagAddr, flagLogLevel string
	var flagStoreInterval int
	var flagFileStoragePath string
	var flagRestore bool

	flag.StringVar(&flagAddr, "a", "", "Net address host:port")
	flag.StringVar(&flagLogLevel, "l", "", "Log level")
	flag.IntVar(&flagStoreInterval, "i", 0, "The -i flag and the STORE_INTERVAL environment variable are the time interval in seconds during which the server maintains a current connection to disk ")
	flag.StringVar(&flagFileStoragePath, "f", "", "The -f flag and the FILE_STORAGE_PATH environment variable are the path to the file where the current value is stored.")
	flag.BoolVar(&flagRestore,"r",false,"The -r flag and the RESTORE environment variable are a Boolean value (true/false) that determines whether previously saved values ​​should be loaded from the specified file when the server starts.")

	flag.Parse()

	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "a":
			if os.Getenv("ADDRESS") == "" {
				cfg.Addr = flagAddr
			}
		case "l":
			if os.Getenv("LOG_LEVEL") == "" {
				cfg.LogLevel = flagLogLevel
			}
		case "i":
			if os.Getenv("STORE_INTERVAL") == "" {
				cfg.StoreInterval = flagStoreInterval
			}
		case "f":
			if os.Getenv("FILE_STORAGE_PATH") == "" {
				cfg.FileStoragePath = flagFileStoragePath
			}
		case "r":
			if os.Getenv("RESTORE") == "" {
				cfg.Restore = flagRestore
			}
		}
	})

	return cfg
}
