package keys

import (
	"fmt"
	"strings"
	"testing"

	evdev "github.com/holoplot/go-evdev"
)

func TestGetAbsNameKnown(t *testing.T) {
	for code, expected := range AbsCodeNames {
		got := GetAbsName(code)
		if got != expected {
			t.Errorf("GetAbsName(%d) = %q, want %q", code, got, expected)
		}
	}
}

func TestGetAbsNameUnknown(t *testing.T) {
	got := GetAbsName(9999)
	want := "ABS_9999"
	if got != want {
		t.Errorf("GetAbsName(9999) = %q, want %q", got, want)
	}
	if !strings.HasPrefix(got, "ABS_") {
		t.Errorf("unknown code should return ABS_N format, got %q", got)
	}
}

func TestBtnKeysInKeyCodeMap(t *testing.T) {
	keyNames := []string{
		"btn_0", "btn_1", "btn_2", "btn_3", "btn_4",
		"btn_5", "btn_6", "btn_7", "btn_8", "btn_9",
		"btn_south", "btn_north", "btn_east", "btn_west",
		"btn_tl", "btn_tr", "btn_tl2", "btn_tr2",
		"btn_start", "btn_select", "btn_mode",
		"btn_thumbl", "btn_thumbr",
		"btn_dpad_up", "btn_dpad_down", "btn_dpad_left", "btn_dpad_right",
		"btn_tool_pen", "btn_touch", "btn_stylus", "btn_stylus2",
	}

	for _, key := range keyNames {
		code, ok := ResolveKeyCode(key)
		if !ok || code == 0 {
			t.Errorf("key %q not in keyCodeMap", key)
			continue
		}
		name := GetKeyName(code)
		if name == "" {
			t.Errorf("code for %q (%d) has no reverse name mapping", key, code)
		}
	}
}

func TestGamepadHomeAliases(t *testing.T) {
	for _, name := range []string{"btn_mode", "gp_guide", "gp_home"} {
		code, ok := ResolveKeyCode(name)
		if !ok {
			t.Errorf("gamepad home alias %q not in KeyCodeMap", name)
			continue
		}
		if code != uint16(evdev.BTN_MODE) {
			t.Errorf("gamepad home alias %q resolved to %d, want BTN_MODE", name, code)
		}
	}
}

func TestGamepadDpadKeys(t *testing.T) {
	tests := map[string]uint16{
		"btn_dpad_up":    uint16(evdev.BTN_DPAD_UP),
		"btn_dpad_down":  uint16(evdev.BTN_DPAD_DOWN),
		"btn_dpad_left":  uint16(evdev.BTN_DPAD_LEFT),
		"btn_dpad_right": uint16(evdev.BTN_DPAD_RIGHT),
		"gp_up":          uint16(evdev.BTN_DPAD_UP),
		"gp_down":        uint16(evdev.BTN_DPAD_DOWN),
		"gp_left":        uint16(evdev.BTN_DPAD_LEFT),
		"gp_right":       uint16(evdev.BTN_DPAD_RIGHT),
	}
	for name, want := range tests {
		code, ok := ResolveKeyCode(name)
		if !ok {
			t.Errorf("D-pad key %q not in KeyCodeMap", name)
			continue
		}
		if code != want {
			t.Errorf("D-pad key %q resolved to %d, want %d", name, code, want)
		}
	}
}

func TestBtnKeysNoDuplicateCodes(t *testing.T) {
	btnKeys := []string{
		"btn_0", "btn_1", "btn_2", "btn_3", "btn_4",
		"btn_5", "btn_6", "btn_7", "btn_8", "btn_9",
		"btn_south", "btn_north", "btn_east", "btn_west",
		"btn_tl", "btn_tr", "btn_tl2", "btn_tr2",
		"btn_start", "btn_select", "btn_mode",
		"btn_thumbl", "btn_thumbr",
		"btn_dpad_up", "btn_dpad_down", "btn_dpad_left", "btn_dpad_right",
		"btn_tool_pen", "btn_touch", "btn_stylus", "btn_stylus2",
	}

	seen := make(map[uint16]string)
	for _, key := range btnKeys {
		code, ok := ResolveKeyCode(key)
		if !ok || code == 0 {
			continue
		}
		if prev, exists := seen[code]; exists {
			t.Errorf("code %d shared by %q and %q", code, prev, key)
		}
		seen[code] = key
	}
}

func TestGetAbsNameFallbackFormat(t *testing.T) {
	for _, code := range []uint16{100, 200, 500} {
		got := GetAbsName(code)
		want := fmt.Sprintf("ABS_%d", code)
		if got != want {
			t.Errorf("GetAbsName(%d) = %q, want %q", code, got, want)
		}
	}
}

func TestStickAxisAliasesAreUnambiguous(t *testing.T) {
	tests := map[string]uint16{
		"lx": uint16(evdev.ABS_X),
		"ly": uint16(evdev.ABS_Y),
		"rx": uint16(evdev.ABS_RX),
		"ry": uint16(evdev.ABS_RY),
	}
	for name, want := range tests {
		code, ok := ResolveAbsCode(name)
		if !ok || code != want {
			t.Errorf("ResolveAbsCode(%q) = (%d, %v), want (%d, true)", name, code, ok, want)
		}
	}
	for _, ambiguous := range []string{"x", "y", "z"} {
		if _, ok := ResolveAbsCode(ambiguous); ok {
			t.Errorf("ambiguous keyboard key %q resolved as an axis", ambiguous)
		}
	}
}
