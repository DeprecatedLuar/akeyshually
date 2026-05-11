package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/deprecatedluar/akeyshually/internal/keys"
	gohelp "github.com/DeprecatedLuar/gohelp-luar"
)

// ValidationError holds a single validation failure with file and line context.
type ValidationError struct {
	File    string
	Line    int
	Key     string
	Message string
}

func (e ValidationError) Error() string {
	msg := fmt.Sprintf("%q: %s", e.Key, e.Message)
	if e.Line > 0 {
		return fmt.Sprintf("config error in %s\n  line %d: %s", e.File, e.Line, msg)
	}
	return fmt.Sprintf("config error in %s\n  %s", e.File, msg)
}

// ValidationErrors holds multiple validation errors and can format them nicely
type ValidationErrors struct {
	Errors []ValidationError
}

func (ve ValidationErrors) Error() string {
	if len(ve.Errors) == 0 {
		return "no validation errors"
	}

	var parts []string
	for _, err := range ve.Errors {
		parts = append(parts, err.Error())
	}
	return strings.Join(parts, "\n")
}

// FormatWithGohelp renders errors using gohelp for visual consistency
func (ve ValidationErrors) FormatWithGohelp() {
	if len(ve.Errors) == 0 {
		return
	}

	// Group errors by file
	byFile := make(map[string][]ValidationError)
	for _, err := range ve.Errors {
		filename := filepath.Base(err.File)
		byFile[filename] = append(byFile[filename], err)
	}

	// Build page with sections per file
	page := gohelp.NewPage("Config Errors", "")

	for filename, errors := range byFile {
		var items []gohelp.Entry
		for _, err := range errors {
			label := fmt.Sprintf("line %d: %q", err.Line, err.Key)
			items = append(items, gohelp.Item(label, err.Message))
		}
		page = page.Section(filename, items...)
	}

	gohelp.Run([]string{}, page)
}

// getLineNumbers parses the TOML file to extract line numbers for shortcut keys
func getLineNumbers(filePath string) map[string]int {
	lineMap := make(map[string]int)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return lineMap
	}

	inShortcuts := false
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track when we enter/exit [shortcuts] section
		if trimmed == "[shortcuts]" {
			inShortcuts = true
			continue
		}
		if strings.HasPrefix(trimmed, "[") && trimmed != "[shortcuts]" {
			inShortcuts = false
			continue
		}

		// Extract key from "key" = value lines
		if inShortcuts && strings.Contains(trimmed, "=") && !strings.HasPrefix(trimmed, "#") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				key := strings.Trim(strings.TrimSpace(parts[0]), "\"")
				lineMap[key] = i + 1 // Line numbers start at 1
			}
		}
	}
	return lineMap
}

// validateConfig validates a parsed Config.
// Collects all validation errors and returns them together.
func validateConfig(cfg *Config, filePath string, meta *toml.MetaData) error {
	var errors []ValidationError

	// Build line number map from source file
	lineNumbers := getLineNumbers(filePath)

	for key, value := range cfg.Shortcuts {
		line := lineNumbers[key]
		if err := validateShortcutEntry(key, value, filePath, line); err != nil {
			if ve, ok := err.(ValidationError); ok {
				errors = append(errors, ve)
			}
		}
	}

	if len(errors) > 0 {
		return ValidationErrors{Errors: errors}
	}
	return nil
}

// validateShortcutEntry validates a single shortcut entry using the real parser
func validateShortcutEntry(key string, value interface{}, filePath string, line int) error {
	// Use ParseShortcut as single source of truth for all syntax validation
	parsed, err := ParseShortcut(key, value)
	if err != nil {
		return ValidationError{
			File:    filePath,
			Line:    line,
			Key:     key,
			Message: err.Error(),
		}
	}

	// Validate all keys in the combo exist (use original key to preserve +/- suffix)
	comboToValidate := strings.Split(key, ".")[0] // Get combo part before behavior modifiers
	if err := validateKeysExist(comboToValidate); err != nil {
		return ValidationError{
			File:    filePath,
			Line:    line,
			Key:     key,
			Message: err.Error(),
		}
	}

	// Validate behavior-specific requirements (command counts, etc.)
	if err := validateBehaviorRequirements(parsed); err != nil {
		return ValidationError{
			File:    filePath,
			Line:    line,
			Key:     key,
			Message: err.Error(),
		}
	}

	return nil
}

// validateKeysExist checks if all keys in a combo are valid
// Handles both single combos ("super+k") and aliases ("f1/f2/f3")
func validateKeysExist(combo string) error {
	// Split on / for aliases first
	aliases := strings.Split(combo, "/")
	for _, alias := range aliases {
		// Check if this is an axis shortcut (ends with +/-)
		isAxis := strings.HasSuffix(alias, "+") || strings.HasSuffix(alias, "-")

		// Strip direction suffix for axis shortcuts (e.g., "RX+" → "RX")
		if isAxis {
			alias = strings.TrimSuffix(alias, "+")
			alias = strings.TrimSuffix(alias, "-")
		}

		// Then split on + for key combos
		parts := strings.Split(alias, "+")
		for i, part := range parts {
			keyName := strings.ToLower(strings.TrimSpace(part))

			// For axis shortcuts, the final part should be an axis name
			if isAxis && i == len(parts)-1 {
				if _, ok := keys.ResolveAbsCode(keyName); !ok {
					return fmt.Errorf("unknown axis: %s", keyName)
				}
			} else {
				// Regular key validation
				if _, ok := keys.ResolveKeyCode(keyName); !ok {
					return fmt.Errorf("unknown key: %s", keyName)
				}
			}
		}
	}
	return nil
}

