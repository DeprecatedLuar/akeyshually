package matcher

import (
	"github.com/deprecatedluar/akeyshually/internal/keys"
)

// ResolveKeyCode looks up a key name and returns its evdev code.
// Delegates to internal/keys package.
func ResolveKeyCode(name string) (uint16, bool) {
	return keys.ResolveKeyCode(name)
}

// GetKeyName returns the canonical name for a key code.
// Delegates to internal/keys package.
func GetKeyName(code uint16) string {
	return keys.GetKeyName(code)
}

// GetAbsName returns a human-readable name for an EV_ABS axis code.
// Delegates to internal/keys package.
func GetAbsName(code uint16) string {
	return keys.GetAbsName(code)
}
