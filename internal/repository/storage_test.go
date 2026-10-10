package repository

import (
	"os"
	"path/filepath"
	"testing"
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
