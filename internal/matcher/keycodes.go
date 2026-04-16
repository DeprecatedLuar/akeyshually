package matcher

import (
	"fmt"

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
	"semicolon": evdev.KEY_SEMICOLON, ";": evdev.KEY_SEMICOLON,
	"ro":    evdev.KEY_RO,
	"102nd": evdev.KEY_102ND,
	"kp0": evdev.KEY_KP0, "kp1": evdev.KEY_KP1, "kp2": evdev.KEY_KP2, "kp3": evdev.KEY_KP3,
	"kp4": evdev.KEY_KP4, "kp5": evdev.KEY_KP5, "kp6": evdev.KEY_KP6, "kp7": evdev.KEY_KP7,
	"kp8": evdev.KEY_KP8, "kp9": evdev.KEY_KP9,
	"kpplus": evdev.KEY_KPPLUS, "kpminus": evdev.KEY_KPMINUS,
	"kpasterisk": evdev.KEY_KPASTERISK, "kpslash": evdev.KEY_KPSLASH,
	"kpenter": evdev.KEY_KPENTER, "kpdot": evdev.KEY_KPDOT,
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
	"mute":          evdev.KEY_MUTE,
	"brightnessup":  evdev.KEY_BRIGHTNESSUP, "brightnessdown": evdev.KEY_BRIGHTNESSDOWN,
	"playpause":     evdev.KEY_PLAYPAUSE, "play": evdev.KEY_PLAYPAUSE,
	"nextsong":      evdev.KEY_NEXTSONG, "next": evdev.KEY_NEXTSONG,
	"previoussong":  evdev.KEY_PREVIOUSSONG, "previous": evdev.KEY_PREVIOUSSONG,
	"calc":          evdev.KEY_CALC, "calculator": evdev.KEY_CALC,
	// Generic/tablet buttons (BTN_0..BTN_9)
	"btn_0": evdev.BTN_0, "btn_1": evdev.BTN_1, "btn_2": evdev.BTN_2, "btn_3": evdev.BTN_3,
	"btn_4": evdev.BTN_4, "btn_5": evdev.BTN_5, "btn_6": evdev.BTN_6, "btn_7": evdev.BTN_7,
	"btn_8": evdev.BTN_8, "btn_9": evdev.BTN_9,
	// Gamepad face buttons
	"btn_south": evdev.BTN_SOUTH, "btn_north": evdev.BTN_NORTH,
	"btn_east": evdev.BTN_EAST, "btn_west": evdev.BTN_WEST,
	// Gamepad shoulder buttons
	"btn_tl": evdev.BTN_TL, "btn_tr": evdev.BTN_TR,
	"btn_tl2": evdev.BTN_TL2, "btn_tr2": evdev.BTN_TR2,
	// Gamepad system buttons
	"btn_start": evdev.BTN_START, "btn_select": evdev.BTN_SELECT, "btn_mode": evdev.BTN_MODE,
	// Gamepad analog stick buttons
	"btn_thumbl": evdev.BTN_THUMBL, "btn_thumbr": evdev.BTN_THUMBR,
	// Tablet pen buttons
	"btn_tool_pen": evdev.BTN_TOOL_PEN, "btn_touch": evdev.BTN_TOUCH,
	"btn_stylus": evdev.BTN_STYLUS, "btn_stylus2": evdev.BTN_STYLUS2,
}

// codeToNameMap is a reverse lookup map for O(1) code -> name lookups
var codeToNameMap map[uint16]string

