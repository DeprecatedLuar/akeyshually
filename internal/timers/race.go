package timers

import (
	"context"
	"sync"
)

const (
	PhaseIdle          = 0
	PhaseHoldWindow    = 1
	PhaseDoubleTapWindow = 2
	PhaseTapHoldWindow = 3
)

// ComboState persists across press/release for one combo, driving the sequential chain.
type ComboState struct {
	sync.Mutex
	Phase     int
	cancel    context.CancelFunc
	ReleaseCh chan struct{} // press → release signal
	PressCh   chan struct{} // second press signal (doubletap window)
}

func NewComboState(cancel context.CancelFunc) *ComboState {
	return &ComboState{
		cancel:    cancel,
		ReleaseCh: make(chan struct{}, 1),
		PressCh:   make(chan struct{}, 1),
	}
}

func (s *ComboState) Cancel() {
	if s != nil && s.cancel != nil {
		s.cancel()
	}
}

// SignalRelease notifies the chain goroutine that the key was released.
func (s *ComboState) SignalRelease() {
	select {
	case s.ReleaseCh <- struct{}{}:
	default:
	}
}

// SignalPress notifies the chain goroutine that a second press arrived.
func (s *ComboState) SignalPress() {
	select {
	case s.PressCh <- struct{}{}:
	default:
	}
}

// StateMap holds one ComboState per combo.
type StateMap struct {
	mu     sync.Mutex
	states map[string]*ComboState
}

func NewStateMap() *StateMap {
	return &StateMap{states: make(map[string]*ComboState)}
}

func (sm *StateMap) Get(combo string) *ComboState {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.states[combo]
}

func (sm *StateMap) Set(combo string, s *ComboState) {
	sm.mu.Lock()
	sm.states[combo] = s
	sm.mu.Unlock()
}

func (sm *StateMap) Delete(combo string) {
	sm.mu.Lock()
	delete(sm.states, combo)
	sm.mu.Unlock()
}
