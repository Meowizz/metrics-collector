package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/Meowizz/metrics-collector/internal/config"
)

// Config — конфигурация агента. Приоритет: дефолты < файл < флаги < env.
type Config struct {
	Addr           string
	ReportInterval int
	PollInterval   int
	BatchSize      int
	Key            string
	RateLimiter    int
	CryptoKey      string
}

type fileConfig struct {
	Address        *string `json:"address"`
	ReportInterval *string `json:"report_interval"`
	PollInterval   *string `json:"poll_interval"`
	BatchSize      *int    `json:"batch_size"`
	Key            *string `json:"key"`
	RateLimit      *int    `json:"rate_limit"`
	CryptoKey      *string `json:"crypto_key"`
}

func ParseFlag() *Config {
	cfg, err := parseConfig(os.Args[0], os.Args[1:], envMap(os.Environ()))
	if err != nil {
		log.Fatal(err)
	}
	return cfg
}

func parseConfig(name string, args []string, environ map[string]string) (*Config, error) {
	cfg := &Config{
		Addr:           "localhost:8080",
		ReportInterval: 10,
		PollInterval:   2,
		RateLimiter:    10,
	}
	var configPath string

	fs := flag.NewFlagSet(name, flag.ExitOnError)
	fs.StringVar(&cfg.Addr, "a", cfg.Addr, "Net address host:port")
	fs.IntVar(&cfg.ReportInterval, "r", cfg.ReportInterval, "report interval in seconds")
	fs.IntVar(&cfg.PollInterval, "p", cfg.PollInterval, "poll interval in seconds")
	fs.IntVar(&cfg.BatchSize, "b", cfg.BatchSize, "batch size for sending metrics (0 = disabled)")
	fs.StringVar(&cfg.Key, "k", cfg.Key, "super secret key")
	fs.IntVar(&cfg.RateLimiter, "l", cfg.RateLimiter, "rate limiter for worker pool")
	fs.StringVar(&cfg.CryptoKey, "crypto-key", cfg.CryptoKey, "path to public key file (PEM)")
	fs.StringVar(&configPath, "c", "", "path to JSON config file")
	fs.StringVar(&configPath, "config", "", "path to JSON config file (same as -c)")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if v := environ["CONFIG"]; v != "" {
		configPath = v
	}
	if configPath != "" {
		// Файл пишет поверх флагов, поэтому явно заданные флаги применяем заново.
		setFlags := map[string]string{}
		fs.Visit(func(f *flag.Flag) { setFlags[f.Name] = f.Value.String() })

		if err := applyFile(cfg, configPath); err != nil {
			return nil, err
		}
		for name, value := range setFlags {
			if err := fs.Set(name, value); err != nil {
				return nil, err
			}
		}
	}

	if err := applyEnv(cfg, environ); err != nil {
		return nil, err
	}

	cfg.Key = strings.TrimSpace(cfg.Key)
	if strings.HasPrefix(cfg.Key, "@") {
		filePath := strings.TrimSpace(strings.TrimPrefix(cfg.Key, "@"))
		keyBytes, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("error read key from file %s: %w", filePath, err)
		}
		cfg.Key = strings.TrimSpace(string(keyBytes))
	}

	return cfg, nil
}

func applyFile(cfg *Config, path string) error {
	var fc fileConfig
	if err := config.LoadJSON(path, &fc); err != nil {
		return err
	}

	if fc.Address != nil {
		cfg.Addr = *fc.Address
	}
	if fc.ReportInterval != nil {
		sec, err := config.Seconds(*fc.ReportInterval)
		if err != nil {
			return fmt.Errorf("report_interval в %s: %w", path, err)
		}
		cfg.ReportInterval = sec
	}
	if fc.PollInterval != nil {
		sec, err := config.Seconds(*fc.PollInterval)
		if err != nil {
			return fmt.Errorf("poll_interval в %s: %w", path, err)
		}
		cfg.PollInterval = sec
	}
	if fc.BatchSize != nil {
		cfg.BatchSize = *fc.BatchSize
	}
	if fc.Key != nil {
		cfg.Key = *fc.Key
	}
	if fc.RateLimit != nil {
		cfg.RateLimiter = *fc.RateLimit
	}
	if fc.CryptoKey != nil {
		cfg.CryptoKey = *fc.CryptoKey
	}
	return nil
}

func applyEnv(cfg *Config, environ map[string]string) error {
	if v := environ["ADDRESS"]; v != "" {
		cfg.Addr = v
	}
	for name, dst := range map[string]*int{
		"REPORT_INTERVAL": &cfg.ReportInterval,
		"POLL_INTERVAL":   &cfg.PollInterval,
		"BATCH_SIZE":      &cfg.BatchSize,
		"RATE_LIMIT":      &cfg.RateLimiter,
	} {
		v := environ[name]
		if v == "" {
			continue
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("unknown parameter in env:%s: %w", name, err)
		}
		*dst = n
	}
	if v := environ["KEY"]; v != "" {
		cfg.Key = v
	}
	if v := environ["CRYPTO_KEY"]; v != "" {
		cfg.CryptoKey = v
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
