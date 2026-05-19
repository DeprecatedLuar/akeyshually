package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVirtualKeysIntegration(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.toml")

	configContent := `[virtual_keys]
action = ["playcd", "pausecd"]
media = "playpause"

[shortcuts]
"action" = "echo 'Quick press'"
"action.hold" = "echo 'Holding media key'"
"super+media" = "echo 'Super + media key'"
"super+k" = "echo 'Regular shortcut'"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// Load config
	cfg, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify expanded shortcuts
	expectedShortcuts := map[string]bool{
		"playcd":        true,
		"pausecd":       true,
		"playcd.hold":   true,
		"pausecd.hold":  true,
		"super+playpause": true,
		"super+k":       true,
	}

	for key := range expectedShortcuts {
		if _, ok := cfg.Shortcuts[key]; !ok {
			t.Errorf("Expected shortcut %q not found in expanded shortcuts", key)
		}
	}

	// Verify original virtual key references are gone
	unexpectedShortcuts := []string{
		"action",
		"action.hold",
		"super+media",
	}

	for _, key := range unexpectedShortcuts {
		if _, ok := cfg.Shortcuts[key]; ok {
			t.Errorf("Virtual key reference %q should have been expanded", key)
		}
	}

	// Verify parsed shortcuts work correctly
	// playcd should have 2 behaviors: normal (onpress) and hold
	if shortcuts, ok := cfg.ParsedShortcuts["playcd"]; !ok {
		t.Error("playcd shortcut not found in ParsedShortcuts")
	} else if len(shortcuts) != 2 {
		t.Errorf("Expected 2 parsed shortcuts for playcd (normal + hold), got %d", len(shortcuts))
	} else {
		// Verify we have both normal and hold behaviors
		behaviors := make(map[BehaviorMode]bool)
		for _, s := range shortcuts {
			behaviors[s.Behavior] = true
		}
		if !behaviors[BehaviorNormal] {
			t.Error("Expected normal behavior for playcd")
		}
		if !behaviors[BehaviorHold] {
			t.Error("Expected hold behavior for playcd")
		}
	}

	// pausecd should also have 2 behaviors
	if shortcuts, ok := cfg.ParsedShortcuts["pausecd"]; !ok {
		t.Error("pausecd shortcut not found in ParsedShortcuts")
	} else if len(shortcuts) != 2 {
		t.Errorf("Expected 2 parsed shortcuts for pausecd (normal + hold), got %d", len(shortcuts))
	}

	if shortcuts, ok := cfg.ParsedShortcuts["super+playpause"]; !ok {
		t.Error("super+playpause shortcut not found in ParsedShortcuts")
	} else if len(shortcuts) != 1 {
		t.Errorf("Expected 1 parsed shortcut for super+playpause, got %d", len(shortcuts))
	}
}