// absCodeNames maps EV_ABS codes to human-readable names (used for debug logging only)
var absCodeNames = map[uint16]string{
	evdev.ABS_X: "ABS_X", evdev.ABS_Y: "ABS_Y", evdev.ABS_Z: "ABS_Z",
	evdev.ABS_RX: "ABS_RX", evdev.ABS_RY: "ABS_RY", evdev.ABS_RZ: "ABS_RZ",
	evdev.ABS_THROTTLE: "ABS_THROTTLE", evdev.ABS_RUDDER: "ABS_RUDDER",
	evdev.ABS_WHEEL: "ABS_WHEEL", evdev.ABS_GAS: "ABS_GAS", evdev.ABS_BRAKE: "ABS_BRAKE",
	evdev.ABS_HAT0X: "ABS_HAT0X", evdev.ABS_HAT0Y: "ABS_HAT0Y",
	evdev.ABS_PRESSURE: "ABS_PRESSURE", evdev.ABS_DISTANCE: "ABS_DISTANCE",
	evdev.ABS_TILT_X: "ABS_TILT_X", evdev.ABS_TILT_Y: "ABS_TILT_Y",
	evdev.ABS_MISC: "ABS_MISC",
}

func init() {
	codeToNameMap = make(map[uint16]string, len(keyCodeMap))

	for name, code := range keyCodeMap {
		codeToNameMap[code] = name
	}

	// Override with canonical names (preferred) and add modifiers
	canonicalOverrides := map[uint16]string{
		evdev.KEY_ENTER:        "return",
		evdev.KEY_ESC:          "esc",
		evdev.KEY_SYSRQ:        "print",
		evdev.KEY_LEFTMETA:     "super",
		evdev.KEY_RIGHTMETA:    "super",
		evdev.KEY_LEFTCTRL:     "ctrl",
		evdev.KEY_RIGHTCTRL:    "ctrl",
		evdev.KEY_LEFTALT:      "alt",
		evdev.KEY_RIGHTALT:     "alt",
		evdev.KEY_LEFTSHIFT:    "shift",
		evdev.KEY_RIGHTSHIFT:   "shift",
		evdev.KEY_PLAYPAUSE:    "playpause",
		evdev.KEY_NEXTSONG:     "nextsong",
		evdev.KEY_PREVIOUSSONG: "previoussong",
		evdev.KEY_CALC:         "calc",
		// Gamepad/tablet canonical names
		evdev.BTN_0: "btn_0", evdev.BTN_1: "btn_1", evdev.BTN_2: "btn_2", evdev.BTN_3: "btn_3",
		evdev.BTN_4: "btn_4", evdev.BTN_5: "btn_5", evdev.BTN_6: "btn_6", evdev.BTN_7: "btn_7",
		evdev.BTN_8: "btn_8", evdev.BTN_9: "btn_9",
		evdev.BTN_SOUTH: "btn_south", evdev.BTN_NORTH: "btn_north",
		evdev.BTN_EAST: "btn_east", evdev.BTN_WEST: "btn_west",
		evdev.BTN_TL: "btn_tl", evdev.BTN_TR: "btn_tr",
		evdev.BTN_TL2: "btn_tl2", evdev.BTN_TR2: "btn_tr2",
		evdev.BTN_START: "btn_start", evdev.BTN_SELECT: "btn_select", evdev.BTN_MODE: "btn_mode",
		evdev.BTN_THUMBL: "btn_thumbl", evdev.BTN_THUMBR: "btn_thumbr",
		evdev.BTN_TOOL_PEN: "btn_tool_pen", evdev.BTN_TOUCH: "btn_touch",
		evdev.BTN_STYLUS: "btn_stylus", evdev.BTN_STYLUS2: "btn_stylus2",
	}
	for code, name := range canonicalOverrides {
		codeToNameMap[code] = name
	}
}

// GetAbsName returns a human-readable name for an EV_ABS axis code
func GetAbsName(code uint16) string {
	if name, ok := absCodeNames[code]; ok {
		return name
	}
	return fmt.Sprintf("ABS_%d", code)
}

func getKeyCode(name string) uint16 {
	if code, ok := keyCodeMap[name]; ok {
		return code
	}
	return 0
}

// GetKeyName returns the canonical name for a key code
func GetKeyName(code uint16) string {
	return codeToNameMap[code]
}
