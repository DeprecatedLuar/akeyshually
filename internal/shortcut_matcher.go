package internal

import (
	"context"
	"strings"
	"sync"

	"github.com/deprecatedluar/akeyshually/internal/config"

	evdev "github.com/holoplot/go-evdev"
)

var keyCodeMap = map[string]uint16{
	"a": evdev.KEY_A, "b": evdev.KEY_B, "c": evdev.KEY_C, "d": evdev.KEY_D,
	"e": evdev.KEY_E, "f": evdev.KEY_F, "g": evdev.KEY_G, "h": evdev.KEY_H,
	"i": evdev.KEY_I, "j": evdev.KEY_J, "k": evdev.KEY_K, "l": evdev.KEY_L,
	"m": evdev.KEY_M, "n": evdev.KEY_N, "o": evdev.KEY_O, "p": evdev.KEY_P,
	"q": evdev.KEY_Q, "r": evdev.KEY_R, "s": evdev.KEY_S, "t": evdev.KEY_T,
	"u": evdev.KEY_U, "v": evdev.KEY_V, "w": evdev.KEY_W, "x": evdev.KEY_X,
	"y": evdev.KEY_Y, "z": evdev.KEY_Z,
	"1": evdev.KEY_1, "2": evdev.KEY_2, "3": evdev.KEY_3, "4": evdev.KEY_4,
	"5": evdev.KEY_5, "6": evdev.KEY_6, "7": evdev.KEY_7, "8": evdev.KEY_8,
	"9": evdev.KEY_9, "0": evdev.KEY_0,
	"return": evdev.KEY_ENTER, "enter": evdev.KEY_ENTER,
	"space": evdev.KEY_SPACE, "tab": evdev.KEY_TAB,
	"esc": evdev.KEY_ESC, "escape": evdev.KEY_ESC,
	"backspace": evdev.KEY_BACKSPACE,
	"print": evdev.KEY_SYSRQ, "printscreen": evdev.KEY_SYSRQ,
	"f1": evdev.KEY_F1, "f2": evdev.KEY_F2, "f3": evdev.KEY_F3, "f4": evdev.KEY_F4,
	"f5": evdev.KEY_F5, "f6": evdev.KEY_F6, "f7": evdev.KEY_F7, "f8": evdev.KEY_F8,
	"f9": evdev.KEY_F9, "f10": evdev.KEY_F10, "f11": evdev.KEY_F11, "f12": evdev.KEY_F12,
	"f13": evdev.KEY_F13, "f14": evdev.KEY_F14, "f15": evdev.KEY_F15, "f16": evdev.KEY_F16,
	"f17": evdev.KEY_F17, "f18": evdev.KEY_F18, "f19": evdev.KEY_F19, "f20": evdev.KEY_F20,
	"f21": evdev.KEY_F21, "f22": evdev.KEY_F22, "f23": evdev.KEY_F23, "f24": evdev.KEY_F24,
	"left": evdev.KEY_LEFT, "right": evdev.KEY_RIGHT,
	"up": evdev.KEY_UP, "down": evdev.KEY_DOWN,
	"home": evdev.KEY_HOME, "end": evdev.KEY_END,
	"pageup": evdev.KEY_PAGEUP, "pagedown": evdev.KEY_PAGEDOWN,
	"delete": evdev.KEY_DELETE, "insert": evdev.KEY_INSERT,
	"volumeup": evdev.KEY_VOLUMEUP, "volumedown": evdev.KEY_VOLUMEDOWN,
	"mute": evdev.KEY_MUTE,
	"brightnessup": evdev.KEY_BRIGHTNESSUP, "brightnessdown": evdev.KEY_BRIGHTNESSDOWN,
	"playpause": evdev.KEY_PLAYPAUSE, "play": evdev.KEY_PLAYPAUSE,
	"nextsong": evdev.KEY_NEXTSONG, "next": evdev.KEY_NEXTSONG,
	"previoussong": evdev.KEY_PREVIOUSSONG, "previous": evdev.KEY_PREVIOUSSONG,
}

// Reverse lookup map for O(1) code -> name lookups
var codeToNameMap map[uint16]string

