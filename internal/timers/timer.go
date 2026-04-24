package timers

import (
	"context"
	"time"
)

// Timer is a cancellable one-shot timer.
type Timer struct {
	cancel context.CancelFunc
}

// Start fires fn after interval milliseconds unless cancelled first.
func Start(interval float64, fn func()) *Timer {
	ctx, cancel := context.WithCancel(context.Background())
	t := &Timer{cancel: cancel}
	go func() {
		select {
		case <-ctx.Done():
		case <-time.After(time.Duration(interval) * time.Millisecond):
			fn()
		}
	}()
	return t
}

// Cancel stops the timer before it fires. Safe to call multiple times.
func (t *Timer) Cancel() {
	if t != nil {
		t.cancel()
	}
}
