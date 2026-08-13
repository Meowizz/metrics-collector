package pkg

import (
	"errors"
	"fmt"
	"time"
)

var ErrNotRetriable = errors.New("operation is not retriable")

func DoWithRetry(operation func() error) error {
	delays := []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}
	var lastErr error

	for attempt := 0; attempt <= 3; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}

		if errors.Is(err, ErrNotRetriable) {
			return err
		}

		lastErr = err

		if attempt < 3 {
			time.Sleep(delays[attempt])
		}
	}

	return fmt.Errorf("operation failed after 3 retries: %w", lastErr)
}
