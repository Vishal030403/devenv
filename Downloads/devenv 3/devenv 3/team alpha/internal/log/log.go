package log

import (
	"fmt"
	"time"
)

const pause = 250 * time.Millisecond

func ts() string {
	return time.Now().Format("15:04:05")
}

func Info(message string) {
	fmt.Printf("[%s] [INFO]    %s\n", ts(), message)
	time.Sleep(pause)
}

func Running(message string) {
	fmt.Printf("[%s] [RUNNING] %s\n", ts(), message)
	time.Sleep(pause)
}

func Step(number, total int, title string) {
	fmt.Printf("\n[%s] [STEP %d/%d] %s ...\n", ts(), number, total, title)
	time.Sleep(400 * time.Millisecond)
}

func OK(message string) {
	fmt.Printf("[%s] [OK]      %s\n", ts(), message)
	time.Sleep(pause)
}

func Success(message string) {
	fmt.Printf("[%s] [SUCCESS] %s\n", ts(), message)
	time.Sleep(pause)
}

func Warn(message string) {
	fmt.Printf("[%s] [WARN]    %s\n", ts(), message)
	time.Sleep(pause)
}

func Warning(message string) {
	Warn(message)
}

func Done(message string) {
	fmt.Printf("\n[%s] [DONE]    %s\n", ts(), message)
	time.Sleep(pause)
}

func Error(message string) {
	fmt.Printf("[%s] [ERROR]   %s\n", ts(), message)
	time.Sleep(pause)
}

func Failed(message string) {
	Error(message)
}

func Hint(message string) {
	fmt.Printf("[%s] [HINT]    %s\n", ts(), message)
	time.Sleep(pause)
}

func StageLine(name, status string, dur time.Duration) {
	durStr := "-"
	if dur > 0 {
		durStr = dur.Round(time.Millisecond).String()
	}
	fmt.Printf("[%s] [STAGE]   %-30s %-20s %s\n", ts(), name, status, durStr)
	time.Sleep(pause)
}
