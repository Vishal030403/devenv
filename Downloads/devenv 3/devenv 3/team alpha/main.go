package main

import (
	"devenv/teamalpha/cmd"
	"devenv/teamalpha/internal/config"
)

func main() {
	// Config is optional — missing configs/config.yaml falls back to built-in defaults.
	_ = config.LoadConfig()
	cmd.Execute()
}
