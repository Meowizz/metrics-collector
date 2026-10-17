package pkg

import (
	"errors"
	"testing"
)

func TestDoWithRetry_Success(t *testing.T) {
	attemptCount := 0
	err := DoWithRetry(func() error {
		attemptCount++
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if attemptCount != 1 {
		t.Errorf("expected 1 attempt, got %d", attemptCount)
	}
}

func TestDoWithRetry_RetriesOnTransient(t *testing.T) {
	attemptCount := 0
	err := DoWithRetry(func() error {
		attemptCount++
		if attemptCount < 3 {
			return errors.New("transient error")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error after retries, got %v", err)
	}
	if attemptCount != 3 {
		t.Errorf("expected 3 attempts, got %d", attemptCount)
	}
}

func TestDoWithRetry_FailsAfterMaxRetries(t *testing.T) {
	attemptCount := 0
	err := DoWithRetry(func() error {
		attemptCount++
		return errors.New("persistent error")
	})
	if err == nil {
		t.Fatal("expected error after max retries, got nil")
	}
	if attemptCount != 4 { // 1 initial + 3 retries
		t.Errorf("expected 4 attempts, got %d", attemptCount)
	}
	if attemptCount != 4 {
		t.Fatalf("expected 4 total attempts (1 initial + 3 retries), got %d", attemptCount)
	}
}

func TestDoWithRetry_NonRetriable(t *testing.T) {
	attemptCount := 0
	err := DoWithRetry(func() error {
		attemptCount++
		return ErrNotRetriable
	})
	if err == nil {
		t.Fatal("expected ErrNotRetriable, got nil")
	}
	if !errors.Is(err, ErrNotRetriable) {
		t.Errorf("expected ErrNotRetriable, got %v", err)
	}
	if attemptCount != 1 {
		t.Errorf("expected 1 attempt (no retries), got %d", attemptCount)
	}
}
