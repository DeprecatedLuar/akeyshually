package config

import (
	"testing"
)

func TestAliasGroupParsing(t *testing.T) {
	dst := make(map[string][]*ParsedShortcut)
	err := parseShortcutsInto(dst, "f6/f7/f8.switch", []interface{}{"cmd1", "cmd2", "cmd3"})
	if err != nil {
		t.Fatalf("parseShortcutsInto error: %v", err)
	}

	aliases := []string{"f6", "f7", "f8"}
	for _, combo := range aliases {
		shortcuts, ok := dst[combo]
		if !ok {
			t.Errorf("expected combo %q in map, not found", combo)
			continue
		}
		if len(shortcuts) != 1 {
			t.Errorf("combo %q: expected 1 shortcut, got %d", combo, len(shortcuts))
			continue
		}
		s := shortcuts[0]
		if s.AliasGroup != "f6/f7/f8.switch" {
			t.Errorf("combo %q: AliasGroup = %q, want %q", combo, s.AliasGroup, "f6/f7/f8.switch")
		}
		if s.Behavior != BehaviorSwitch {
			t.Errorf("combo %q: Behavior = %v, want BehaviorSwitch", combo, s.Behavior)
		}
	}
}

func TestNonAliasSwitchHasNoGroup(t *testing.T) {
	dst := make(map[string][]*ParsedShortcut)
	err := parseShortcutsInto(dst, "f1.switch", []interface{}{"cmd1", "cmd2"})
	if err != nil {
		t.Fatalf("parseShortcutsInto error: %v", err)
	}
	shortcuts, ok := dst["f1"]
	if !ok {
		t.Fatal("expected f1 in map")
	}
	if shortcuts[0].AliasGroup != "" {
		t.Errorf("non-alias AliasGroup should be empty, got %q", shortcuts[0].AliasGroup)
	}
}

func TestMergeDeduplicatesDevices(t *testing.T) {
	base := &Config{
		Shortcuts:       map[string]interface{}{"f1": "echo base"},
		Commands:        make(map[string]string),
		ParsedShortcuts: make(map[string][]*ParsedShortcut),
		Settings:        Settings{Devices: []string{"Huion"}},
	}

	overlay := &Config{
		Shortcuts: make(map[string]interface{}),
		Commands:  make(map[string]string),
		Settings:  Settings{Devices: []string{"huion", "Xbox Controller"}}, // "huion" duplicates "Huion"
	}

	base.Merge(overlay)

	if len(base.Settings.Devices) != 2 {
		t.Errorf("expected 2 devices after merge, got %d: %v", len(base.Settings.Devices), base.Settings.Devices)
	}
}

func TestRemapParsing(t *testing.T) {
	dst := make(map[string][]*ParsedShortcut)
	if err := parseShortcutsInto(dst, "btn_0", ">ctrl+z"); err != nil {
		t.Fatalf("remap shortcut should parse without error: %v", err)
	}
	s := dst["btn_0"][0]
	if len(s.Commands) != 1 || s.Commands[0] != ">ctrl+z" {
		t.Errorf("Commands = %v, want [\">ctrl+z\"]", s.Commands)
	}
}

func TestDevicesFieldParses(t *testing.T) {
	dst := make(map[string][]*ParsedShortcut)
	err := parseShortcutsInto(dst, "btn_south", "notify-send test")
	if err != nil {
		t.Fatalf("btn_south should parse without error: %v", err)
	}
	shortcuts, ok := dst["btn_south"]
	if !ok {
		t.Fatal("expected btn_south in parsed shortcuts")
	}
	if shortcuts[0].KeyCombo != "btn_south" {
		t.Errorf("KeyCombo = %q, want %q", shortcuts[0].KeyCombo, "btn_south")
	}
}

func TestPressReleaseParsing(t *testing.T) {
	ps, err := ParseShortcut("super+m.pressrelease", []interface{}{"mic-on", "mic-off"})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if ps.Behavior != BehaviorPressRelease {
		t.Errorf("Behavior = %v, want BehaviorPressRelease", ps.Behavior)
	}
	if len(ps.Commands) != 2 || ps.Commands[0] != "mic-on" || ps.Commands[1] != "mic-off" {
		t.Errorf("Commands = %v, want [mic-on mic-off]", ps.Commands)
	}
}

func TestPressReleaseSingleCommandRejected(t *testing.T) {
	err := validateShortcutEntry("super+m.pressrelease", "mic-on", "test.toml", 0)
	if err == nil {
		t.Fatal("expected error for single command, got nil")
	}
}

func TestHoldBehavior(t *testing.T) {
	ps, err := ParseShortcut("super+h.hold", "mute")
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if ps.Behavior != BehaviorHold {
		t.Errorf("Behavior = %v, want BehaviorHold", ps.Behavior)
	}
}

func TestHoldTwoCommandsRejected(t *testing.T) {
	err := validateShortcutEntry("super+h.hold", []interface{}{"start", "stop"}, "test.toml", 0)
	if err == nil {
		t.Fatal("expected error for 2-command hold, got nil")
	}
}

