package matcher

import (
	"strings"

	evdev "github.com/holoplot/go-evdev"
)

type ModifierState struct {
	Super bool
	Ctrl  bool
	Alt   bool
	Shift bool
}

type Matcher struct {
	state     ModifierState
	shortcuts map[string]string
}

func New(shortcuts map[string]string) *Matcher {
	return &Matcher{
		shortcuts: shortcuts,
	}
}

func (m *Matcher) HandleKeyEvent(code uint16, value int32) (string, bool) {
	if value == 1 {
		m.updateModifierState(code, true)
		// fmt.Fprintf(os.Stderr, "[DEBUG] Key pressed: code=%d, state: super=%v ctrl=%v alt=%v shift=%v\n",
		// 	code, m.state.Super, m.state.Ctrl, m.state.Alt, m.state.Shift)
		return m.checkShortcut(code)
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

func (m *Matcher) checkShortcut(code uint16) (string, bool) {
	for shortcut, command := range m.shortcuts {
		if m.matches(shortcut, code) {
			return command, true
		}
	}
	return "", false
}

func (m *Matcher) matches(shortcut string, code uint16) bool {
	parts := strings.Split(strings.ToLower(shortcut), "+")

	expectedState := ModifierState{}
	var keyName string

	for _, part := range parts {
		switch part {
		case "super":
			expectedState.Super = true
		case "ctrl":
			expectedState.Ctrl = true
		case "alt":
			expectedState.Alt = true
		case "shift":
			expectedState.Shift = true
		default:
			keyName = part
		}
	}

	if m.state != expectedState {
		return false
	}

	return getKeyCode(keyName) == code
}

func getKeyCode(name string) uint16 {
	keyMap := map[string]uint16{
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

	if code, ok := keyMap[strings.ToLower(name)]; ok {
		return code
	}
	return 0
}
