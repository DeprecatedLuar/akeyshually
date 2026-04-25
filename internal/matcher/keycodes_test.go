package matcher

import (
	"fmt"
	"strings"
	"testing"

	"github.com/deprecatedluar/akeyshually/internal/keys"
)

func TestGetAbsNameKnown(t *testing.T) {
	for code, expected := range keys.AbsCodeNames {
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

func TestBtnKeysNoDuplicateCodes(t *testing.T) {
	btnKeys := []string{
		"btn_0", "btn_1", "btn_2", "btn_3", "btn_4",
		"btn_5", "btn_6", "btn_7", "btn_8", "btn_9",
		"btn_south", "btn_north", "btn_east", "btn_west",
		"btn_tl", "btn_tr", "btn_tl2", "btn_tr2",
		"btn_start", "btn_select", "btn_mode",
		"btn_thumbl", "btn_thumbr",
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
