package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/caarlos0/env/v6"

	"github.com/Meowizz/metrics-collector/internal/config"
)

// ConfigAddr — конфигурация сервера. Приоритет: дефолты < файл < env < флаги.
type ConfigAddr struct {
	Addr            string `env:"ADDRESS"`
	LogLevel        string `env:"LOG_LEVEL"`
	StoreInterval   int    `env:"STORE_INTERVAL"`
	FileStoragePath string `env:"FILE_STORAGE_PATH"`
	Restore         bool   `env:"RESTORE"`
	DatabaseDSN     string `env:"DATABASE_DSN"`
	Key             string `env:"KEY"`
	AuditFile       string `env:"AUDIT_FILE"`
	AuditURL        string `env:"AUDIT_URL"`
	CryptoKey       string `env:"CRYPTO_KEY"`
}

type fileConfig struct {
	Address       *string `json:"address"`
	LogLevel      *string `json:"log_level"`
	StoreInterval *string `json:"store_interval"`
	StoreFile     *string `json:"store_file"`
	Restore       *bool   `json:"restore"`
	DatabaseDSN   *string `json:"database_dsn"`
	Key           *string `json:"key"`
	AuditFile     *string `json:"audit_file"`
	AuditURL      *string `json:"audit_url"`
	CryptoKey     *string `json:"crypto_key"`
}

func defaultConfig() *ConfigAddr {
	return &ConfigAddr{
		Addr:            "localhost:8080",
		LogLevel:        "info",
		StoreInterval:   300,
		FileStoragePath: "./home/storage.json",
	}
}

func ParseFlag() *ConfigAddr {
	cfg, err := parseConfig(os.Args[0], os.Args[1:], envMap(os.Environ()))
	if err != nil {
		log.Fatal(err)
	}
	return cfg
}

func parseConfig(name string, args []string, environ map[string]string) (*ConfigAddr, error) {
	var (
		flagAddr, flagLogLevel, flagFileStoragePath, flagDatabaseDSN, flagKey, flagAuditFile, flagAuditURL, flagCryptoKey, flagConfig string
		flagStoreInterval                                                                                                             int
		flagRestore                                                                                                                   bool
	)

	fs := flag.NewFlagSet(name, flag.ExitOnError)
	fs.StringVar(&flagAddr, "a", "", "Net address host:port")
	fs.StringVar(&flagLogLevel, "l", "", "Log level")
	fs.IntVar(&flagStoreInterval, "i", 0, "The -i flag and the STORE_INTERVAL environment variable are the time interval in seconds during which the server maintains a current connection to disk ")
	fs.StringVar(&flagFileStoragePath, "f", "", "The -f flag and the FILE_STORAGE_PATH environment variable are the path to the file where the current value is stored.")
	fs.BoolVar(&flagRestore, "r", false, "The -r flag and the RESTORE environment variable are a Boolean value (true/false) that determines whether previously saved values should be loaded from the specified file when the server starts.")
	fs.StringVar(&flagDatabaseDSN, "d", "", "Database connection setting")
	fs.StringVar(&flagKey, "k", "", "Super Secret Key")
	fs.StringVar(&flagAuditFile, "audit-file", "", "Audit name file")
	fs.StringVar(&flagAuditURL, "audit-url", "", "URL for audit")
	fs.StringVar(&flagCryptoKey, "crypto-key", "", "path to private key file (PEM)")
	fs.StringVar(&flagConfig, "c", "", "path to JSON config file")
	fs.StringVar(&flagConfig, "config", "", "path to JSON config file (same as -c)")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	cfg := defaultConfig()

	configPath := flagConfig
	if configPath == "" {
		configPath = environ["CONFIG"]
	}
	if configPath != "" {
		if err := applyFile(cfg, configPath); err != nil {
			return nil, err
		}
	}

	if err := env.Parse(cfg, env.Options{Environment: environ}); err != nil {
		return nil, fmt.Errorf("error parsing environment variables: %w", err)
	}

	if flagAddr != "" {
		cfg.Addr = flagAddr
	}
	if flagLogLevel != "" {
		cfg.LogLevel = flagLogLevel
	}
	setFlags := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) { setFlags[f.Name] = true })
	if setFlags["i"] {
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
	if flagCryptoKey != "" {
		cfg.CryptoKey = flagCryptoKey
	}
	if flagAuditFile != "" {
		cfg.AuditFile = flagAuditFile
	}
	if flagAuditURL != "" {
		cfg.AuditURL = flagAuditURL
	}

	cfg.Key = strings.TrimSpace(cfg.Key)
	if strings.HasPrefix(cfg.Key, "@") {
		filePath := strings.TrimPrefix(cfg.Key, "@")
		keyBytes, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("yе удалось прочитать файл ключа %s: %w", filePath, err)
		}
		cfg.Key = strings.TrimSpace(string(keyBytes))
	}

	return cfg, nil
}

func applyFile(cfg *ConfigAddr, path string) error {
	var fc fileConfig
	if err := config.LoadJSON(path, &fc); err != nil {
		return err
	}

	if fc.Address != nil {
		cfg.Addr = *fc.Address
	}
	if fc.LogLevel != nil {
		cfg.LogLevel = *fc.LogLevel
	}
	if fc.StoreInterval != nil {
		sec, err := config.Seconds(*fc.StoreInterval)
		if err != nil {
			return fmt.Errorf("store_interval в %s: %w", path, err)
		}
		cfg.StoreInterval = sec
	}
	if fc.StoreFile != nil {
		cfg.FileStoragePath = *fc.StoreFile
	}
	if fc.Restore != nil {
		cfg.Restore = *fc.Restore
	}
	if fc.DatabaseDSN != nil {
		cfg.DatabaseDSN = *fc.DatabaseDSN
	}
	if fc.Key != nil {
		cfg.Key = *fc.Key
	}
	if fc.AuditFile != nil {
		cfg.AuditFile = *fc.AuditFile
	}
	if fc.AuditURL != nil {
		cfg.AuditURL = *fc.AuditURL
	}
	if fc.CryptoKey != nil {
		cfg.CryptoKey = *fc.CryptoKey
	}
	return nil
}

func envMap(environ []string) map[string]string {
	m := make(map[string]string, len(environ))
	for _, kv := range environ {
		if k, v, ok := strings.Cut(kv, "="); ok {
			m[k] = v
		}
	}
	return m
}
