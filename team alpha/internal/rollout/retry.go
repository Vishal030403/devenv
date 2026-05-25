package rollout

import (
	"fmt"
	"time"

	"devenv/teamalpha/internal/log"
)

func Retry(operation func() error, retries int) error {
	var err error
	for i := 1; i <= retries; i++ {
		err = operation()
		if err == nil {
			return nil
		}
		log.Warning(fmt.Sprintf("Retry attempt %d failed", i))
		log.Info("Retrying in 5 seconds")
		time.Sleep(5 * time.Second)
	}
	return err
}
