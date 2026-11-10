package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "server.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

const fullServerConfig = `{
	"address": "file:1",
	"restore": true,
	"store_interval": "1s",
	"store_file": "/file/store.db",
	"database_dsn": "file-dsn",
	"crypto_key": "/file/key.pem"
}`

func TestParseConfigDefaults(t *testing.T) {
	cfg, err := parseConfig("server", nil, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	want := *defaultConfig()
	if *cfg != want {
		t.Errorf("cfg = %+v, want %+v", *cfg, want)
	}
}

func TestParseConfigFromFile(t *testing.T) {
	path := writeConfig(t, fullServerConfig)

	for _, tc := range []struct {
		name    string
		args    []string
		environ map[string]string
	}{
		{name: "flag -c", args: []string{"-c", path}, environ: map[string]string{}},
		{name: "flag -config", args: []string{"-config", path}, environ: map[string]string{}},
		{name: "env CONFIG", environ: map[string]string{"CONFIG": path}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := parseConfig("server", tc.args, tc.environ)
			if err != nil {
				t.Fatal(err)
			}
			want := ConfigAddr{
				Addr:            "file:1",
				LogLevel:        "info",
				StoreInterval:   1,
				FileStoragePath: "/file/store.db",
				Restore:         true,
				DatabaseDSN:     "file-dsn",
				CryptoKey:       "/file/key.pem",
			}
			if *cfg != want {
				t.Errorf("cfg = %+v, want %+v", *cfg, want)
			}
		})
	}
}

func TestParseConfigPriority(t *testing.T) {
	path := writeConfig(t, fullServerConfig)

	// Файл < env < флаг; незаданные в env и флагах поля остаются из файла.
	cfg, err := parseConfig("server",
		[]string{"-c", path, "-a", "flag:3", "-i", "7"},
		map[string]string{
			"ADDRESS":        "env:2",
			"STORE_INTERVAL": "5",
			"DATABASE_DSN":   "env-dsn",
		})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Addr != "flag:3" {
		t.Errorf("Addr = %q, want flag:3", cfg.Addr)
	}
	if cfg.StoreInterval != 7 {
		t.Errorf("StoreInterval = %d, want 7", cfg.StoreInterval)
	}
	if cfg.DatabaseDSN != "env-dsn" {
		t.Errorf("DatabaseDSN = %q, want env-dsn", cfg.DatabaseDSN)
	}
	if cfg.FileStoragePath != "/file/store.db" || !cfg.Restore || cfg.CryptoKey != "/file/key.pem" {
		t.Errorf("значения из файла потеряны: %+v", *cfg)
	}
}

func TestParseConfigEnvOverridesFileFalse(t *testing.T) {
	path := writeConfig(t, `{"restore": true}`)
	cfg, err := parseConfig("server", nil, map[string]string{"CONFIG": path, "RESTORE": "false"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Restore {
		t.Error("RESTORE=false из env должен перекрывать restore=true из файла")
	}
}

func TestParseConfigFlagOverridesEnvConfigPath(t *testing.T) {
	fromEnv := writeConfig(t, `{"address": "env-file:1"}`)
	fromFlag := writeConfig(t, `{"address": "flag-file:1"}`)
	cfg, err := parseConfig("server", []string{"-c", fromFlag}, map[string]string{"CONFIG": fromEnv})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "flag-file:1" {
		t.Errorf("Addr = %q, want flag-file:1", cfg.Addr)
	}
}

func TestParseConfigFileErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
	}{
		{name: "невалидный JSON", content: `{"address": `},
		{name: "интервал без единиц", content: `{"store_interval": "10"}`},
		{name: "интервал числом", content: `{"store_interval": 10}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeConfig(t, tc.content)
			if _, err := parseConfig("server", []string{"-c", path}, map[string]string{}); err == nil {
				t.Error("ожидалась ошибка")
			}
		})
	}

	if _, err := parseConfig("server", []string{"-c", "/nonexistent/server.json"}, map[string]string{}); err == nil {
		t.Error("ожидалась ошибка для отсутствующего файла")
	}
}
