package matcher

import (
	"fmt"
	"strings"
	"sync"

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

type Matcher struct {
	state     ModifierState
	shortcuts map[ShortcutKey]*config.ParsedShortcut

	// Passthrough shortcuts (indexed by base key only, no modifiers)
	passthroughShortcuts map[ShortcutKey]*config.ParsedShortcut

	// Switch state (cycle through commands)
	switchState map[string]int // "super+k.switch.press" -> next index
	switchMutex sync.Mutex

	// Tap shortcuts (lone modifiers with .onrelease)
	tapShortcuts map[uint16]string

	// Shared tap state (for mouse cancellation)
	tapState *TapState

	// Reusable string builder (avoids allocations in hot path)
	comboBuilder strings.Builder
}

func New(parsedShortcuts map[string][]*config.ParsedShortcut) *Matcher {
	shortcuts := make(map[ShortcutKey]*config.ParsedShortcut)
	passthroughShortcuts := make(map[ShortcutKey]*config.ParsedShortcut)
	tapShortcuts := make(map[uint16]string)

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
		}
	}

	return &Matcher{
		shortcuts:            shortcuts,
		passthroughShortcuts: passthroughShortcuts,
		switchState:          make(map[string]int),
		tapShortcuts:         tapShortcuts,
		tapState:             nil, // Set via SetTapState() if needed
	}
}

// SetTapState sets the shared tap state (call after New if tap shortcuts exist)
func (m *Matcher) SetTapState(ts *TapState) {
	m.tapState = ts
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

// extractBaseKey returns the last component of a combo (the non-modifier key)
// Example: "shift+ctrl+kp6" -> "kp6"
func extractBaseKey(combo string) string {
	parts := strings.Split(combo, "+")
	if len(parts) == 0 {
		return combo
	}
	return parts[len(parts)-1]
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
