package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
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

// validateBehaviorRequirements routes to appropriate validator based on behavior type
func validateBehaviorRequirements(parsed *ParsedShortcut) error {
	// Remaps don't need command count validation - they always have 1 command
	if parsed.IsRemap {
		return nil
	}

	validator, ok := behaviorValidators[parsed.Behavior]
	if !ok {
		return fmt.Errorf("unknown behavior: %v", parsed.Behavior)
	}
	return validator(parsed)
}

// Lookup table mapping behaviors to their validation functions
var behaviorValidators = map[BehaviorMode]func(*ParsedShortcut) error{
	BehaviorNormal:       validateNormal,
	BehaviorHold:         validateHold,
	BehaviorLongPress:    validateLongPress,
	BehaviorSwitch:       validateSwitch,
	BehaviorDoubleTap:    validateDoubleTap,
	BehaviorPressRelease: validatePressRelease,
	BehaviorHoldRelease:  validateHoldRelease,
	BehaviorTapHold:      validateTapHold,
	BehaviorTapLongPress: validateTapLongPress,
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
