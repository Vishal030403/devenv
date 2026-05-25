package pipeline

import (
	"fmt"
	"strings"
	"time"

	"devenv/teamalpha/internal/log"
	runtimeengine "devenv/teamalpha/internal/runtime"
)

type Status string

const (
	StatusRunning Status = "RUNNING"
	StatusSuccess Status = "SUCCESS"
	StatusFailed  Status = "FAILED"
	StatusSkipped Status = "SKIPPED"
)

type Stage struct {
	Name     string
	Status   Status
	Duration time.Duration
	Err      error
}

type Pipeline struct {
	Stages     []Stage
	startTime  time.Time
	failedAt   string
	rolledBack bool
}

func New() *Pipeline {
	return &Pipeline{startTime: time.Now()}
}

// Run executes a fatal stage. A failure stops the pipeline (caller must return the error).
func (p *Pipeline) Run(name string, fn func() error) error {
	log.Info("Stage: " + name)
	start := time.Now()
	err := fn()
	dur := time.Since(start)

	s := Stage{Name: name, Duration: dur, Err: err}
	if err != nil {
		s.Status = StatusFailed
		if p.failedAt == "" {
			p.failedAt = name
		}
		log.StageLine(name, string(StatusFailed), dur)
	} else {
		s.Status = StatusSuccess
		log.StageLine(name, string(StatusSuccess), dur)
	}
	p.Stages = append(p.Stages, s)
	return err
}

// RunOptional executes a non-fatal stage. Failures are logged but do not stop the pipeline.
// If the function returns a SkipError, the stage is recorded as SKIPPED with the reason.
func (p *Pipeline) RunOptional(name string, fn func() error) {
	log.Info("Stage: " + name + " (optional)")
	start := time.Now()
	err := fn()
	dur := time.Since(start)

	s := Stage{Name: name, Duration: dur, Err: err}
	if err != nil {
		if runtimeengine.IsSkip(err) {
			s.Status = StatusSkipped
			log.Warn(fmt.Sprintf("Stage %q skipped: %s", name, err.Error()))
			log.StageLine(name, string(StatusSkipped), 0)
		} else {
			s.Status = StatusFailed
			log.Warn(fmt.Sprintf("Optional stage %q failed: %v", name, err))
			log.StageLine(name, string(StatusFailed)+" (non-fatal)", dur)
		}
	} else {
		s.Status = StatusSuccess
		log.StageLine(name, string(StatusSuccess), dur)
	}
	p.Stages = append(p.Stages, s)
}

// Skip records a stage as skipped with a reason.
func (p *Pipeline) Skip(name, reason string) {
	log.Warn(fmt.Sprintf("Stage %q skipped: %s", name, reason))
	log.StageLine(name, string(StatusSkipped), 0)
	p.Stages = append(p.Stages, Stage{Name: name, Status: StatusSkipped})
}

func (p *Pipeline) MarkRolledBack()  { p.rolledBack = true }
func (p *Pipeline) RolledBack() bool { return p.rolledBack }
func (p *Pipeline) FailedAt() string { return p.failedAt }

func (p *Pipeline) PrintSummary() {
	total := time.Since(p.startTime)
	sep := strings.Repeat("─", 64)

	fmt.Printf("\n%s\n", sep)
	fmt.Printf("  Pipeline Execution Summary\n")
	fmt.Printf("%s\n", sep)
	fmt.Printf("  %-30s  %-22s  %s\n", "Stage", "Status", "Duration")
	fmt.Printf("  %-30s  %-22s  %s\n",
		strings.Repeat("─", 30), strings.Repeat("─", 22), strings.Repeat("─", 10))

	for _, s := range p.Stages {
		durStr := "-"
		if s.Duration > 0 {
			durStr = s.Duration.Round(time.Millisecond).String()
		}
		fmt.Printf("  %-30s  %-22s  %s\n", s.Name, string(s.Status), durStr)
	}

	fmt.Printf("%s\n", sep)
	if p.failedAt != "" {
		fmt.Printf("  Failed Stage  : %s\n", p.failedAt)
	} else {
		fmt.Printf("  Failed Stage  : none\n")
	}
	rollbackStr := "not triggered"
	if p.rolledBack {
		rollbackStr = "triggered"
	}
	fmt.Printf("  Rollback      : %s\n", rollbackStr)
	fmt.Printf("  Total Duration: %s\n", total.Round(time.Millisecond))
	result := "SUCCESS"
	if p.failedAt != "" {
		result = "FAILED"
	}
	fmt.Printf("  Result        : %s\n", result)
	fmt.Printf("%s\n\n", sep)
}
