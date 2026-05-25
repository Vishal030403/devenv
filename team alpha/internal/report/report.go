package report

import (
	"fmt"
	"time"

	"devenv/teamalpha/internal/log"
)

type Status string

const (
	StatusOK      Status = "OK"
	StatusWarning Status = "WARNING"
	StatusFailed  Status = "FAILED"
)

type entry struct {
	Stage    string
	Status   Status
	Duration time.Duration
}

type Report struct {
	entries []entry
}

func New() *Report { return &Report{} }

func (r *Report) Record(stage string, status Status, duration time.Duration) {
	r.entries = append(r.entries, entry{Stage: stage, Status: status, Duration: duration})
}

func (r *Report) Print() {
	log.Info("=== Deployment Summary ===")
	for _, e := range r.entries {
		line := fmt.Sprintf("%-32s %8s", e.Stage, truncateDuration(e.Duration))
		switch e.Status {
		case StatusOK:
			log.OK(line)
		case StatusWarning:
			log.Warning(line)
		case StatusFailed:
			log.Failed(line)
		}
	}
	log.Info("=== End Summary ===")
}

func truncateDuration(d time.Duration) string {
	if d >= time.Second {
		return d.Round(100 * time.Millisecond).String()
	}
	return d.Round(time.Millisecond).String()
}
