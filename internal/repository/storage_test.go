package repository

import (
	"testing"
)

func TestMemStorage_UpdateGauge(t *testing.T) {
	type args struct {
		name  string
		value float64
	}
	tests := []struct {
		name       string
		m          *MemStorage
		args       args
		wantErr    bool
		wantValue  float64
		wantExists bool
	}{
		{
			name:       "Create new gauge metric",
			m:          NewMemStorage(),
			args:       args{name: "Alloc", value: 123.45},
			wantErr:    false,
			wantValue:  123.45,
			wantExists: true,
		},
		{
			name: "Overwrite existing gauge metric",
			m: &MemStorage{
				gauge:   map[string]float64{"Alloc": 100.0},
				counter: make(map[string]int64),
			},
			args:       args{name: "Alloc", value: 200.5},
			wantErr:    false,
			wantValue:  200.5,
			wantExists: true,
		},
		{
			name:       "Zero value gauge",
			m:          NewMemStorage(),
			args:       args{name: "Test", value: 0.0},
			wantErr:    false,
			wantValue:  0.0,
			wantExists: true,
		},
		{
			name:       "Negative value gauge",
			m:          NewMemStorage(),
			args:       args{name: "Test", value: -42.5},
			wantErr:    false,
			wantValue:  -42.5,
			wantExists: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.m.UpdateGauge(tt.args.name, tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("MemStorage.UpdateGauge() error = %v, wantErr %v", err, tt.wantErr)
			}

			gotValue, gotExists := tt.m.gauge[tt.args.name]
			if gotExists != tt.wantExists {
				t.Errorf("Metric exists = %v, want %v", gotExists, tt.wantExists)
			}
			if gotValue != tt.wantValue {
				t.Errorf("Metric value = %v, want %v", gotValue, tt.wantValue)
			}
		})
	}
}

func TestMemStorage_UpdateCounter(t *testing.T) {
	type args struct {
		name  string
		value int64
	}
	tests := []struct {
		name       string
		m          *MemStorage
		args       args
		wantErr    bool
		wantValue  int64
		wantExists bool
	}{
		{
			name:       "Create new counter metric",
			m:          NewMemStorage(),
			args:       args{name: "PollCount", value: 1},
			wantErr:    false,
			wantValue:  1,
			wantExists: true,
		},
		{
			name: "Increment existing counter",
			m: &MemStorage{
				gauge:   make(map[string]float64),
				counter: map[string]int64{"PollCount": 5},
			},
			args:       args{name: "PollCount", value: 1},
			wantErr:    false,
			wantValue:  6,
			wantExists: true,
		},
		{
			name:       "Increment by large value",
			m:          NewMemStorage(),
			args:       args{name: "Test", value: 100},
			wantErr:    false,
			wantValue:  100,
			wantExists: true,
		},
		{
			name:       "Multiple increments",
			m:          NewMemStorage(),
			args:       args{name: "Test", value: 1},
			wantErr:    false,
			wantValue:  1,
			wantExists: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.m.UpdateCounter(tt.args.name, tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("MemStorage.UpdateCounter() error = %v, wantErr %v", err, tt.wantErr)
			}

			gotValue, gotExists := tt.m.counter[tt.args.name]
			if gotExists != tt.wantExists {
				t.Errorf("Metric exists = %v, want %v", gotExists, tt.wantExists)
			}
			if gotValue != tt.wantValue {
				t.Errorf("Metric value = %v, want %v", gotValue, tt.wantValue)
			}
		})
	}
}

func TestMemStorage_CounterIncrement(t *testing.T) {
	storage := NewMemStorage()

	for i := 0; i < 5; i++ {
		storage.UpdateCounter("PollCount", 1)
	}

	value := storage.counter["PollCount"]
	if value != 5 {
		t.Errorf("Expected counter to be 5, got %d", value)
	}
}

func TestMemStorage_GaugeOverwrite(t *testing.T) {
	storage := NewMemStorage()

	storage.UpdateGauge("Alloc", 100.0)
	storage.UpdateGauge("Alloc", 200.0)
	storage.UpdateGauge("Alloc", 300.0)

	value := storage.gauge["Alloc"]
	if value != 300.0 {
		t.Errorf("Expected gauge to be 300.0, got %f", value)
	}
}
