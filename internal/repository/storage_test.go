package repository

import (
	"os"
	"path/filepath"
	"testing"

	models "github.com/Meowizz/metrics-collector/internal/model"
)

func TestMemStorage_BasicOperations(t *testing.T) {
	storage := NewMemStorage()

	err := storage.UpdateGauge("temp", 25.5)
	if err != nil {
		t.Fatalf("UpdateGauge failed: %v", err)
	}
	val, ok := storage.GetGauge("temp")
	if !ok || val != 25.5 {
		t.Errorf("Expected gauge 'temp' to be 25.5, got %v, exists: %v", val, ok)
	}

	err = storage.UpdateCounter("requests", 5)
	if err != nil {
		t.Fatalf("UpdateCounter failed: %v", err)
	}
	err = storage.UpdateCounter("requests", 3)
	if err != nil {
		t.Fatalf("UpdateCounter failed: %v", err)
	}

	valInt, ok := storage.GetCounter("requests")
	if !ok || valInt != 8 {
		t.Errorf("Expected counter 'requests' to be 8, got %v, exists: %v", valInt, ok)
	}

	if err := storage.Ping(); err != nil {
		t.Errorf("MemStorage Ping should always return nil, got: %v", err)
	}
}

func TestMemStorage_SaveAndLoad(t *testing.T) {

	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test_storage.json")

	storage1 := NewMemStorage()
	storage1.UpdateGauge("cpu", 95.0)
	storage1.UpdateCounter("hits", 100)

	err := storage1.SaveToFile(filePath)
	if err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("Expected file to be created at %s", filePath)
	}

	storage2 := NewMemStorage()
	err = storage2.LoadFromFile(filePath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	val, ok := storage2.GetGauge("cpu")
	if !ok || val != 95.0 {
		t.Errorf("Expected loaded gauge 'cpu' to be 95.0, got %v", val)
	}

	valInt, ok := storage2.GetCounter("hits")
	if !ok || valInt != 100 {
		t.Errorf("Expected loaded counter 'hits' to be 100, got %v", valInt)
	}
}

func TestMemStorage_LoadFromFile_NotExist(t *testing.T) {
	storage := NewMemStorage()
	err := storage.LoadFromFile("/path/to/nonexistent/file.json")
	if err != nil {
		t.Errorf("Expected no error when loading from non-existent file, got: %v", err)
	}
}

func TestMemStorage_UpdateBatch(t *testing.T) {
	storage := NewMemStorage()

	metrics := []models.Metrics{
		{ID: "BatchGauge1", MType: models.Gauge, Value: ptrFloat64(100.5)},
		{ID: "BatchGauge2", MType: models.Gauge, Value: ptrFloat64(200.0)},
		{ID: "BatchCounter1", MType: models.Counter, Delta: ptrInt64(10)},
		{ID: "BatchCounter2", MType: models.Counter, Delta: ptrInt64(20)},
	}

	err := storage.UpdateBatch(metrics)
	if err != nil {
		t.Fatalf("UpdateBatch failed: %v", err)
	}

	// Verify gauges
	val, ok := storage.GetGauge("BatchGauge1")
	if !ok || val != 100.5 {
		t.Errorf("Expected gauge 'BatchGauge1' to be 100.5, got %v, exists: %v", val, ok)
	}

	val, ok = storage.GetGauge("BatchGauge2")
	if !ok || val != 200.0 {
		t.Errorf("Expected gauge 'BatchGauge2' to be 200.0, got %v, exists: %v", val, ok)
	}

	// Verify counters
	valInt, ok := storage.GetCounter("BatchCounter1")
	if !ok || valInt != 10 {
		t.Errorf("Expected counter 'BatchCounter1' to be 10, got %v, exists: %v", valInt, ok)
	}

	valInt, ok = storage.GetCounter("BatchCounter2")
	if !ok || valInt != 20 {
		t.Errorf("Expected counter 'BatchCounter2' to be 20, got %v, exists: %v", valInt, ok)
	}
}

func TestMemStorage_UpdateBatch_AccumulatesCounter(t *testing.T) {
	storage := NewMemStorage()

	// Set initial counter value
	storage.UpdateCounter("AccumCounter", 50)

	// Update batch with delta
	metrics := []models.Metrics{
		{ID: "AccumCounter", MType: models.Counter, Delta: ptrInt64(10)},
	}

	err := storage.UpdateBatch(metrics)
	if err != nil {
		t.Fatalf("UpdateBatch failed: %v", err)
	}

	valInt, ok := storage.GetCounter("AccumCounter")
	if !ok || valInt != 60 {
		t.Errorf("Expected counter 'AccumCounter' to be 60 (50+10), got %v, exists: %v", valInt, ok)
	}
}

func TestMemStorage_UpdateBatch_Empty(t *testing.T) {
	storage := NewMemStorage()

	err := storage.UpdateBatch([]models.Metrics{})
	if err != nil {
		t.Fatalf("UpdateBatch with empty slice should not error, got: %v", err)
	}
}

func TestMemStorage_UpdateBatch_NullValues(t *testing.T) {
	storage := NewMemStorage()

	// Metrics with nil Value/Delta should be skipped
	metrics := []models.Metrics{
		{ID: "NullGauge", MType: models.Gauge, Value: nil},
		{ID: "NullCounter", MType: models.Counter, Delta: nil},
	}

	err := storage.UpdateBatch(metrics)
	if err != nil {
		t.Fatalf("UpdateBatch with null values should not error, got: %v", err)
	}

	// These should not exist since nil values are skipped
	_, okGauge := storage.GetGauge("NullGauge")
	_, okCounter := storage.GetCounter("NullCounter")

	if okGauge {
		t.Error("Expected NullGauge to not exist (nil value skipped)")
	}
	if okCounter {
		t.Error("Expected NullCounter to not exist (nil delta skipped)")
	}
}

func ptrFloat64(v float64) *float64 {
	return &v
}

func ptrInt64(v int64) *int64 {
	return &v
}
