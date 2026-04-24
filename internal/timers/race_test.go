package timers

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestTimerFires(t *testing.T) {
	var fired int32
	timer := Start(50, func() { atomic.StoreInt32(&fired, 1) })
	_ = timer
	time.Sleep(100 * time.Millisecond)
	if atomic.LoadInt32(&fired) != 1 {
		t.Error("timer did not fire")
	}
}

func TestTimerCancel(t *testing.T) {
	var fired int32
	timer := Start(50, func() { atomic.StoreInt32(&fired, 1) })
	timer.Cancel()
	time.Sleep(100 * time.Millisecond)
	if atomic.LoadInt32(&fired) != 0 {
		t.Error("cancelled timer should not fire")
	}
}

func TestComboStateSignals(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	s := NewComboState(cancel)

	s.SignalRelease()
	select {
	case <-s.ReleaseCh:
	default:
		t.Error("SignalRelease did not send")
	}

	s.SignalPress()
	select {
	case <-s.PressCh:
	default:
		t.Error("SignalPress did not send")
	}
}

func TestComboStateCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := NewComboState(cancel)
	_ = ctx
	s.Cancel()
	select {
	case <-ctx.Done():
	case <-time.After(50 * time.Millisecond):
		t.Error("Cancel did not cancel context")
	}
}

func TestComboStateSignalNonBlocking(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	s := NewComboState(cancel)
	// Fill the buffered channel
	s.SignalRelease()
	// Second signal must not block
	done := make(chan struct{})
	go func() {
		s.SignalRelease()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(50 * time.Millisecond):
		t.Error("SignalRelease blocked on full channel")
	}
}

func TestStateMapSetGetDelete(t *testing.T) {
	sm := NewStateMap()
	_, cancel := context.WithCancel(context.Background())
	s := NewComboState(cancel)

	sm.Set("super+t", s)
	if got := sm.Get("super+t"); got != s {
		t.Error("Get returned wrong state")
	}
	sm.Delete("super+t")
	if got := sm.Get("super+t"); got != nil {
		t.Error("Get after Delete should return nil")
	}
}
