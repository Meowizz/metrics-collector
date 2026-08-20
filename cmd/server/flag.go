package main

import (
	"flag"
	"log"

	"github.com/caarlos0/env/v6"
)

type ConfigAddr struct {
	Addr            string `env:"ADDRESS" envDefault:"localhost:8080"`
	LogLevel        string `env:"LOG_LEVEL" envDefault:"info"`
	StoreInterval   int    `env:"STORE_INTERVAL" envDefault:"300"`
	FileStoragePath string `env:"FILE_STORAGE_PATH" envDefault:"./home/storage.json"`
	Restore         bool   `env:"RESTORE" envDefault:"false"`
	DatabaseDSN     string `env:"DATABASE_DSN" envDefaut:""`
	Key             string `env:"KEY" envDefault:""`
}

func ParseFlag() *ConfigAddr {
	cfg := &ConfigAddr{}

	if err := env.Parse(cfg); err != nil {
		log.Fatalf("Error parsing environment variables: %v", err)
	}

	var (
		flagAddr, flagLogLevel, flagFileStoragePath, flagDatabaseDSN, flagKey string
		flagStoreInterval                                                     int
		flagRestore                                                           bool
	)

	flag.StringVar(&flagAddr, "a", "", "Net address host:port")
	flag.StringVar(&flagLogLevel, "l", "", "Log level")
	flag.IntVar(&flagStoreInterval, "i", 0, "The -i flag and the STORE_INTERVAL environment variable are the time interval in seconds during which the server maintains a current connection to disk ")
	flag.StringVar(&flagFileStoragePath, "f", "", "The -f flag and the FILE_STORAGE_PATH environment variable are the path to the file where the current value is stored.")
	flag.BoolVar(&flagRestore, "r", false, "The -r flag and the RESTORE environment variable are a Boolean value (true/false) that determines whether previously saved values ​​should be loaded from the specified file when the server starts.")
	flag.StringVar(&flagDatabaseDSN, "d", "", "Database connection setting")
	flag.StringVar(&flagKey, "k", "", "Super Secret Key")

	flag.Parse()

	if flagAddr != "" {
		cfg.Addr = flagAddr
	}
	if flagLogLevel != "" {
		cfg.LogLevel = flagLogLevel
	}
	if flagStoreInterval != 0 {
		cfg.StoreInterval = flagStoreInterval
	}
	if flagFileStoragePath != "" {
		cfg.FileStoragePath = flagFileStoragePath
	}

	if flagRestore {
		cfg.Restore = flagRestore
	}

	if flagDatabaseDSN != "" {
		cfg.DatabaseDSN = flagDatabaseDSN
	}

	if flagKey != "" {
		cfg.Key = flagKey
	}

	return cfg
}
