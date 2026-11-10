package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agent.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

const fullAgentConfig = `{
	"address": "file:1",
	"report_interval": "1s",
	"poll_interval": "1s",
	"crypto_key": "/file/key.pem"
}`

func TestParseConfigDefaults(t *testing.T) {
	cfg, err := parseConfig("agent", nil, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	want := Config{Addr: "localhost:8080", ReportInterval: 10, PollInterval: 2, RateLimiter: 10}
	if *cfg != want {
		t.Errorf("cfg = %+v, want %+v", *cfg, want)
	}
}

func TestParseConfigFromFile(t *testing.T) {
	path := writeConfig(t, fullAgentConfig)
	want := Config{Addr: "file:1", ReportInterval: 1, PollInterval: 1, RateLimiter: 10, CryptoKey: "/file/key.pem"}

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
			cfg, err := parseConfig("agent", tc.args, tc.environ)
			if err != nil {
				t.Fatal(err)
			}
			if *cfg != want {
				t.Errorf("cfg = %+v, want %+v", *cfg, want)
			}
		})
	}
}

func TestParseConfigPriority(t *testing.T) {
	path := writeConfig(t, fullAgentConfig)

	cfg, err := parseConfig("agent",
		[]string{"-c", path, "-a", "flag:2", "-r", "5", "-p", "0"},
		map[string]string{"ADDRESS": "env:3"})
	if err != nil {
		t.Fatal(err)
	}

	want := Config{Addr: "env:3", ReportInterval: 5, PollInterval: 0, RateLimiter: 10, CryptoKey: "/file/key.pem"}
	if *cfg != want {
		t.Errorf("cfg = %+v, want %+v", *cfg, want)
	}
}

func TestParseConfigFileErrors(t *testing.T) {
	for _, content := range []string{`{"address": `, `{"poll_interval": "2"}`, `{"report_interval": 10}`} {
		path := writeConfig(t, content)
		if _, err := parseConfig("agent", []string{"-c", path}, map[string]string{}); err == nil {
			t.Errorf("%s: ожидалась ошибка", content)
		}
	}
	if _, err := parseConfig("agent", nil, map[string]string{"POLL_INTERVAL": "x"}); err == nil {
		t.Error("POLL_INTERVAL=x: ожидалась ошибка")
	}
}
