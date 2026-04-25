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
	DefaultInterval       float64  `toml:"default_interval"`        // >= 10 = milliseconds, < 10 = seconds (default: 150ms)
	DisableMediaKeys      bool     `toml:"disable_media_keys"`      // Forward media keys to system (default: false)
	Shell                 string   `toml:"shell"`                   // Optional: override $SHELL
	EnvFile               string   `toml:"env_file"`                // Optional: source before commands
	NotifyOnOverlayChange bool     `toml:"notify_on_overlay_change"` // Desktop notifications for overlay changes
	Devices               []string `toml:"devices"`                 // Device name substrings to grab (case-insensitive)
}

const (
	defaultIntervalMs          = 150.0 // milliseconds, used when default_interval is not set in config
	normalizeIntervalThreshold = 10.0  // values below this are treated as seconds, not milliseconds
	configDirPerm              = 0755
	configFilePerm             = 0644
)

var defaultConfigFiles = []string{"config.toml", "akeyshually.service"}

const (
	RemapTap         = 0 // down+up+SYN (default)
	RemapHoldForever = 1 // keydown only; keys stay held until <<
	RemapKeyUp       = 2 // keyup only for specified key
	RemapReleaseAll  = 3 // release all persistently held keys
)

type BehaviorMode int

const (
	BehaviorNormal BehaviorMode = iota
	BehaviorHold                // sustained while key held; 1 command
	BehaviorLongPress           // fires once after threshold, done
	BehaviorSwitch
	BehaviorDoubleTap
	BehaviorPressRelease        // Commands[0] on press (can be ""), Commands[1] on release
	BehaviorHoldRelease         // Commands[0] at hold threshold (can be ""), Commands[1] on release after threshold
	BehaviorTapHold             // tap fires Commands[0], tap-then-hold sustains Commands[1]
	BehaviorTapLongPress        // tap fires Commands[0], tap-then-longpress fires Commands[1] once
)

type TimingMode int

const (
	TimingPress TimingMode = iota
	TimingRelease
)

type ParsedShortcut struct {
	KeyCombo    string       // "super+k" (without suffix)
	Behavior    BehaviorMode
	Timing      TimingMode
	Repeat      bool     // stacks on any trigger; stop condition follows trigger semantics
	Interval    float64  // Milliseconds (0 = use default) — tap window for taphold
	HoldInterval float64 // Milliseconds (0 = use default) — hold threshold for taphold
	Commands    []string // Single command OR switch array
	Passthrough bool     // Ignore modifiers when matching
	AliasGroup  string   // Canonical key for shared state (e.g. "f1/f2.switch"), empty if not an alias
	IsRemap     bool     // true if value starts with ">" or "<"
	RemapCombo  string   // the key/combo to remap to (empty for RemapReleaseAll)
	RemapMode   int      // RemapTap, RemapHoldForever, RemapKeyUp, RemapReleaseAll
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
	if value >= normalizeIntervalThreshold {
		return value // Already in milliseconds
	}
	return value * 1000 // Convert seconds to milliseconds
}

func Load() (*Config, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return nil, err
	}
	return loadFromFile(filepath.Join(configDir, "config.toml"))
}

// LoadFromPath loads config from a custom path
// Path can be: filename (resolved to config dir), or absolute/relative path
// Adds .toml extension if missing
func LoadFromPath(path string) (*Config, error) {
	// Add .toml extension if missing
	if !strings.HasSuffix(path, ".toml") {
		path += ".toml"
	}

	// If not an absolute path, resolve relative to config dir
	if !filepath.IsAbs(path) {
		configDir, err := getConfigDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(configDir, path)
	}

	return loadFromFile(path)
}

// parseShortcutsInto parses a raw shortcut key (possibly with / aliases) into the map.
// Aliases share an AliasGroup so switch state is shared across all combos in the group.
func parseShortcutsInto(dst map[string][]*ParsedShortcut, key string, value interface{}) error {
	aliases := strings.Split(key, "/")
	aliasGroup := ""
	if len(aliases) > 1 {
		aliasGroup = key
	}

	// Extract dot-modifiers from last alias and apply to earlier ones
	lastPart := aliases[len(aliases)-1]
	dotIdx := strings.Index(lastPart, ".")
	modifiers := ""
	if dotIdx != -1 {
		modifiers = lastPart[dotIdx:]
	}

	for i, alias := range aliases {
		var fullKey string
		if i == len(aliases)-1 {
			fullKey = strings.TrimSpace(alias)
		} else {
			fullKey = strings.TrimSpace(alias) + modifiers
		}

		parsed, err := ParseShortcut(fullKey, value)
		if err != nil {
			return err
		}
		parsed.AliasGroup = aliasGroup
		dst[parsed.KeyCombo] = append(dst[parsed.KeyCombo], parsed)
	}
	return nil
}

