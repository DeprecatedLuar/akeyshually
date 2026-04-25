package timers

import (
	"context"
	"sync"

	"github.com/deprecatedluar/akeyshually/internal/config"
)

// WinCondition describes the exact state that must hold for a trigger to win.
type WinCondition struct {
	Count   int  // number of key-down events required
	Pressed bool // key must still be held at resolution time
	Phase   int  // 1 = first timer boundary, 2 = second timer boundary
}

// Candidate pairs a shortcut with its pre-computed win condition.
// Switch and immediate-fire shortcuts are excluded — they fire outside the chain.
type Candidate struct {
	Shortcut  *config.ParsedShortcut
	Condition WinCondition
}

// BuildCandidates maps shortcuts to their win conditions.
func BuildCandidates(shortcuts []*config.ParsedShortcut) []Candidate {
	var out []Candidate
	for _, s := range shortcuts {
		cond, ok := winConditionFor(s.Behavior)
		if !ok {
			continue
		}
		out = append(out, Candidate{Shortcut: s, Condition: cond})
	}
	return out
}

func winConditionFor(b config.BehaviorMode) (WinCondition, bool) {
	switch b {
	case config.BehaviorNormal:
		return WinCondition{Count: 1, Pressed: false, Phase: 1}, true
	case config.BehaviorPressRelease:
		return WinCondition{Count: 1, Pressed: false, Phase: 1}, true
	case config.BehaviorHold, config.BehaviorHoldRelease, config.BehaviorLongPress:
		return WinCondition{Count: 1, Pressed: true, Phase: 1}, true
	case config.BehaviorDoubleTap:
		return WinCondition{Count: 2, Pressed: false, Phase: 0}, true
	case config.BehaviorTapHold, config.BehaviorTapLongPress:
		return WinCondition{Count: 2, Pressed: true, Phase: 2}, true
	}
	return WinCondition{}, false
}

// ComboState drives the chain goroutine for a single key press.
// The event handler signals it via channels; the goroutine owns resolution.
type ComboState struct {
	sync.Mutex
	cancel    context.CancelFunc
	ReleaseCh chan struct{} // key released
	PressCh   chan struct{} // second press arrived (doubletap window)
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

func (s *ComboState) SignalRelease() {
	select {
	case s.ReleaseCh <- struct{}{}:
	default:
	}
}

func (s *ComboState) SignalPress() {
	select {
	case s.PressCh <- struct{}{}:
	default:
	}
}

// StateMap holds one active ComboState per combo.
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
