package mcts

import (
	"time"
)

type _Timer struct {
	start    time.Time
	duration time.Duration
}

func _NewTimer() *_Timer {
	return &_Timer{time.Now(), -1}
}

// Check if this timer has ended
func (t *_Timer) IsEnd() bool {
	return t.duration > 0 && time.Since(t.start) >= t.duration
}

func (t *_Timer) IsSet() bool {
	return t.duration != -1
}

// Set the 'start' as now
func (t *_Timer) Reset() {
	t.start = time.Now()
}

// Get the start time
func (t *_Timer) Start() time.Time {
	return t.start
}

func (t *_Timer) Deltatime() int {
	return max(int(time.Since(t.start).Milliseconds()), 1)
}

// In milliseconds
func (t *_Timer) Movetime(movetime int) {
	if movetime < 0 {
		t.duration = -1
	} else {
		t.duration = time.Duration(movetime) * time.Millisecond
	}
}