func loadFromFile(configPath string) (*Config, error) {
	cfg := &Config{
		Shortcuts: make(map[string]interface{}),
		Commands:  make(map[string]string),
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config not found: %s", configPath)
	}

	// Decode with metadata to get line numbers
	meta, err := toml.DecodeFile(configPath, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate config before processing
	if err := validateConfig(cfg, configPath, &meta); err != nil {
		return nil, err
	}

	// Set default loop interval if not specified
	if cfg.Settings.DefaultInterval == 0 {
		cfg.Settings.DefaultInterval = defaultIntervalMs
	} else {
		cfg.Settings.DefaultInterval = normalizeInterval(cfg.Settings.DefaultInterval)
	}

	// Parse shortcuts
	cfg.ParsedShortcuts = make(map[string][]*ParsedShortcut)
	for key, value := range cfg.Shortcuts {
		if err := parseShortcutsInto(cfg.ParsedShortcuts, key, value); err != nil {
			return nil, fmt.Errorf("failed to parse shortcut '%s': %w", key, err)
		}
	}

	return cfg, nil
}

// LoadWithOverlays loads the base config and merges overlay configs on top
// All configs (base + overlays) must be valid or this returns an error
func LoadWithOverlays(overlays []string) (*Config, error) {
	base, err := Load()
	if err != nil {
		return nil, err
	}

	for _, overlayFile := range overlays {
		overlay, err := loadOverlay(overlayFile)
		if err != nil {
			return nil, fmt.Errorf("overlay %s: %w", overlayFile, err)
		}
		base.Merge(overlay)
	}

	return base, nil
}

// Merge merges an overlay config into this config
func (c *Config) Merge(overlay *Config) {
	// Merge shortcuts (overlay overrides base)
	for key, value := range overlay.Shortcuts {
		c.Shortcuts[key] = value
	}

	// Merge command_variables (overlay overrides base)
	for key, value := range overlay.Commands {
		c.Commands[key] = value
	}

	// Merge default_loop_interval if overlay specifies one
	if overlay.Settings.DefaultInterval != 0 {
		c.Settings.DefaultInterval = overlay.Settings.DefaultInterval
	}

	// Merge devices (deduplicated, case-insensitive)
	existing := make(map[string]bool, len(c.Settings.Devices))
	for _, d := range c.Settings.Devices {
		existing[strings.ToLower(d)] = true
	}
	for _, d := range overlay.Settings.Devices {
		if !existing[strings.ToLower(d)] {
			c.Settings.Devices = append(c.Settings.Devices, d)
			existing[strings.ToLower(d)] = true
		}
	}

	// Rebuild ParsedShortcuts after merge
	// Note: All shortcuts were already validated, so errors here indicate a bug
	c.ParsedShortcuts = make(map[string][]*ParsedShortcut)
	for key, value := range c.Shortcuts {
		if err := parseShortcutsInto(c.ParsedShortcuts, key, value); err != nil {
			panic(fmt.Sprintf("BUG: validated shortcut failed to parse during merge: '%s': %v", key, err))
		}
	}
}

// loadOverlay loads an overlay config file from the config directory
func loadOverlay(filename string) (*Config, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return nil, err
	}

	overlayPath := filepath.Join(configDir, filename)

	cfg := &Config{
		Shortcuts: make(map[string]interface{}),
		Commands:  make(map[string]string),
	}

	// Decode with metadata to get line numbers
	meta, err := toml.DecodeFile(overlayPath, cfg)
	if err != nil {
		return nil, err
	}

	// Validate overlay before returning
	if err := validateConfig(cfg, overlayPath, &meta); err != nil {
		return nil, err
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
	// Media key aliases
	case "play":
		return "playpause"
	case "next":
		return "nextsong"
	case "previous", "prev":
		return "previoussong"
	case "calculator":
		return "calc"
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
// Examples: "super+k", "super+k.whileheld", "super+k.repeat-whileheld(100).onrelease"
func ParseShortcut(key string, value interface{}) (*ParsedShortcut, error) {
	parts := strings.Split(key, ".")
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty shortcut key")
	}

	shortcut := &ParsedShortcut{
		KeyCombo:    normalizeKeyCombo(parts[0]),
		Behavior:    BehaviorNormal,
		Timing:      TimingPress,
		Interval:    0, // 0 means use default
		Passthrough: false,
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

	// Detect remap syntax: value starting with ">" or "<"
	if len(shortcut.Commands) == 1 {
		val := shortcut.Commands[0]
		switch {
		case val == "<<":
			shortcut.IsRemap = true
			shortcut.RemapMode = RemapReleaseAll
		case strings.HasPrefix(val, ">>"):
			target := val[2:]
			if target == "" {
				return nil, fmt.Errorf("remap target cannot be empty")
			}
			shortcut.IsRemap = true
			shortcut.RemapCombo = normalizeKeyCombo(target)
			shortcut.RemapMode = RemapHoldForever
		case strings.HasPrefix(val, ">"):
			target := val[1:]
			if target == "" {
				return nil, fmt.Errorf("remap target cannot be empty")
			}
			shortcut.IsRemap = true
			shortcut.RemapCombo = normalizeKeyCombo(target)
			shortcut.RemapMode = RemapTap
		case strings.HasPrefix(val, "<"):
			target := val[1:]
			if target == "" {
				return nil, fmt.Errorf("remap keyup target cannot be empty")
			}
			shortcut.IsRemap = true
			shortcut.RemapCombo = normalizeKeyCombo(target)
			shortcut.RemapMode = RemapKeyUp
		}
	}

	// Parse modifiers (behavior and timing)
	intervalRegex := regexp.MustCompile(`^(hold|longpress|doubletap|holdrelease)\((\d+\.?\d*|\d*\.\d+)\)$`)
	tapHoldRegex := regexp.MustCompile(`^tap(?:\((\d+\.?\d*|\d*\.\d+)\))?hold(?:\((\d+\.?\d*|\d*\.\d+)\))?$`)
	tapLongPressRegex := regexp.MustCompile(`^tap(?:\((\d+\.?\d*|\d*\.\d+)\))?longpress(?:\((\d+\.?\d*|\d*\.\d+)\))?$`)

	for i := 1; i < len(parts); i++ {
		part := strings.ToLower(parts[i])

		// Check for taphold with optional intervals: tap(N)hold(N)
		if matches := tapHoldRegex.FindStringSubmatch(part); matches != nil {
			shortcut.Behavior = BehaviorTapHold
			if matches[1] != "" {
				interval, _ := strconv.ParseFloat(matches[1], 64)
				shortcut.Interval = normalizeInterval(interval)
			}
			if matches[2] != "" {
				interval, _ := strconv.ParseFloat(matches[2], 64)
				shortcut.HoldInterval = normalizeInterval(interval)
			}
			continue
		}

		// Check for taplongpress with optional intervals: tap(N)longpress(N)
		if matches := tapLongPressRegex.FindStringSubmatch(part); matches != nil {
			shortcut.Behavior = BehaviorTapLongPress
			if matches[1] != "" {
				interval, _ := strconv.ParseFloat(matches[1], 64)
				shortcut.Interval = normalizeInterval(interval)
			}
			if matches[2] != "" {
				interval, _ := strconv.ParseFloat(matches[2], 64)
				shortcut.HoldInterval = normalizeInterval(interval)
			}
			continue
		}

		// Check for interval notation: hold(N), longpress(N), doubletap(N)
		if matches := intervalRegex.FindStringSubmatch(part); matches != nil {
			modifierName := matches[1]
			interval, _ := strconv.ParseFloat(matches[2], 64)

			switch modifierName {
			case "hold":
				shortcut.Behavior = BehaviorHold
			case "longpress":
				shortcut.Behavior = BehaviorLongPress
			case "doubletap":
				shortcut.Behavior = BehaviorDoubleTap
			case "holdrelease":
				shortcut.Behavior = BehaviorHoldRelease
			}
			shortcut.Interval = normalizeInterval(interval)
			continue
		}

		// Parse behavior modes (without interval)
		switch part {
		case "hold":
			shortcut.Behavior = BehaviorHold
		case "holdrelease":
			shortcut.Behavior = BehaviorHoldRelease
		case "longpress":
			shortcut.Behavior = BehaviorLongPress
		case "repeat":
			shortcut.Repeat = true
		case "switch":
			shortcut.Behavior = BehaviorSwitch
		case "doubletap":
			shortcut.Behavior = BehaviorDoubleTap
		case "pressrelease":
			shortcut.Behavior = BehaviorPressRelease
		case "onrelease":
			return nil, fmt.Errorf("onrelease removed: use .pressrelease = [\"\", \"cmd\"]")
		case "onpress":
			shortcut.Timing = TimingPress
		case "passthrough":
			shortcut.Passthrough = true
		default:
			return nil, fmt.Errorf("unknown modifier: %s", part)
		}
	}

	// Command count validation now happens in validateConfig before ParseShortcut is called
	return shortcut, nil
}

func behaviorName(b BehaviorMode) string {
	switch b {
	case BehaviorNormal:
		return "normal"
	case BehaviorHold:
		return "hold"
	case BehaviorLongPress:
		return "longpress"
	case BehaviorSwitch:
		return "switch"
	case BehaviorDoubleTap:
		return "doubletap"
	case BehaviorPressRelease:
		return "pressrelease"
	case BehaviorHoldRelease:
		return "holdrelease"
	case BehaviorTapHold:
		return "taphold"
	case BehaviorTapLongPress:
		return "taplongpress"
	default:
		return "unknown"
	}
}

func GetConfigDir() (string, error) {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "akeyshually"), nil
	}
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

	if err := os.MkdirAll(configDir, configDirPerm); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	for _, filename := range defaultConfigFiles {
		destPath := filepath.Join(configDir, filename)
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			data, err := embeddedConfigs.ReadFile("defaults/" + filename)
			if err != nil {
				return fmt.Errorf("failed to read embedded %s: %w", filename, err)
			}
			if err := os.WriteFile(destPath, data, configFilePerm); err != nil {
				return fmt.Errorf("failed to write %s: %w", filename, err)
			}
		}
	}

	return nil
}
