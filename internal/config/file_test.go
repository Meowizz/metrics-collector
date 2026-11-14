package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSeconds(t *testing.T) {
	tests := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{in: "1s", want: 1},
		{in: "0s", want: 0},
		{in: "5m", want: 300},
		{in: "1m30s", want: 90},
		{in: "500ms", wantErr: true},
		{in: "-1s", wantErr: true},
		{in: "10", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := Seconds(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Seconds(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Seconds(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestLoadJSON(t *testing.T) {
	dir := t.TempDir()

	var v struct {
		Address *string `json:"address"`
	}

	good := filepath.Join(dir, "good.json")
	if err := os.WriteFile(good, []byte(`{"address":"host:1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := LoadJSON(good, &v); err != nil {
		t.Fatalf("LoadJSON: %v", err)
	}
	if v.Address == nil || *v.Address != "host:1" {
		t.Errorf("address = %v, want host:1", v.Address)
	}

	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"address": // комментарий
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := LoadJSON(bad, &v); err == nil {
		t.Error("ожидалась ошибка разбора невалидного JSON")
	}

	if err := LoadJSON(filepath.Join(dir, "missing.json"), &v); err == nil {
		t.Error("ожидалась ошибка для отсутствующего файла")
	}
}
