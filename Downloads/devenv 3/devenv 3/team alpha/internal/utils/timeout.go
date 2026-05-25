package utils

import (
	"errors"
	"time"
)

func WithTimeout(seconds int, fn func() error) error {
	done := make(chan error, 1)
	go func() { done <- fn() }()

	select {
	case err := <-done:
		return err
	case <-time.After(time.Duration(seconds) * time.Second):
		return errors.New("operation timed out")
	}
}
