package repository

type MemStorage struct {
	gauge   map[string]float64
	counter map[string]int64
}

type Storage interface {
	UpdateGauge(name string, value float64) error
	UpdateCounter(name string, value int64) error
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauge:   make(map[string]float64),
		counter: make(map[string]int64),
	}
}

func (m *MemStorage) UpdateGauge(name string, value float64) error {
	m.gauge[name] = value
	return nil
}

func (m *MemStorage) UpdateCounter(name string, value int64) error {
	m.counter[name] += value
	return nil
}
