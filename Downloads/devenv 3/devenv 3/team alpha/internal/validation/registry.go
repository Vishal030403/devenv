package validation

import (
	"fmt"

	"devenv/teamalpha/internal/smoke"
)

func ValidateRegistry(port int) error {
	url := fmt.Sprintf("http://127.0.0.1:%d/v2/", port)
	return smoke.ValidateURL("Registry", url)
}
