package matcher

import (
	"strings"

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

type ModifierState struct {
	Super bool
	Ctrl  bool
	Alt   bool
	Shift bool
}

type ParsedShortcut struct {
	State   ModifierState
	KeyCode uint16
	Command string
}

type Matcher struct {
	state     ModifierState
	shortcuts []ParsedShortcut
}

func New(shortcuts map[string]string) *Matcher {
	parsed := make([]ParsedShortcut, 0, len(shortcuts))

	for shortcut, command := range shortcuts {
		parts := strings.Split(strings.ToLower(shortcut), "+")

		state := ModifierState{}
		var keyName string

		for _, part := range parts {
			switch part {
			case "super":
				state.Super = true
			case "ctrl":
				state.Ctrl = true
			case "alt":
				state.Alt = true
			case "shift":
				state.Shift = true
			default:
				keyName = part
			}
		}

		keyCode := getKeyCode(keyName)
		if keyCode != 0 {
			parsed = append(parsed, ParsedShortcut{
				State:   state,
				KeyCode: keyCode,
				Command: command,
			})
		}
	}

	return &Matcher{
		shortcuts: parsed,
	}
}

func (m *Matcher) HandleKeyEvent(code uint16, value int32) (string, bool) {
	if value == 1 {
		m.updateModifierState(code, true)
		command, matched := m.checkShortcut(code)
		if matched {
			return command, true
		}
		return "", false
	} else if value == 0 {
		m.updateModifierState(code, false)
	}
	return "", false
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

// WouldMatch checks if a key code with current modifiers would match a shortcut
func (m *Matcher) WouldMatch(code uint16) (string, bool) {
	return m.checkShortcut(code)
}

// GetCurrentModifiers returns a copy of current modifier state
func (m *Matcher) GetCurrentModifiers() ModifierState {
	return m.state
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

func (m *Matcher) checkShortcut(code uint16) (string, bool) {
	for _, shortcut := range m.shortcuts {
		if m.state == shortcut.State && code == shortcut.KeyCode {
			return shortcut.Command, true
		}
	}
	return "", false
}

func getKeyCode(name string) uint16 {
	if code, ok := keyCodeMap[strings.ToLower(name)]; ok {
		return code
	}
	return 0
}
