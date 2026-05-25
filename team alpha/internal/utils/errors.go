package utils

import "fmt"

func FormatError(stage string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("[%s] %s", stage, err.Error())
}
