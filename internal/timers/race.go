package timers

import (
	"context"
	"sync"

	"github.com/deprecatedluar/akeyshually/internal/config"
)

// Candidate represents a shortcut that participates in the timer ladder.
// Switch behaviors are excluded — they fire outside the ladder.
type Candidate struct {
	Shortcut *config.ParsedShortcut
}

// BuildCandidates filters shortcuts to those that need ladder resolution.
// Switch behaviors fire immediately and are excluded.
func BuildCandidates(shortcuts []*config.ParsedShortcut) []Candidate {
	var out []Candidate
	for _, s := range shortcuts {
		// Exclude switch — fires outside the ladder
		if s.Behavior == config.BehaviorSwitch {
			continue
		}
		out = append(out, Candidate{Shortcut: s})
	}
	return out
}

// NewEscapeCandidate creates a pseudo-candidate that prevents premature ladder resolution
// when escape hatches (e.g. super+w, super+shift+b) might arrive.
func NewEscapeCandidate() Candidate {
	return Candidate{
		Shortcut: &config.ParsedShortcut{
			Behavior: config.BehaviorEscapePending,
		},
	}
}

// ComboState drives the chain goroutine for a single key press.
// The event handler signals it via channels; the goroutine owns resolution.
type ComboState struct {
	sync.Mutex
	cancel    context.CancelFunc
	ReleaseCh chan struct{} // key released
	PressCh   chan struct{} // second press arrived (doubletap window)
	EscapeCh  chan string   // foreign key pressed (escape hatch to combo)
}

func NewComboState(cancel context.CancelFunc) *ComboState {
	return &ComboState{
		cancel:    cancel,
		ReleaseCh: make(chan struct{}, 1),
		PressCh:   make(chan struct{}, 1),
		EscapeCh:  make(chan string, 1),
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

// CancelCombosWithModifier cancels all active combos that include the given modifier
func (sm *StateMap) CancelCombosWithModifier(modifierName string) []string {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var cancelled []string
	prefix := modifierName + "+"

	for combo, state := range sm.states {
		// Match "modifier+key" or exact "modifier"
		if combo == modifierName || len(combo) > len(prefix) && combo[:len(prefix)] == prefix {
			state.Cancel()
			delete(sm.states, combo)
			cancelled = append(cancelled, combo)
		}
	}

	return cancelled
}

// EmittedModifierTracker tracks which modifiers we've emitted to system (so we can release them)
type EmittedModifierTracker struct {
	mu      sync.Mutex
	emitted map[string]bool // modifier name -> emitted state
}

func NewEmittedModifierTracker() *EmittedModifierTracker {
	return &EmittedModifierTracker{
		emitted: make(map[string]bool),
	}
}

func (t *EmittedModifierTracker) MarkEmitted(keyName string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.emitted[keyName] = true
}

func (t *EmittedModifierTracker) WasEmitted(keyName string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.emitted[keyName]
}

func (t *EmittedModifierTracker) ClearEmitted(keyName string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.emitted, keyName)
}