// isRemapCommand checks if a command string uses remap syntax (>, >>, <, <<)
func isRemapCommand(cmd string) bool {
	return strings.HasPrefix(cmd, ">") || strings.HasPrefix(cmd, "<")
}

// validateBehaviorRequirements routes to appropriate validator based on behavior type
func validateBehaviorRequirements(parsed *ParsedShortcut) error {
	// Validate remap syntax if detected
	if len(parsed.Commands) == 1 && isRemapCommand(parsed.Commands[0]) {
		return validateRemapCommand(parsed.Commands[0])
	}

	validator, ok := behaviorValidators[parsed.Behavior]
	if !ok {
		return fmt.Errorf("unknown behavior: %v", parsed.Behavior)
	}
	return validator(parsed)
}

// validateRemapCommand validates remap syntax and ensures non-empty, valid targets
func validateRemapCommand(cmd string) error {
	var target string

	switch {
	case cmd == "<<":
		return nil // RemapReleaseAll - no target needed
	case strings.HasPrefix(cmd, ">>"):
		if len(cmd) == 2 {
			return fmt.Errorf("remap target cannot be empty")
		}
		target = cmd[2:]
	case strings.HasPrefix(cmd, ">"):
		if len(cmd) == 1 {
			return fmt.Errorf("remap target cannot be empty")
		}
		target = cmd[1:]
	case strings.HasPrefix(cmd, "<"):
		if len(cmd) == 1 {
			return fmt.Errorf("remap keyup target cannot be empty")
		}
		target = cmd[1:]
	}

	// Validate the target combo contains valid keys
	if target != "" {
		return validateKeysExist(target)
	}
	return nil
}

// Lookup table mapping behaviors to their validation functions
var behaviorValidators = map[BehaviorMode]func(*ParsedShortcut) error{
	BehaviorNormal:          validateNormal,
	BehaviorHold:            validateHold,
	BehaviorLongPress:       validateLongPress,
	BehaviorSwitch:          validateSwitch,
	BehaviorDoubleTap:       validateDoubleTap,
	BehaviorPressRelease:    validatePressRelease,
	BehaviorHoldRelease:     validateHoldRelease,
	BehaviorTapHold:         validateTapHold,
	BehaviorTapLongPress:    validateTapLongPress,
	BehaviorTapPressRelease: validateTapPressRelease,
	BehaviorTapHoldRelease:  validateTapHoldRelease,
}

// Individual behavior validators - each validates command count and behavior-specific rules

func validateNormal(p *ParsedShortcut) error {
	if len(p.Commands) != 1 {
		return fmt.Errorf("normal behavior requires exactly 1 command")
	}
	return nil
}

func validateHold(p *ParsedShortcut) error {
	if len(p.Commands) != 1 {
		return fmt.Errorf("hold behavior requires exactly 1 command")
	}
	return nil
}

func validateLongPress(p *ParsedShortcut) error {
	if p.Repeat {
		return fmt.Errorf("longpress.repeat is not valid (longpress is one-shot)")
	}
	if len(p.Commands) != 1 {
		return fmt.Errorf("longpress behavior requires exactly 1 command")
	}
	return nil
}

func validateSwitch(p *ParsedShortcut) error {
	if len(p.Commands) < 2 {
		return fmt.Errorf("switch behavior requires at least 2 commands")
	}
	return nil
}

func validateDoubleTap(p *ParsedShortcut) error {
	if len(p.Commands) != 1 {
		return fmt.Errorf("doubletap behavior requires exactly 1 command")
	}
	return nil
}

func validatePressRelease(p *ParsedShortcut) error {
	if len(p.Commands) != 2 {
		return fmt.Errorf("pressrelease behavior requires exactly 2 commands")
	}
	return nil
}

func validateHoldRelease(p *ParsedShortcut) error {
	if len(p.Commands) != 2 {
		return fmt.Errorf("holdrelease behavior requires exactly 2 commands")
	}
	return nil
}

func validateTapHold(p *ParsedShortcut) error {
	if len(p.Commands) != 1 {
		return fmt.Errorf("taphold behavior requires exactly 1 command")
	}
	return nil
}

func validateTapLongPress(p *ParsedShortcut) error {
	if len(p.Commands) != 2 {
		return fmt.Errorf("taplongpress behavior requires exactly 2 commands")
	}
	return nil
}

func validateTapPressRelease(p *ParsedShortcut) error {
	if len(p.Commands) != 2 {
		return fmt.Errorf("tappressrelease behavior requires exactly 2 commands")
	}
	return nil
}

func validateTapHoldRelease(p *ParsedShortcut) error {
	if len(p.Commands) != 2 {
		return fmt.Errorf("tapholdrelease behavior requires exactly 2 commands")
	}
	return nil
}
