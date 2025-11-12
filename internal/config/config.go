package config

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

//go:embed defaults/*
var embeddedConfigs embed.FS

type Settings struct {
	DefaultLoopInterval float64 `toml:"default_loop_interval"` // >= 10 = milliseconds, < 10 = seconds (default: 100)
	DisableMediaKeys    bool    `toml:"disable_media_keys"`    // Forward media keys to system (default: false)
	Shell               string  `toml:"shell"`                 // Optional: override $SHELL
	EnvFile             string  `toml:"env_file"`              // Optional: source before commands
}

type BehaviorMode int

const (
	BehaviorNormal BehaviorMode = iota
	BehaviorLoop
	BehaviorToggle
	BehaviorSwitch
)

type TimingMode int

const (
	TimingPress TimingMode = iota
	TimingRelease
)

type ParsedShortcut struct {
	KeyCombo string       // "super+k" (without suffix)
	Behavior BehaviorMode
	Timing   TimingMode
	Interval float64  // Milliseconds (0 = use default)
	Commands []string // Single command OR switch array
}

type Config struct {
	Settings  Settings               `toml:"settings"`
	Shortcuts map[string]interface{} `toml:"shortcuts"`         // Can be string or []interface{}
	Commands  map[string]string      `toml:"command_variables"` // Command aliases

	// Parsed shortcuts grouped by key combo
	ParsedShortcuts map[string][]*ParsedShortcut
}

// normalizeInterval converts interval values based on heuristic:
// >= 10: treat as milliseconds (legacy behavior)
// < 10: treat as seconds, convert to milliseconds
func normalizeInterval(value float64) float64 {
	if value >= 10 {
		return value // Already in milliseconds
	}
	return value * 1000 // Convert seconds to milliseconds
}

func Load() (*Config, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Shortcuts: make(map[string]interface{}),
		Commands:  make(map[string]string),
	}

	configPath := filepath.Join(configDir, "config.toml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config.toml not found: %s", configPath)
	}

	if _, err := toml.DecodeFile(configPath, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config.toml: %w", err)
	}

	if len(cfg.Shortcuts) == 0 {
		return nil, fmt.Errorf("no shortcuts defined in config")
	}

	// Set default loop interval if not specified
	if cfg.Settings.DefaultLoopInterval == 0 {
		cfg.Settings.DefaultLoopInterval = 100
	} else {
		cfg.Settings.DefaultLoopInterval = normalizeInterval(cfg.Settings.DefaultLoopInterval)
	}

	// Parse shortcuts
	cfg.ParsedShortcuts = make(map[string][]*ParsedShortcut)
	for key, value := range cfg.Shortcuts {
		parsed, err := ParseShortcut(key, value)
		if err != nil {
			return nil, fmt.Errorf("failed to parse shortcut '%s': %w", key, err)
		}
		cfg.ParsedShortcuts[parsed.KeyCombo] = append(cfg.ParsedShortcuts[parsed.KeyCombo], parsed)
	}

	return cfg, nil
}

func (c *Config) ResolveCommand(ref string) string {
	if cmd, ok := c.Commands[ref]; ok {
		return cmd
	}
	return ref
}

// normalizeKey converts key name aliases to their canonical form
func normalizeKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))

	// Modifier aliases
	switch key {
	case "mod", "meta", "win", "cmd":
		return "super"
	case "control", "ctl":
		return "ctrl"
	case "sft":
		return "shift"
	// Regular key aliases
	case "prt", "prtsc":
		return "print"
	case "ret":
		return "return"
	case "del":
		return "delete"
	case "ins":
		return "insert"
	case "esc":
		return "escape"
	case "bksp":
		return "backspace"
	}

	return key
}

// normalizeKeyCombo normalizes all keys in a combo string
func normalizeKeyCombo(combo string) string {
	parts := strings.Split(combo, "+")
	for i, part := range parts {
		parts[i] = normalizeKey(part)
	}
	return strings.Join(parts, "+")
}

// ParseShortcut parses a shortcut key with dot syntax into a ParsedShortcut
// Format: "keycombo[.behavior][.timing]"
// Examples: "super+k", "super+k.loop", "super+k.loop(100).onrelease"
func ParseShortcut(key string, value interface{}) (*ParsedShortcut, error) {
	parts := strings.Split(key, ".")
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty shortcut key")
	}

	shortcut := &ParsedShortcut{
		KeyCombo: normalizeKeyCombo(parts[0]),
		Behavior: BehaviorNormal,
		Timing:   TimingPress,
		Interval: 0, // 0 means use default
	}

	// Parse value (string or array)
	switch v := value.(type) {
	case string:
		shortcut.Commands = []string{v}
	case []interface{}:
		commands := make([]string, len(v))
		for i, cmd := range v {
			if s, ok := cmd.(string); ok {
				commands[i] = s
			} else {
				return nil, fmt.Errorf("array value must contain strings")
			}
		}
		shortcut.Commands = commands
	default:
		return nil, fmt.Errorf("value must be string or array of strings")
	}

	// Parse modifiers (behavior and timing)
	intervalRegex := regexp.MustCompile(`^(loop|whileheld|toggle)\((\d+\.?\d*|\d*\.\d+)\)$`)

	for i := 1; i < len(parts); i++ {
		part := strings.ToLower(parts[i])

		// Check for interval notation
		if matches := intervalRegex.FindStringSubmatch(part); matches != nil {
			behaviorName := matches[1]
			interval, _ := strconv.ParseFloat(matches[2], 64)

			switch behaviorName {
			case "loop", "whileheld":
				shortcut.Behavior = BehaviorLoop
			case "toggle":
				shortcut.Behavior = BehaviorToggle
			}
			shortcut.Interval = normalizeInterval(interval)
			continue
		}

		// Parse behavior modes (without interval)
		switch part {
		case "loop", "whileheld":
			shortcut.Behavior = BehaviorLoop
		case "toggle":
			shortcut.Behavior = BehaviorToggle
		case "switch":
			shortcut.Behavior = BehaviorSwitch
		case "onrelease":
			shortcut.Timing = TimingRelease
		case "onpress":
			shortcut.Timing = TimingPress
		default:
			return nil, fmt.Errorf("unknown modifier: %s", part)
		}
	}

	// Validate combinations
	if shortcut.Behavior == BehaviorSwitch {
		if len(shortcut.Commands) < 2 {
			return nil, fmt.Errorf("switch behavior requires array of at least 2 commands")
		}
	} else {
		if len(shortcut.Commands) != 1 {
			return nil, fmt.Errorf("%s behavior requires single command string", behaviorName(shortcut.Behavior))
		}
	}

	return shortcut, nil
}

func behaviorName(b BehaviorMode) string {
	switch b {
	case BehaviorNormal:
		return "normal"
	case BehaviorLoop:
		return "loop"
	case BehaviorToggle:
		return "toggle"
	case BehaviorSwitch:
		return "switch"
	default:
		return "unknown"
	}
}

func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".config", "akeyshually"), nil
}

func getConfigDir() (string, error) {
	return GetConfigDir()
}

func EnsureConfigExists() error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	files := []string{"config.toml", "akeyshually.service"}
	for _, filename := range files {
		destPath := filepath.Join(configDir, filename)
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			data, err := embeddedConfigs.ReadFile("defaults/" + filename)
			if err != nil {
				return fmt.Errorf("failed to read embedded %s: %w", filename, err)
			}
			if err := os.WriteFile(destPath, data, 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", filename, err)
			}
		}
	}

	return nil
}
