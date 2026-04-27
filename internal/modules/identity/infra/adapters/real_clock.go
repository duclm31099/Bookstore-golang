package adapters

import "time"

// RealClock implements ports.Clock using the system clock.
type RealClock struct{}

func NewRealClock() *RealClock {
	return &RealClock{}
}

func (c *RealClock) Now() time.Time {
	return time.Now()
}
