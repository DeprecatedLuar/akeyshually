package matcher

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/deprecatedluar/akeyshually/internal/config"

	evdev "github.com/holoplot/go-evdev"
)

type ModifierState struct {
	Super bool
	Ctrl  bool
	Alt   bool
	Shift bool
}

// ShortcutKey uniquely identifies a shortcut by combo + behavior + timing
type ShortcutKey struct {
	Combo    string
	Behavior config.BehaviorMode
	Timing   config.TimingMode
}

// TapState is shared across all input devices (keyboards and mice)
type TapState struct {
	sync.RWMutex
	candidate uint16 // Which modifier key is the tap candidate (0 = none)
}

// NewTapState creates a new shared tap state
func NewTapState() *TapState {
	return &TapState{}
}

// MarkCandidate sets the tap candidate
func (ts *TapState) MarkCandidate(code uint16) {
	ts.Lock()
	ts.candidate = code
	ts.Unlock()
}

// Clear clears the tap candidate
func (ts *TapState) Clear() {
	ts.Lock()
	ts.candidate = 0
	ts.Unlock()
}

// Check returns true if the given code is the current tap candidate
func (ts *TapState) Check(code uint16) bool {
	ts.RLock()
	defer ts.RUnlock()
	return ts.candidate == code
}

// DoubleTapState tracks pending double-tap timers
type DoubleTapState struct {
	sync.Mutex
	pendingKey  uint16             // Which key is waiting for second tap (0 = none)
	timerCancel context.CancelFunc // Cancel function for timeout goroutine
}

// NewDoubleTapState creates a new double-tap state
func NewDoubleTapState() *DoubleTapState {
	return &DoubleTapState{}
}

// StartTimer starts waiting for a second tap
func (ds *DoubleTapState) StartTimer(code uint16, interval float64, onTimeout func()) {
	ds.Lock()
	defer ds.Unlock()

	// Cancel existing timer if any
	if ds.timerCancel != nil {
		ds.timerCancel()
	}

	ds.pendingKey = code

	ctx, cancel := context.WithCancel(context.Background())
	ds.timerCancel = cancel

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(interval) * time.Millisecond):
			if onTimeout != nil {
				onTimeout()
			}
			ds.Lock()
			ds.pendingKey = 0
			ds.timerCancel = nil
			ds.Unlock()
		}
	}()
}

// CheckSecondTap checks if this is a valid second tap
func (ds *DoubleTapState) CheckSecondTap(code uint16) bool {
	ds.Lock()
	defer ds.Unlock()
	return ds.pendingKey == code
}

// CancelTimer cancels the pending timer and clears state
func (ds *DoubleTapState) CancelTimer() {
	ds.Lock()
	defer ds.Unlock()

	if ds.timerCancel != nil {
		ds.timerCancel()
		ds.timerCancel = nil
	}
	ds.pendingKey = 0
}

// Clear clears the state (for mouse clicks)
func (ds *DoubleTapState) Clear() {
	ds.CancelTimer()
}

type Matcher struct {
	state     ModifierState
	shortcuts map[ShortcutKey]*config.ParsedShortcut

	// Passthrough shortcuts (indexed by base key only, no modifiers)
	passthroughShortcuts map[ShortcutKey]*config.ParsedShortcut

	// Switch state (cycle through commands)
	switchState map[string]int // "super+k.switch.press" -> next index
	switchMutex sync.Mutex

	// Toggle state (loop on/off)
	toggleState map[string]context.CancelFunc // "super+k.toggle.press" -> cancel func
	toggleMutex sync.Mutex

	// Tap shortcuts (lone modifiers with .onrelease)
	tapShortcuts map[uint16]string

	// Double-tap shortcuts (lone modifiers with .doubletap)
	doubleTapShortcuts map[uint16]*config.ParsedShortcut

	// Shared tap state (for mouse cancellation)
	tapState *TapState

	// Shared double-tap state
	doubleTapState *DoubleTapState

	// Reusable string builder (avoids allocations in hot path)
	comboBuilder strings.Builder
}

