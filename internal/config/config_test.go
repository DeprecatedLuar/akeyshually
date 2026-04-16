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
