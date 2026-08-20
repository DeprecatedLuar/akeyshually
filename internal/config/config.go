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
	"github.com/deprecatedluar/akeyshually/internal/keys"
)

//go:embed defaults/*
var embeddedConfigs embed.FS

type Settings struct {
	DefaultInterval       float64  `toml:"default_interval"`         // >= 10 = milliseconds, < 10 = seconds (default: 150ms)
	DisableMediaKeys      bool     `toml:"disable_media_keys"`       // Forward media keys to system (default: false)
	Shell                 string   `toml:"shell"`                    // Optional: override $SHELL
	EnvFile               string   `toml:"env_file"`                 // Optional: source before commands
	NotifyOnOverlayChange bool     `toml:"notify_on_overlay_change"` // Desktop notifications for overlay changes
	Devices               []string `toml:"devices"`                  // Device name substrings to grab (case-insensitive)
}

const (
	defaultIntervalMs          = 150.0 // milliseconds, used when default_interval is not set in config
	normalizeIntervalThreshold = 10.0  // values below this are treated as seconds, not milliseconds
	configDirPerm              = 0755
	configFilePerm             = 0644
)

var defaultConfigFiles = []string{"config.toml", "akeyshually.service"}

type BehaviorMode int

const (
	BehaviorNormal    BehaviorMode = iota
	BehaviorHold                   // sustained while key held; 1 command
	BehaviorLongPress              // fires once after threshold, done
	BehaviorSwitch
	BehaviorDoubleTap
	BehaviorPressRelease    // Commands[0] on press (can be ""), Commands[1] on release
	BehaviorHoldRelease     // Commands[0] at hold threshold (can be ""), Commands[1] on release after threshold
	BehaviorTapHold         // tap fires Commands[0], tap-then-hold sustains Commands[1]
	BehaviorTapLongPress    // tap fires Commands[0], tap-then-longpress fires Commands[1] once
	BehaviorTapPressRelease // tap, then Commands[0] on second press, Commands[1] on release
	BehaviorTapHoldRelease  // tap, then Commands[0] at hold threshold, Commands[1] on release
	BehaviorEscapePending   // pseudo-candidate: prevents early resolution when escape hatches exist
)

type TimingMode int

const (
	TimingPress TimingMode = iota
	TimingRelease
)

type ParsedShortcut struct {
	KeyCombo        string // "super+k" (without suffix)
	Behavior        BehaviorMode
	Timing          TimingMode
	Repeat          bool     // stacks on any trigger; stop condition follows trigger semantics
	Interval        float64  // Milliseconds (0 = use default) — tap window for taphold
	HoldInterval    float64  // Milliseconds (0 = use default) — hold threshold for taphold
	Commands        []string // Single command OR switch array
	Passthrough     bool     // Ignore modifiers when matching
	AliasGroup      string   // Canonical key for shared state (e.g. "f1/f2.switch"), empty if not an alias
	Direction       string   // For axis shortcuts: "+", "-", or "" (both)
	Sensitivity     float64  // For axis shortcuts: fires per full sweep (0 = use default)
	ExplicitOnPress bool     // true if ".onpress" was written explicitly, distinguishes from bare for remap translation
}