func New(parsedShortcuts map[string][]*config.ParsedShortcut) *Matcher {
	shortcuts := make(map[ShortcutKey]*config.ParsedShortcut)
	passthroughShortcuts := make(map[ShortcutKey]*config.ParsedShortcut)
	tapShortcuts := make(map[uint16]string)
	doubleTapShortcuts := make(map[uint16]*config.ParsedShortcut)

	for _, shortcutList := range parsedShortcuts {
		for _, shortcut := range shortcutList {
			key := ShortcutKey{
				Combo:    shortcut.KeyCombo,
				Behavior: shortcut.Behavior,
				Timing:   shortcut.Timing,
			}

			if shortcut.Passthrough {
				passthroughShortcuts[key] = shortcut
			} else {
				shortcuts[key] = shortcut
			}

			// Check for tap shortcuts (lone modifiers with .onrelease)
			if shortcut.Timing == config.TimingRelease && len(shortcut.Commands) == 1 {
				normalized := strings.ToLower(shortcut.KeyCombo)
				switch normalized {
				case "super":
					tapShortcuts[evdev.KEY_LEFTMETA] = shortcut.Commands[0]
					tapShortcuts[evdev.KEY_RIGHTMETA] = shortcut.Commands[0]
				case "ctrl":
					tapShortcuts[evdev.KEY_LEFTCTRL] = shortcut.Commands[0]
					tapShortcuts[evdev.KEY_RIGHTCTRL] = shortcut.Commands[0]
				case "alt":
					tapShortcuts[evdev.KEY_LEFTALT] = shortcut.Commands[0]
					tapShortcuts[evdev.KEY_RIGHTALT] = shortcut.Commands[0]
				case "shift":
					tapShortcuts[evdev.KEY_LEFTSHIFT] = shortcut.Commands[0]
					tapShortcuts[evdev.KEY_RIGHTSHIFT] = shortcut.Commands[0]
				}
			}

			// Check for double-tap shortcuts (any key with .doubletap)
			if shortcut.Behavior == config.BehaviorDoubleTap {
				normalized := strings.ToLower(shortcut.KeyCombo)
				switch normalized {
				case "super":
					doubleTapShortcuts[evdev.KEY_LEFTMETA] = shortcut
					doubleTapShortcuts[evdev.KEY_RIGHTMETA] = shortcut
				case "ctrl":
					doubleTapShortcuts[evdev.KEY_LEFTCTRL] = shortcut
					doubleTapShortcuts[evdev.KEY_RIGHTCTRL] = shortcut
				case "alt":
					doubleTapShortcuts[evdev.KEY_LEFTALT] = shortcut
					doubleTapShortcuts[evdev.KEY_RIGHTALT] = shortcut
				case "shift":
					doubleTapShortcuts[evdev.KEY_LEFTSHIFT] = shortcut
					doubleTapShortcuts[evdev.KEY_RIGHTSHIFT] = shortcut
				default:
					if keyCode := getKeyCode(normalized); keyCode != 0 {
						doubleTapShortcuts[keyCode] = shortcut
					}
				}
			}
		}
	}

	return &Matcher{
		shortcuts:            shortcuts,
		passthroughShortcuts: passthroughShortcuts,
		switchState:          make(map[string]int),
		toggleState:          make(map[string]context.CancelFunc),
		tapShortcuts:         tapShortcuts,
		doubleTapShortcuts:   doubleTapShortcuts,
		tapState:             nil, // Set via SetTapState() if needed
		doubleTapState:       nil, // Set via SetDoubleTapState() if needed
	}
}

// SetTapState sets the shared tap state (call after New if tap shortcuts exist)
func (m *Matcher) SetTapState(ts *TapState) {
	m.tapState = ts
}

// SetDoubleTapState sets the shared double-tap state
func (m *Matcher) SetDoubleTapState(ds *DoubleTapState) {
	m.doubleTapState = ds
}

// GetDoubleTapState returns the double-tap state
func (m *Matcher) GetDoubleTapState() *DoubleTapState {
	return m.doubleTapState
}

// HasDoubleTapShortcut checks if this modifier has a double-tap shortcut
func (m *Matcher) HasDoubleTapShortcut(code uint16) bool {
	_, exists := m.doubleTapShortcuts[code]
	return exists
}