func TestHoldReleaseBehavior(t *testing.T) {
	ps, err := ParseShortcut("super+h.holdrelease", []interface{}{"mic-on", "mic-off"})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if ps.Behavior != BehaviorHoldRelease {
		t.Errorf("Behavior = %v, want BehaviorHoldRelease", ps.Behavior)
	}
	if len(ps.Commands) != 2 {
		t.Errorf("Commands len = %d, want 2", len(ps.Commands))
	}
}

func TestHoldReleaseEmptyFirstCommand(t *testing.T) {
	ps, err := ParseShortcut("super+h.holdrelease", []interface{}{"", "mic-off"})
	if err != nil {
		t.Fatalf("expected success with empty first command, got: %v", err)
	}
	if ps.Commands[0] != "" {
		t.Errorf("Commands[0] = %q, want empty string", ps.Commands[0])
	}
}

func TestHoldReleaseWithInterval(t *testing.T) {
	ps, err := ParseShortcut("super+h.holdrelease(500)", []interface{}{"start", "stop"})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if ps.Interval != 500 {
		t.Errorf("Interval = %v, want 500", ps.Interval)
	}
}

func TestPressReleaseEmptyFirstCommand(t *testing.T) {
	ps, err := ParseShortcut("super.pressrelease", []interface{}{"", "rofi"})
	if err != nil {
		t.Fatalf("expected success with empty first command, got: %v", err)
	}
	if ps.Commands[0] != "" || ps.Commands[1] != "rofi" {
		t.Errorf("Commands = %v, want [\"\", \"rofi\"]", ps.Commands)
	}
}

func TestLongPressBehavior(t *testing.T) {
	ps, err := ParseShortcut("super+h.longpress(200)", "shutdown")
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if ps.Behavior != BehaviorLongPress {
		t.Errorf("Behavior = %v, want BehaviorLongPress", ps.Behavior)
	}
	if ps.Interval != 200 {
		t.Errorf("Interval = %v, want 200", ps.Interval)
	}
}

func TestHoldRepeat(t *testing.T) {
	ps, err := ParseShortcut("super+h.hold.repeat", "scroll")
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if ps.Behavior != BehaviorHold {
		t.Errorf("Behavior = %v, want BehaviorHold", ps.Behavior)
	}
	if !ps.Repeat {
		t.Errorf("Repeat = false, want true")
	}
}

func TestOnPressRepeat(t *testing.T) {
	ps, err := ParseShortcut("super+r.onpress.repeat", "scroll")
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if ps.Behavior != BehaviorNormal {
		t.Errorf("Behavior = %v, want BehaviorNormal", ps.Behavior)
	}
	if ps.Timing != TimingPress {
		t.Errorf("Timing = %v, want TimingPress", ps.Timing)
	}
	if !ps.Repeat {
		t.Errorf("Repeat = false, want true")
	}
}

func TestTapHoldBehavior(t *testing.T) {
	ps, err := ParseShortcut("super+t.taphold", "hold-cmd")
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if ps.Behavior != BehaviorTapHold {
		t.Errorf("Behavior = %v, want BehaviorTapHold", ps.Behavior)
	}
	if len(ps.Commands) != 1 {
		t.Errorf("Commands len = %d, want 1", len(ps.Commands))
	}
}

func TestTapLongPressBehavior(t *testing.T) {
	ps, err := ParseShortcut("super+t.taplongpress", []interface{}{"tap-cmd", "longpress-cmd"})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if ps.Behavior != BehaviorTapLongPress {
		t.Errorf("Behavior = %v, want BehaviorTapLongPress", ps.Behavior)
	}
	if len(ps.Commands) != 2 {
		t.Errorf("Commands len = %d, want 2", len(ps.Commands))
	}
}

func TestDoubleTapSwitchParsing(t *testing.T) {
	ps, err := ParseShortcut("f1.doubletap", "notify")
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if ps.Behavior != BehaviorDoubleTap {
		t.Errorf("Behavior = %v, want BehaviorDoubleTap", ps.Behavior)
	}
}

func TestLongPressRepeatRejected(t *testing.T) {
	err := validateShortcutEntry("super+h.longpress.repeat", "cmd", "test.toml", 0)
	if err == nil {
		t.Fatal("expected error for longpress.repeat, got nil")
	}
}

func TestRemapTap(t *testing.T) {
	ps, err := ParseShortcut("btn_0", ">ctrl+z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ps.Commands) != 1 || ps.Commands[0] != ">ctrl+z" {
		t.Errorf("Commands = %v, want [\">ctrl+z\"]", ps.Commands)
	}
}

func TestRemapHoldForever(t *testing.T) {
	ps, err := ParseShortcut("btn_0", ">>b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ps.Commands) != 1 || ps.Commands[0] != ">>b" {
		t.Errorf("Commands = %v, want [\">>b\"]", ps.Commands)
	}
}

func TestRemapKeyUp(t *testing.T) {
	ps, err := ParseShortcut("btn_1", "<k")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ps.Commands) != 1 || ps.Commands[0] != "<k" {
		t.Errorf("Commands = %v, want [\"<k\"]", ps.Commands)
	}
}

func TestRemapReleaseAll(t *testing.T) {
	ps, err := ParseShortcut("btn_2", "<<")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ps.Commands) != 1 || ps.Commands[0] != "<<" {
		t.Errorf("Commands = %v, want [\"<<\"]", ps.Commands)
	}
}
