package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type MemStorage struct {
	mu      sync.RWMutex
	gauge   map[string]float64
	counter map[string]int64
}

type SavedState struct {
	Counters map[string]int64   `json:"counters"`
	Gauges   map[string]float64 `json:"gauges"`
}

type Storage interface {
	UpdateGauge(name string, value float64) error
	UpdateCounter(name string, value int64) error
	GetGauge(name string) (float64, bool)
	GetCounter(name string) (int64, bool)
	Ping() error
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauge:   make(map[string]float64),
		counter: make(map[string]int64),
	}
}

func (m *MemStorage) UpdateGauge(name string, value float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.gauge[name] = value
	return nil
}

func (m *MemStorage) UpdateCounter(name string, value int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.counter[name] += value
	return nil
}

func (m *MemStorage) GetGauge(name string) (float64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	value, exist := m.gauge[name]
	return value, exist
}

func (m *MemStorage) GetCounter(name string) (int64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	value, exist := m.counter[name]
	return value, exist
}

func (m *MemStorage) LoadFromFile(filepath string) error {
	file, err := os.Open(filepath)

	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("Failed to open file: %w", err)
	}
	defer file.Close()

	var state SavedState

	if err := json.NewDecoder(file).Decode(&state); err != nil {
		return fmt.Errorf("Failed to decode metric: %w", err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range state.Counters {
		m.counter[k] = v
	}
	for k, v := range state.Gauges {
		m.gauge[k] = v
	}
	return nil
}

func (m *MemStorage) SaveToFile(filepath string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state := SavedState{
		Counters: make(map[string]int64, len(m.counter)),
		Gauges:   make(map[string]float64, len(m.gauge)),
	}

	for k, v := range m.counter {
		state.Counters[k] = v
	}

	for k, v := range m.gauge {
		state.Gauges[k] = v
	}

	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("Failed to create file: %w", err)
	}
	defer file.Close()

	if err := json.NewEncoder(file).Encode(state); err != nil {
		return fmt.Errorf("Failed to encode metric %w", err)
	}
	return nil
}

func (m *MemStorage) Ping() error {
	return nil
}