// GetDoubleTapShortcut returns the double-tap shortcut for this modifier
func (m *Matcher) GetDoubleTapShortcut(code uint16) *config.ParsedShortcut {
	return m.doubleTapShortcuts[code]
}

// GetShortcuts returns all shortcuts for a combo (including passthrough matches).
func (m *Matcher) GetShortcuts(combo string) []*config.ParsedShortcut {
	var result []*config.ParsedShortcut
	for key, s := range m.shortcuts {
		if key.Combo == combo {
			result = append(result, s)
		}
	}
	baseKey := extractBaseKey(combo)
	for key, s := range m.passthroughShortcuts {
		if key.Combo == baseKey {
			result = append(result, s)
		}
	}
	return result
}

// CheckShortcut checks if a shortcut exists with given combo, behavior, and timing
func (m *Matcher) CheckShortcut(combo string, behavior config.BehaviorMode, timing config.TimingMode) *config.ParsedShortcut {
	key := ShortcutKey{
		Combo:    combo,
		Behavior: behavior,
		Timing:   timing,
	}
	if shortcut := m.shortcuts[key]; shortcut != nil {
		return shortcut
	}

	// Check passthrough shortcuts (strip modifiers, match base key only)
	baseKey := extractBaseKey(combo)
	passthroughKey := ShortcutKey{
		Combo:    baseKey,
		Behavior: behavior,
		Timing:   timing,
	}
	if shortcut := m.passthroughShortcuts[passthroughKey]; shortcut != nil {
		return shortcut
	}

	return nil
}

// extractBaseKey returns the last component of a combo (the non-modifier key)
// Example: "shift+ctrl+kp6" -> "kp6"
func extractBaseKey(combo string) string {
	parts := strings.Split(combo, "+")
	if len(parts) == 0 {
		return combo
	}
	return parts[len(parts)-1]
}

// HasReleaseShortcut checks if any release shortcuts exist for a combo
func (m *Matcher) HasReleaseShortcut(combo string) bool {
	for key := range m.shortcuts {
		if key.Combo == combo && key.Timing == config.TimingRelease {
			return true
		}
	}

	baseKey := extractBaseKey(combo)
	for key := range m.passthroughShortcuts {
		if key.Combo == baseKey && key.Timing == config.TimingRelease {
			return true
		}
	}

	return false
}

// GetCurrentCombo builds the current key combo string
func (m *Matcher) GetCurrentCombo(code uint16) string {
	// Fast path: no modifiers (most common case)
	if !m.state.Super && !m.state.Ctrl && !m.state.Alt && !m.state.Shift {
		return codeToNameMap[code]
	}

	m.comboBuilder.Reset()
	needPlus := false

	if m.state.Super {
		m.comboBuilder.WriteString("super")
		needPlus = true
	}
	if m.state.Ctrl {
		if needPlus {
			m.comboBuilder.WriteByte('+')
		}
		m.comboBuilder.WriteString("ctrl")
		needPlus = true
	}
	if m.state.Alt {
		if needPlus {
			m.comboBuilder.WriteByte('+')
		}
		m.comboBuilder.WriteString("alt")
		needPlus = true
	}
	if m.state.Shift {
		if needPlus {
			m.comboBuilder.WriteByte('+')
		}
		m.comboBuilder.WriteString("shift")
		needPlus = true
	}

	if name := codeToNameMap[code]; name != "" {
		if needPlus {
			m.comboBuilder.WriteByte('+')
		}
		m.comboBuilder.WriteString(name)
	}

	return m.comboBuilder.String()
}

