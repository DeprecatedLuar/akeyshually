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

// BuildCandidates maps shortcuts to their win conditions with context-aware phase assignment.
// Phases are determined by what other triggers exist on the same key.
func BuildCandidates(shortcuts []*config.ParsedShortcut) []Candidate {
	// Detect what behaviors are present
	hasDoubleTap := false
	hasTapHold := false

	for _, s := range shortcuts {
		if s.Behavior == config.BehaviorDoubleTap {
			hasDoubleTap = true
		}
		if s.Behavior == config.BehaviorTapHold || s.Behavior == config.BehaviorTapLongPress {
			hasTapHold = true
		}
	}

	// Build candidates with context-aware phases
	var out []Candidate
	for _, s := range shortcuts {
		cond, ok := contextAwareWinCondition(s.Behavior, hasDoubleTap, hasTapHold)
		if !ok {
			continue
		}
		out = append(out, Candidate{Shortcut: s, Condition: cond})
	}
	return out
}

// contextAwareWinCondition assigns win conditions based on behavior and what other triggers exist.
func contextAwareWinCondition(b config.BehaviorMode, hasDoubleTap, hasTapHold bool) (WinCondition, bool) {
	switch b {
	case config.BehaviorNormal:
		// If doubletap exists, onpress must wait for doubletap window to close
		if hasDoubleTap {
			return WinCondition{Count: 1, Pressed: false, Phase: 1}, true
		}
		// Solo onpress wins immediately on release
		return WinCondition{Count: 1, Pressed: false, Phase: 0}, true

	case config.BehaviorPressRelease:
		if hasDoubleTap {
			return WinCondition{Count: 1, Pressed: false, Phase: 1}, true
		}
		return WinCondition{Count: 1, Pressed: false, Phase: 0}, true

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
