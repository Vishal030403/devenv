package retry

import (
	"fmt"
	"time"

	"devenv/teamalpha/internal/log"
)

// Config defines retry behavior
type Config struct {
	MaxAttempts int
	Delay       time.Duration
	OnRetry     func(attempt int, err error)
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		MaxAttempts: 3,
		Delay:       2 * time.Second,
		OnRetry: func(attempt int, err error) {
			log.Warn(fmt.Sprintf("Attempt %d failed: %v", attempt, err))
		},
	}
}

// Do executes fn with retry logic
func Do(fn func() error, cfg Config) error {
	var lastErr error

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		if attempt > 1 && cfg.Delay > 0 {
			time.Sleep(cfg.Delay)
		}

		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		if attempt < cfg.MaxAttempts && cfg.OnRetry != nil {
			cfg.OnRetry(attempt, err)
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", cfg.MaxAttempts, lastErr)
}

// DoWithBackoff executes fn with exponential backoff
func DoWithBackoff(fn func() error, maxAttempts int, initialDelay time.Duration) error {
	var lastErr error
	delay := initialDelay

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			log.Info(fmt.Sprintf("Retry %d/%d (waiting %v)...", attempt, maxAttempts, delay))
			time.Sleep(delay)
			delay = delay * 2
		}

		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err
		log.Warn(fmt.Sprintf("Attempt %d failed: %v", attempt, err))
	}

	return fmt.Errorf("failed after %d attempts: %w", maxAttempts, lastErr)
}
