package collector

import (
	"testing"
)

func TestNewCollector(t *testing.T) {
	c := NewCollector()
	if c == nil {
		t.Fatal("expected non-nil collector")
	}
	if c.metrics == nil {
		t.Fatal("expected non-nil metrics map")
	}
	if len(c.metrics) != 0 {
		t.Errorf("expected empty metrics map, got %d entries", len(c.metrics))
	}
}

func TestCollector_Collect(t *testing.T) {
	c := NewCollector()
	c.Collect()

	metrics := c.GetMetrics()
	if len(metrics) == 0 {
		t.Fatal("expected metrics after Collect(), got none")
	}

	// Check that expected runtime metrics are present
	metricMap := make(map[string]*Metric)
	for _, m := range metrics {
		metricMap[m.Name] = m
	}

	expectedKeys := []string{
		"Alloc", "Sys", "HeapAlloc", "HeapObjects",
		"NumGC", "TotalAlloc", "GCCPUFraction", "RandomValue",
	}
	for _, key := range expectedKeys {
		m, ok := metricMap[key]
		if !ok {
			t.Errorf("expected metric %q to be present", key)
			continue
		}
		if m.Type != Gauge {
			t.Errorf("expected metric %q to be Gauge, got %s", key, m.Type)
		}
		if m.Name != key {
			t.Errorf("expected metric name %q, got %q", key, m.Name)
		}
	}

	// PollCount should exist and be a counter
	pollCount, ok := metricMap["PollCount"]
	if !ok {
		t.Error("expected metric PollCount to be present")
	} else {
		if pollCount.Type != Counter {
			t.Errorf("expected PollCount to be Counter, got %s", pollCount.Type)
		}
	}
}

func TestCollector_Collect_IncrementsPollCount(t *testing.T) {
	c := NewCollector()

	// First collection
	c.Collect()
	metrics1 := c.GetMetrics()
	pollCount1 := 0
	for _, m := range metrics1 {
		if m.Name == "PollCount" {
			if v, ok := m.Value.(int64); ok {
				pollCount1 = int(v)
			}
		}
	}
	if pollCount1 != 1 {
		t.Errorf("expected PollCount=1 after first Collect(), got %d", pollCount1)
	}

	// Second collection
	c.Collect()
	metrics2 := c.GetMetrics()
	pollCount2 := 0
	for _, m := range metrics2 {
		if m.Name == "PollCount" {
			if v, ok := m.Value.(int64); ok {
				pollCount2 = int(v)
			}
		}
	}
	if pollCount2 != 2 {
		t.Errorf("expected PollCount=2 after second Collect(), got %d", pollCount2)
	}
}

func TestCollector_GetMetrics(t *testing.T) {
	c := NewCollector()

	// Before collect, metrics should be empty
	metrics := c.GetMetrics()
	if len(metrics) != 0 {
		t.Errorf("expected empty metrics before Collect(), got %d", len(metrics))
	}

	// After collect, should have metrics
	c.Collect()
	metrics = c.GetMetrics()
	if len(metrics) == 0 {
		t.Fatal("expected metrics after Collect(), got none")
	}

	// GetMetrics should be thread-safe (test with concurrent reads)
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_ = c.GetMetrics()
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestCollector_CollectGopsutil(t *testing.T) {
	c := NewCollector()
	c.CollectGopsutil()

	metrics := c.GetMetrics()
	if len(metrics) == 0 {
		t.Fatal("expected metrics after CollectGopsutil(), got none")
	}

	metricMap := make(map[string]*Metric)
	for _, m := range metrics {
		metricMap[m.Name] = m
	}

	// TotalMemory and FreeMemory should be present (may be 0 if gopsutil fails)
	for _, key := range []string{"TotalMemory", "FreeMemory"} {
		if _, ok := metricMap[key]; !ok {
			t.Errorf("expected metric %q to be present after CollectGopsutil()", key)
		}
	}
}