// GetComboCodes returns the keycodes for the current combo as a string like "125+28"
func (m *Matcher) GetComboCodes(code uint16) string {
	// Fast path: no modifiers
	if !m.state.Super && !m.state.Ctrl && !m.state.Alt && !m.state.Shift {
		return fmt.Sprintf("%d", code)
	}

	var codes []string
	if m.state.Super {
		codes = append(codes, fmt.Sprintf("%d", evdev.KEY_LEFTMETA))
	}
	if m.state.Ctrl {
		codes = append(codes, fmt.Sprintf("%d", evdev.KEY_LEFTCTRL))
	}
	if m.state.Alt {
		codes = append(codes, fmt.Sprintf("%d", evdev.KEY_LEFTALT))
	}
	if m.state.Shift {
		codes = append(codes, fmt.Sprintf("%d", evdev.KEY_LEFTSHIFT))
	}
	codes = append(codes, fmt.Sprintf("%d", code))

	result := ""
	for i, c := range codes {
		if i > 0 {
			result += "+"
		}
		result += c
	}
	return result
}

func (m *Matcher) updateModifierState(code uint16, pressed bool) {
	switch code {
	case evdev.KEY_LEFTMETA, evdev.KEY_RIGHTMETA:
		m.state.Super = pressed
	case evdev.KEY_LEFTCTRL, evdev.KEY_RIGHTCTRL:
		m.state.Ctrl = pressed
	case evdev.KEY_LEFTALT, evdev.KEY_RIGHTALT:
		m.state.Alt = pressed
	case evdev.KEY_LEFTSHIFT, evdev.KEY_RIGHTSHIFT:
		m.state.Shift = pressed
	}
}

// UpdateModifierState updates the modifier state (exported for external state tracking)
func (m *Matcher) UpdateModifierState(code uint16, pressed bool) {
	m.updateModifierState(code, pressed)
}

// GetCurrentModifiers returns a copy of current modifier state
func (m *Matcher) GetCurrentModifiers() ModifierState {
	return m.state
}

// MarkTapCandidate sets the tap candidate if this modifier has a tap action
func (m *Matcher) MarkTapCandidate(code uint16) {
	if _, hasTap := m.tapShortcuts[code]; hasTap {
		if m.tapState != nil {
			m.tapState.MarkCandidate(code)
		}
	}
}

// ClearTapCandidate clears the tap candidate (called when combo matches)
func (m *Matcher) ClearTapCandidate() {
	if m.tapState != nil {
		m.tapState.Clear()
	}
}

// CheckTap checks if this modifier release should trigger a tap action
func (m *Matcher) CheckTap(code uint16) (string, bool) {
	if m.tapState != nil && m.tapState.Check(code) {
		if command, ok := m.tapShortcuts[code]; ok {
			m.tapState.Clear()
			return command, true
		}
	}
	return "", false
}

// GetNextSwitchCommand returns the next command in the switch cycle
func (m *Matcher) GetNextSwitchCommand(key string, commands []string) string {
	m.switchMutex.Lock()
	defer m.switchMutex.Unlock()

	idx := m.switchState[key]
	command := commands[idx]
	m.switchState[key] = (idx + 1) % len(commands)
	return command
}

// StartToggleLoop starts a toggle loop
func (m *Matcher) StartToggleLoop(key string, cancel context.CancelFunc) {
	m.toggleMutex.Lock()
	defer m.toggleMutex.Unlock()
	m.toggleState[key] = cancel
}

// StopToggleLoop stops a toggle loop and returns the cancel function
func (m *Matcher) StopToggleLoop(key string) context.CancelFunc {
	m.toggleMutex.Lock()
	defer m.toggleMutex.Unlock()

	if cancel, exists := m.toggleState[key]; exists {
		delete(m.toggleState, key)
		return cancel
	}
	return nil
}

// IsToggleActive checks if a toggle loop is active
func (m *Matcher) IsToggleActive(key string) bool {
	m.toggleMutex.Lock()
	defer m.toggleMutex.Unlock()
	_, exists := m.toggleState[key]
	return exists
}

// IsModifierKey returns true if the key code is a modifier key
func IsModifierKey(code uint16) bool {
	switch code {
	case evdev.KEY_LEFTMETA, evdev.KEY_RIGHTMETA,
		evdev.KEY_LEFTCTRL, evdev.KEY_RIGHTCTRL,
		evdev.KEY_LEFTALT, evdev.KEY_RIGHTALT,
		evdev.KEY_LEFTSHIFT, evdev.KEY_RIGHTSHIFT:
		return true
	}
	return false
}
