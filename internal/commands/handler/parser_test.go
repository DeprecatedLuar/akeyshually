package handler

import (
	"testing"
)

const testVersion = "dev"
const testRepo = "DeprecatedLuar/akeyshually"

func TestParseNoArgs(t *testing.T) {
	result := Parse([]string{}, testVersion, testRepo)
	if !result.RunForeground {
		t.Error("expected RunForeground=true with no args")
	}
	if result.ConfigPath != "" {
		t.Errorf("expected empty ConfigPath, got %q", result.ConfigPath)
	}
}

func TestParseConfigShortFlag(t *testing.T) {
	result := Parse([]string{"-c", "myconfig.toml"}, testVersion, testRepo)
	if !result.RunForeground {
		t.Error("expected RunForeground=true")
	}
	if result.ConfigPath != "myconfig.toml" {
		t.Errorf("expected ConfigPath=%q, got %q", "myconfig.toml", result.ConfigPath)
	}
}

func TestParseConfigLongFlag(t *testing.T) {
	result := Parse([]string{"--config", "myconfig.toml"}, testVersion, testRepo)
	if result.ConfigPath != "myconfig.toml" {
		t.Errorf("expected ConfigPath=%q, got %q", "myconfig.toml", result.ConfigPath)
	}
}

func TestParseDebugFlag(t *testing.T) {
	result := Parse([]string{"--debug"}, testVersion, testRepo)
	if !result.RunForeground {
		t.Error("expected RunForeground=true with only --debug")
	}
}

func TestParseDebugWithConfig(t *testing.T) {
	result := Parse([]string{"--debug", "-c", "custom.toml"}, testVersion, testRepo)
	if result.ConfigPath != "custom.toml" {
		t.Errorf("expected ConfigPath=%q, got %q", "custom.toml", result.ConfigPath)
	}
	if !result.RunForeground {
		t.Error("expected RunForeground=true")
	}
}

func TestParseConfigFlagBeforeDebug(t *testing.T) {
	result := Parse([]string{"-c", "first.toml", "--debug"}, testVersion, testRepo)
	if result.ConfigPath != "first.toml" {
		t.Errorf("expected ConfigPath=%q, got %q", "first.toml", result.ConfigPath)
	}
}
