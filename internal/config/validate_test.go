package config

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestValidateShortcutEntry_UnknownKeyInCombo(t *testing.T) {
	err := validateShortcutEntry("super+unknownkey", "echo test", "test.toml")
	if err == nil {
		t.Fatal("expected error for unknown key, got nil")
	}
	verr, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if !strings.Contains(verr.Message, "unknownkey") {
		t.Errorf("error message should mention unknown key, got: %s", verr.Message)
	}
	if verr.File != "test.toml" {
		t.Errorf("expected file test.toml, got %s", verr.File)
	}
}

func TestValidateShortcutEntry_UnknownKeyInRemapTarget(t *testing.T) {
	err := validateShortcutEntry("super+t", ">unknownkey", "test.toml")
	if err == nil {
		t.Fatal("expected error for unknown key in remap target, got nil")
	}
	verr, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if !strings.Contains(verr.Message, "unknownkey") {
		t.Errorf("error message should mention unknown key, got: %s", verr.Message)
	}
}

func TestValidateShortcutEntry_EmptyRemapTarget(t *testing.T) {
	err := validateShortcutEntry("super+t", ">", "test.toml")
	if err == nil {
		t.Fatal("expected error for empty remap target, got nil")
	}
	verr, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if !strings.Contains(verr.Message, "empty") {
		t.Errorf("error message should mention empty target, got: %s", verr.Message)
	}
}

func TestValidateShortcutEntry_WrongCommandCount(t *testing.T) {
	tests := []struct {
		key      string
		value    interface{}
		errMatch string
	}{
		{"super+t.pressrelease", "single command", "exactly 2 commands"},
		{"super+t.holdrelease", "single command", "exactly 2 commands"},
		{"super+t.switch", "single command", "at least 2 commands"},
		{"super+t.taphold", []interface{}{"cmd1", "cmd2"}, "exactly 1 command"},
		{"super+t.taplongpress", "single command", "exactly 2 commands"},
		{"super+t.hold", []interface{}{"cmd1", "cmd2"}, "exactly 1 command"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			err := validateShortcutEntry(tt.key, tt.value, "test.toml")
			if err == nil {
				t.Fatal("expected error for wrong command count, got nil")
			}
			verr, ok := err.(ValidationError)
			if !ok {
				t.Fatalf("expected ValidationError, got %T", err)
			}
			if !strings.Contains(verr.Message, tt.errMatch) {
				t.Errorf("expected error containing %q, got: %s", tt.errMatch, verr.Message)
			}
		})
	}
}

func TestValidateShortcutEntry_ValidConfig(t *testing.T) {
	tests := []struct {
		key   string
		value interface{}
	}{
		{"super+t", "echo test"},
		{"super+t.hold", "echo test"},
		{"super+t.doubletap", "echo test"},
		{"super+t.pressrelease", []interface{}{"echo press", "echo release"}},
		{"super+t.holdrelease", []interface{}{"echo hold", "echo release"}},
		{"super+t.switch", []interface{}{"cmd1", "cmd2", "cmd3"}},
		{"super+t.taphold", "echo test"},
		{"super+t.taplongpress", []interface{}{"echo tap", "echo longpress"}},
		{"f1/f2/f3.switch", []interface{}{"cmd1", "cmd2"}},
		{"super+t", ">escape"},
		{"super+t", ">>ctrl+c"},
		{"super+t", "<<"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			err := validateShortcutEntry(tt.key, tt.value, "test.toml")
			if err != nil {
				t.Errorf("unexpected error for valid config: %v", err)
			}
		})
	}
}


func TestValidateShortcutEntry_LongpressRepeatRejected(t *testing.T) {
	err := validateShortcutEntry("super+t.longpress.repeat", "echo test", "test.toml")
	if err == nil {
		t.Fatal("expected error for longpress.repeat, got nil")
	}
	verr, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if !strings.Contains(verr.Message, "longpress.repeat") || !strings.Contains(verr.Message, "one-shot") {
		t.Errorf("expected error about longpress.repeat being invalid, got: %s", verr.Message)
	}
}

func TestValidateShortcutEntry_InvalidValueType(t *testing.T) {
	err := validateShortcutEntry("super+t", 123, "test.toml")
	if err == nil {
		t.Fatal("expected error for invalid value type, got nil")
	}
	verr, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if !strings.Contains(verr.Message, "string or array") {
		t.Errorf("error should mention required type, got: %s", verr.Message)
	}
}

func TestValidateShortcutEntry_ArrayWithNonString(t *testing.T) {
	err := validateShortcutEntry("super+t", []interface{}{"cmd1", 123}, "test.toml")
	if err == nil {
		t.Fatal("expected error for array with non-string, got nil")
	}
	verr, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if !strings.Contains(verr.Message, "strings") {
		t.Errorf("error should mention strings requirement, got: %s", verr.Message)
	}
}

func TestValidateConfig_EmptyShortcuts(t *testing.T) {
	// A config with no shortcuts should be valid
	cfg := &Config{
		Shortcuts: make(map[string]interface{}),
	}
	err := validateConfig(cfg, "test.toml")
	if err != nil {
		t.Errorf("empty shortcuts config should be valid, got error: %v", err)
	}
}

func TestValidateConfig_MultipleShortcuts(t *testing.T) {
	tomlContent := `
[shortcuts]
"super+t" = "echo test"
"super+k" = "echo k"
"super+unknownkey" = "echo bad"
`
	cfg := &Config{
		Shortcuts: make(map[string]interface{}),
	}
	_, err := toml.Decode(tomlContent, cfg)
	if err != nil {
		t.Fatalf("failed to decode test config: %v", err)
	}

	err = validateConfig(cfg, "test.toml")
	if err == nil {
		t.Fatal("expected error for unknown key, got nil")
	}
	verr, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if !strings.Contains(verr.Message, "unknownkey") {
		t.Errorf("error should mention the unknown key, got: %s", verr.Message)
	}
	if verr.File != "test.toml" {
		t.Errorf("expected file test.toml, got %s", verr.File)
	}
}

func TestValidationError_Format(t *testing.T) {
	verr := ValidationError{
		File:    "/path/to/config.toml",
		Line:    0,
		Message: "unknown key",
	}
	errStr := verr.Error()
	expected := []string{"config.toml", "unknown key"}
	for _, exp := range expected {
		if !strings.Contains(errStr, exp) {
			t.Errorf("error string should contain %q, got: %s", exp, errStr)
		}
	}
}

func TestValidateShortcutEntry_AliasValidation(t *testing.T) {
	// All aliases should be validated
	err := validateShortcutEntry("f1/unknownkey/f3.switch", []interface{}{"cmd1", "cmd2"}, "test.toml")
	if err == nil {
		t.Fatal("expected error for unknown key in alias, got nil")
	}
	verr, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if !strings.Contains(verr.Message, "unknownkey") {
		t.Errorf("error should mention the unknown key, got: %s", verr.Message)
	}
}