type Config struct {
	Settings    Settings               `toml:"settings"`
	VirtualKeys map[string]interface{} `toml:"virtual_keys"`      // Virtual key definitions
	Shortcuts   map[string]interface{} `toml:"shortcuts"`         // Can be string or []interface{}
	Commands    map[string]string      `toml:"command_variables"` // Command aliases

	// Parsed shortcuts grouped by key combo
	ParsedShortcuts map[string][]*ParsedShortcut
	// EscapeMap tracks which combos have child escape hatches (e.g. "super" -> true if "super+w" exists)
	EscapeMap map[string]bool
	// RemapTable maps a combo string (e.g. "capslock", "ctrl+r") to its remap target name,
	// for shortcuts eligible for input-stage translation rather than ladder resolution.
	RemapTable map[string]string
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

// expandVirtualKeys expands virtual key references in shortcuts to their physical key equivalents.
// Virtual keys are file-scoped and only expand within the same config file.
func expandVirtualKeys(cfg *Config) error {
	if len(cfg.VirtualKeys) == 0 {
		return nil
	}

	// Parse virtual keys into map: virtualName -> []physicalKeys
	virtualKeyMap := make(map[string][]string)
	for virtualName, value := range cfg.VirtualKeys {
		virtualName = strings.ToLower(strings.TrimSpace(virtualName))

		switch v := value.(type) {
		case string:
			virtualKeyMap[virtualName] = []string{strings.ToLower(strings.TrimSpace(v))}
		case []interface{}:
			keys := make([]string, len(v))
			for i, k := range v {
				if s, ok := k.(string); ok {
					keys[i] = strings.ToLower(strings.TrimSpace(s))
				} else {
					return fmt.Errorf("virtual key %q: array must contain strings", virtualName)
				}
			}
			virtualKeyMap[virtualName] = keys
		default:
			return fmt.Errorf("virtual key %q: value must be string or array of strings", virtualName)
		}
	}

	// Expand shortcuts that reference virtual keys
	expandedShortcuts := make(map[string]interface{})
	for key, value := range cfg.Shortcuts {
		expanded := expandShortcutKey(key, virtualKeyMap)
		if len(expanded) == 0 {
			// No expansion needed, keep original
			expandedShortcuts[key] = value
		} else {
			// Expand to multiple shortcuts
			for _, expandedKey := range expanded {
				expandedShortcuts[expandedKey] = value
			}
		}
	}

	cfg.Shortcuts = expandedShortcuts
	return nil
}

// expandShortcutKey expands a single shortcut key if it references any virtual keys.
// Returns nil if no expansion needed, or slice of expanded keys.
func expandShortcutKey(key string, virtualKeyMap map[string][]string) []string {
	// Split off behavior suffix (e.g., "super+action.hold" -> "super+action", ".hold")
	parts := strings.Split(key, ".")
	combo := parts[0]
	suffix := ""
	if len(parts) > 1 {
		suffix = "." + strings.Join(parts[1:], ".")
	}

	// Handle aliases: "f1/f2/action" -> each alias may need expansion
	aliases := strings.Split(combo, "/")
	var expandedAliases []string
	hasExpansion := false

	for _, alias := range aliases {
		alias = strings.TrimSpace(alias)
		expanded := expandCombo(alias, virtualKeyMap)
		if len(expanded) > 1 || (len(expanded) == 1 && expanded[0] != alias) {
			hasExpansion = true
		}
		expandedAliases = append(expandedAliases, expanded...)
	}

	if !hasExpansion {
		return nil // No expansion needed
	}

	// Join all expanded aliases with "/" to create shared alias group
	// This ensures behaviors like .switch share state across all physical keys
	joinedCombo := strings.Join(expandedAliases, "/")
	return []string{joinedCombo + suffix}
}

// expandCombo expands a single combo (e.g., "super+action") if it contains a virtual key.
func expandCombo(combo string, virtualKeyMap map[string][]string) []string {
	combo = strings.ToLower(strings.TrimSpace(combo))

	// Check if entire combo is a virtual key (simple case: "action")
	if physicalKeys, ok := virtualKeyMap[combo]; ok {
		return physicalKeys
	}

	// Split by + to get modifiers and main key
	parts := strings.Split(combo, "+")
	if len(parts) == 1 {
		// Single key, not a virtual key
		return []string{combo}
	}

	// Check if last part (main key) is a virtual key
	mainKey := strings.TrimSpace(parts[len(parts)-1])
	if physicalKeys, ok := virtualKeyMap[mainKey]; ok {
		// Expand: reconstruct combo with each physical key
		prefix := strings.Join(parts[:len(parts)-1], "+") + "+"
		var result []string
		for _, physicalKey := range physicalKeys {
			result = append(result, prefix+physicalKey)
		}
		return result
	}

	// No expansion needed
	return []string{combo}
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
		// Include direction in map key for axis shortcuts
		mapKey := parsed.KeyCombo
		if parsed.Direction != "" {
			mapKey = parsed.KeyCombo + parsed.Direction
		}
		dst[mapKey] = append(dst[mapKey], parsed)
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

	// Expand virtual keys (file-scoped, happens before validation)
	if err := expandVirtualKeys(cfg); err != nil {
		return nil, fmt.Errorf("failed to expand virtual keys: %w", err)
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

	// Build escape map
	cfg.EscapeMap = buildEscapeMap(cfg.ParsedShortcuts)
	cfg.RemapTable = cfg.buildRemapTable()

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

	// Rebuild escape map
	c.EscapeMap = buildEscapeMap(c.ParsedShortcuts)
	c.RemapTable = c.buildRemapTable()
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

	// Expand virtual keys (file-scoped, happens before validation)
	if err := expandVirtualKeys(cfg); err != nil {
		return nil, fmt.Errorf("failed to expand virtual keys: %w", err)
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

// normalizeKeyCombo normalizes all keys in a combo string and reorders modifiers
// into canonical order: super → ctrl → alt → shift → key
func normalizeKeyCombo(combo string) string {
	parts := strings.Split(combo, "+")

	// Normalize each part
	for i, part := range parts {
		parts[i] = normalizeKey(part)
	}

	// Separate modifiers from regular key
	var modifiers []string
	var regularKey string

	for _, part := range parts {
		switch part {
		case "super", "ctrl", "alt", "shift":
			modifiers = append(modifiers, part)
		default:
			regularKey = part
		}
	}

	// Build result in canonical order: super → ctrl → alt → shift → key
	var result []string
	for _, mod := range []string{"super", "ctrl", "alt", "shift"} {
		for _, m := range modifiers {
			if m == mod {
				result = append(result, mod)
				break
			}
		}
	}

	// Append regular key last (if present)
	if regularKey != "" {
		result = append(result, regularKey)
	}

	return strings.Join(result, "+")
}

// ParseShortcut parses a shortcut key with dot syntax into a ParsedShortcut
// Format: "keycombo[.behavior][.timing]"
// Examples: "super+k", "super+k.whileheld", "super+k.repeat-whileheld(100).onrelease"
func ParseShortcut(key string, value interface{}) (*ParsedShortcut, error) {
	parts := strings.Split(key, ".")
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty shortcut key")
	}

	// Extract direction suffix for axis shortcuts (e.g., "RX+" → direction "+", combo "RX")
	combo := parts[0]
	direction := ""
	if strings.HasSuffix(combo, "+") {
		direction = "+"
		combo = strings.TrimSuffix(combo, "+")
	} else if strings.HasSuffix(combo, "-") {
		direction = "-"
		combo = strings.TrimSuffix(combo, "-")
	}
	if direction != "" {
		axisName := strings.ToLower(strings.TrimSpace(combo))
		if axisCode, ok := keys.ResolveAbsCode(axisName); ok {
			combo = strings.ToLower(keys.GetAbsName(axisCode))
		} else {
			combo = axisName
		}
	}

	shortcut := &ParsedShortcut{
		KeyCombo:    normalizeKeyCombo(combo),
		Behavior:    BehaviorNormal,
		Timing:      TimingPress,
		Interval:    0, // 0 means use default
		Passthrough: false,
		Direction:   direction,
		Sensitivity: 0, // 0 means use default
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
	intervalRegex := regexp.MustCompile(`^(hold|longpress|doubletap|holdrelease)\((\d+\.?\d*|\d*\.\d+)\)$`)
	tapHoldRegex := regexp.MustCompile(`^tap(?:\((\d+\.?\d*|\d*\.\d+)\))?hold(?:\((\d+\.?\d*|\d*\.\d+)\))?$`)
	tapLongPressRegex := regexp.MustCompile(`^tap(?:\((\d+\.?\d*|\d*\.\d+)\))?longpress(?:\((\d+\.?\d*|\d*\.\d+)\))?$`)
	tapPressReleaseRegex := regexp.MustCompile(`^tap(?:\((\d+\.?\d*|\d*\.\d+)\))?pressrelease$`)
	tapHoldReleaseRegex := regexp.MustCompile(`^tap(?:\((\d+\.?\d*|\d*\.\d+)\))?holdrelease(?:\((\d+\.?\d*|\d*\.\d+)\))?$`)

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

		// Check for tappressrelease with optional interval: tap(N)pressrelease
		if matches := tapPressReleaseRegex.FindStringSubmatch(part); matches != nil {
			shortcut.Behavior = BehaviorTapPressRelease
			if matches[1] != "" {
				interval, _ := strconv.ParseFloat(matches[1], 64)
				shortcut.Interval = normalizeInterval(interval)
			}
			continue
		}

		// Check for tapholdrelease with optional intervals: tap(N)holdrelease(N)
		if matches := tapHoldReleaseRegex.FindStringSubmatch(part); matches != nil {
			shortcut.Behavior = BehaviorTapHoldRelease
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
		case "tappressrelease":
			shortcut.Behavior = BehaviorTapPressRelease
		case "tapholdrelease":
			shortcut.Behavior = BehaviorTapHoldRelease
		case "onrelease":
			return nil, fmt.Errorf("onrelease removed: use .pressrelease = [\"\", \"cmd\"]")
		case "onpress":
			shortcut.Timing = TimingPress
			shortcut.ExplicitOnPress = true
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
	case BehaviorTapPressRelease:
		return "tappressrelease"
	case BehaviorTapHoldRelease:
		return "tapholdrelease"
	default:
		return "unknown"
	}
}

// buildEscapeMap creates a map of combo prefixes that have child escape hatches.
// For "super+w", marks "super" -> true. For "super+shift+b", marks both "super" and "super+shift" -> true.
func buildEscapeMap(shortcuts map[string][]*ParsedShortcut) map[string]bool {
	escapeMap := make(map[string]bool)
	for combo, shortcutList := range shortcuts {
		if len(shortcutList) > 0 && shortcutList[0].Direction != "" {
			continue
		}
		// Find last '+' and extract prefix
		lastPlus := strings.LastIndex(combo, "+")
		if lastPlus == -1 {
			continue // No prefix (standalone key like "super")
		}
		prefix := combo[:lastPlus]
		escapeMap[prefix] = true

		// Also mark intermediate prefixes (e.g. "super+shift+b" -> mark "super")
		for {
			lastPlus = strings.LastIndex(prefix, "+")
			if lastPlus == -1 {
				break
			}
			prefix = prefix[:lastPlus]
			escapeMap[prefix] = true
		}
	}
	return escapeMap
}

// scrollAliasTargets are KeyCodeMap entries that resolve to REL codes for scroll/wheel remap
// output (see keys.KeyCodeMap), not real keys. They must stay on the existing one-shot
// emitScrollWheel path and never enter RemapTable, which expects EV_KEY targets.
var scrollAliasTargets = map[string]bool{
	"scrollup": true, "scrolldown": true, "scrollleft": true, "scrollright": true,
	"wheelup": true, "wheeldown": true, "wheelleft": true, "wheelright": true,
}

// buildRemapTable maps each combo eligible for input-stage translation to its remap target name.
// Eligible: the combo has exactly one shortcut (no competing triggers to race), that shortcut is
// BehaviorNormal, not explicitly ".onpress", not ".repeat", and its single command resolves (through
// command_variables) to a plain ">target" remap whose target is a real key.
func (c *Config) buildRemapTable() map[string]string {
	remapTable := make(map[string]string)
	for combo, shortcutList := range c.ParsedShortcuts {
		if len(shortcutList) != 1 {
			continue
		}
		s := shortcutList[0]
		if s.Direction != "" {
			continue
		}
		if s.Behavior != BehaviorNormal || s.ExplicitOnPress || s.Repeat {
			continue
		}
		if len(s.Commands) != 1 {
			continue
		}
		resolved := c.ResolveCommand(s.Commands[0])
		if !strings.HasPrefix(resolved, ">") || strings.HasPrefix(resolved, ">>") {
			continue
		}
		target := resolved[1:]
		if target == "" {
			continue
		}
		if scrollAliasTargets[strings.ToLower(strings.TrimSpace(target))] {
			continue
		}
		if _, ok := keys.ResolveKeyCode(target); !ok {
			continue
		}
		remapTable[combo] = target
	}
	return remapTable
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
