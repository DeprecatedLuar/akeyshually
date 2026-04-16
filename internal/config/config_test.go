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
	if !s.IsRemap {
		t.Errorf("IsRemap = false, want true")
	}
	if s.RemapCombo != "ctrl+z" {
		t.Errorf("RemapCombo = %q, want %q", s.RemapCombo, "ctrl+z")
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
