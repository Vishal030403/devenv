package utils

import "time"

func StartTimer() time.Time                  { return time.Now() }
func EndTimer(start time.Time) time.Duration { return time.Since(start) }