func init() {
	codeToNameMap = make(map[uint16]string, len(keyCodeMap))

	// Build reverse lookup from keyCodeMap
	for name, code := range keyCodeMap {
		codeToNameMap[code] = name
	}

	// Override with canonical names (preferred)
	canonicalOverrides := map[uint16]string{
		evdev.KEY_ENTER: "return",
		evdev.KEY_ESC:   "esc",
		evdev.KEY_SYSRQ: "print",
	}
	for code, name := range canonicalOverrides {
		codeToNameMap[code] = name
	}
}

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

// Check returns the current candidate and clears it
func (ts *TapState) Check(code uint16) bool {
	ts.RLock()
	defer ts.RUnlock()
	return ts.candidate == code
}

type Matcher struct {
	state     ModifierState
	shortcuts map[ShortcutKey]*config.ParsedShortcut

	// Switch state (cycle through commands)
	switchState map[string]int // "super+k.switch.press" -> next index
	switchMutex sync.Mutex

	// Toggle state (loop on/off)
	toggleState map[string]context.CancelFunc // "super+k.toggle.press" -> cancel func
	toggleMutex sync.Mutex

	// Tap shortcuts (lone modifiers with .onrelease)
	tapShortcuts map[uint16]string

	// Shared tap state (for mouse cancellation)
	tapState *TapState
}

func New(parsedShortcuts map[string][]*config.ParsedShortcut) *Matcher {
	shortcuts := make(map[ShortcutKey]*config.ParsedShortcut)
	tapShortcuts := make(map[uint16]string)

	// Build shortcut lookup map and extract tap shortcuts
	for _, shortcutList := range parsedShortcuts {
		for _, shortcut := range shortcutList {
			key := ShortcutKey{
				Combo:    shortcut.KeyCombo,
				Behavior: shortcut.Behavior,
				Timing:   shortcut.Timing,
			}
			shortcuts[key] = shortcut

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
		shortcuts:    shortcuts,
		switchState:  make(map[string]int),
		toggleState:  make(map[string]context.CancelFunc),
		tapShortcuts: tapShortcuts,
		tapState:     nil, // Set via SetTapState() if needed
	}
}

// SetTapState sets the shared tap state (call after New if tap shortcuts exist)
func (m *Matcher) SetTapState(ts *TapState) {
	m.tapState = ts
}

// CheckShortcut checks if a shortcut exists with given combo, behavior, and timing
func (m *Matcher) CheckShortcut(combo string, behavior config.BehaviorMode, timing config.TimingMode) *config.ParsedShortcut {
	key := ShortcutKey{
		Combo:    combo,
		Behavior: behavior,
		Timing:   timing,
	}
	return m.shortcuts[key]
}

// HasReleaseShortcut checks if any release shortcuts exist for a combo
func (m *Matcher) HasReleaseShortcut(combo string) bool {
	for key := range m.shortcuts {
		if key.Combo == combo && key.Timing == config.TimingRelease {
			return true
		}
	}
	return false
}

// GetCurrentCombo builds the current key combo string
func (m *Matcher) GetCurrentCombo(code uint16) string {
	// Fast path: no modifiers pressed
	if !m.state.Super && !m.state.Ctrl && !m.state.Alt && !m.state.Shift {
		return codeToNameMap[code]
	}

	// Build combo from current state (pre-allocate for up to 5 parts: 4 modifiers + 1 key)
	parts := make([]string, 0, 5)
	if m.state.Super {
		parts = append(parts, "super")
	}
	if m.state.Ctrl {
		parts = append(parts, "ctrl")
	}
	if m.state.Alt {
		parts = append(parts, "alt")
	}
	if m.state.Shift {
		parts = append(parts, "shift")
	}

	// Find key name using O(1) lookup
	if keyName := codeToNameMap[code]; keyName != "" {
		parts = append(parts, keyName)
	}

	return strings.Join(parts, "+")
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

func getKeyCode(name string) uint16 {
	if code, ok := keyCodeMap[strings.ToLower(name)]; ok {
		return code
	}
	return 0
}

func getKeyName(code uint16) string {
	return codeToNameMap[code]
}
