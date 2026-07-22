package repository

import "sync"

type MemStorage struct {
	mu      sync.RWMutex
	gauge   map[string]float64
	counter map[string]int64
}

type Storage interface {
	UpdateGauge(name string, value float64) error
	UpdateCounter(name string, value int64) error
	GetGauge(name string) (float64, bool)
	GetCounter(name string) (int64, bool)
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
