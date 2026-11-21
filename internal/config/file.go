// Package config содержит общие помощники для загрузки конфигурации приложений.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// LoadJSON читает файл конфигурации path и декодирует его JSON-содержимое в v.
func LoadJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("чтение файла конфигурации %s: %w", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("разбор файла конфигурации %s: %w", path, err)
	}
	return nil
}

// Seconds переводит длительность вида "1s" в целые секунды.
func Seconds(s string) (int, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("некорректная длительность %q: %w", s, err)
	}
	if d < 0 {
		return 0, fmt.Errorf("отрицательная длительность %q", s)
	}
	if d%time.Second != 0 {
		return 0, fmt.Errorf("длительность %q должна быть кратна секунде", s)
	}
	return int(d / time.Second), nil
}
