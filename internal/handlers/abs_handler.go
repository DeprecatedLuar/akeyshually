package handlers

import (
	"strings"

	"github.com/deprecatedluar/akeyshually/internal/common"
	"github.com/deprecatedluar/akeyshually/internal/config"
	"github.com/deprecatedluar/akeyshually/internal/executor"
	"github.com/deprecatedluar/akeyshually/internal/keys"
	evdev "github.com/holoplot/go-evdev"
)

// AbsInfoMap stores metadata for each absolute axis
type AbsInfoMap map[uint16]evdev.AbsInfo

// AccumulatorMap tracks accumulated deltas per axis direction
type AccumulatorMap map[string]float64

// PrevValuesMap tracks previous raw values for delta calculation
type PrevValuesMap map[uint16]int32

// BuildAbsInfoMap queries AbsInfo for all EV_ABS axes at device open
func BuildAbsInfoMap(device *evdev.InputDevice) AbsInfoMap {
	absInfoMap := make(AbsInfoMap)

	// Query all absolute axes supported by the device
	if infos, err := device.AbsInfos(); err == nil {
		for code, info := range infos {
			absInfoMap[uint16(code)] = info
		}
	}

	return absInfoMap
}

// IsAbsAxis returns true if the code is an EV_ABS axis
func IsAbsAxis(code uint16) bool {
	return code < 0x40 // ABS codes are 0x00-0x3f
}

// IsDigitalAxis returns true if max-min <= 2 (dpad detection)
func IsDigitalAxis(info evdev.AbsInfo) bool {
	return (info.Maximum - info.Minimum) <= 2
}

// GetAxisDirection returns "+", "-", or "" based on delta relative to flat zone
func GetAxisDirection(prev, curr int32, info evdev.AbsInfo) string {
	delta := curr - prev

	// Calculate center point
	center := (info.Maximum + info.Minimum) / 2

	// Check if current value is within flat zone of center
	if info.Flat > 0 {
		distanceFromCenter := curr - center
		if distanceFromCenter < 0 {
			distanceFromCenter = -distanceFromCenter
		}
		if distanceFromCenter <= info.Flat {
			return "" // Within deadzone
		}
	}

	if delta > 0 {
		return "+"
	} else if delta < 0 {
		return "-"
	}

	return ""
}

// HandleAbs processes an EV_ABS event
// Returns true if event should be suppressed, false if it should be forwarded
func HandleAbs(
	code uint16,
	value int32,
	absInfoMap AbsInfoMap,
	accumulators AccumulatorMap,
	prevValues PrevValuesMap,
	contactState *bool,
	cfg *config.Config,
	execCtx executor.ExecContext,
) bool {
	axisName := keys.GetAbsName(code)
	common.LogDebug("[ABS] code=%s(%d) value=%d contactState=%v", axisName, code, value, *contactState)

	// ABS_MISC: contact detection (15=touching, 0=lifted)
	if code == uint16(evdev.ABS_MISC) {
		if value == 15 {
			*contactState = true
		} else if value == 0 {
			*contactState = false
			// Reset all accumulators and previous values on lift
			for key := range accumulators {
				delete(accumulators, key)
			}
			for key := range prevValues {
				delete(prevValues, key)
			}
		}
		return false // Always forward ABS_MISC
	}

	// Look up axis info
	info, exists := absInfoMap[code]
	if !exists {
		return false // Unknown axis, forward it
	}

	// Digital axis: synthesize press/release directly
	if IsDigitalAxis(info) {
		// For digital axes (dpads), treat as key press/release
		// Value of 0 = neutral, -1 or +1 = direction
		if value == 0 {
			// Release - would need to track which direction was pressed
			// For now, just forward (behaviors will come in Phase 3)
			return false
		}
		// Non-zero value on digital axis - forward for now
		return false
	}

	// Analog axis: accumulate delta only if in contact
	// Skip if not in contact (for touchpad/touchstrip)
	if code == uint16(evdev.ABS_X) || code == uint16(evdev.ABS_Y) ||
		code == uint16(evdev.ABS_RX) || code == uint16(evdev.ABS_RY) || code == uint16(evdev.ABS_RZ) {
		if !*contactState {
			common.LogDebug("[ABS] Skipping %s - not in contact", axisName)
			return false
		}
	}

	// Get previous value for delta calculation
	prev, hasPrev := prevValues[code]
	if !hasPrev {
		// First reading - just store and skip
		prevValues[code] = value
		return false
	}

	// Calculate direction
	direction := GetAxisDirection(prev, value, info)
	if direction == "" {
		// Within deadzone or no movement
		prevValues[code] = value
		return false
	}

	// Update previous value
	prevValues[code] = value

	// Build combo string (e.g., "rx+", "y-")
	comboAxisName := axisName
	// Strip ABS_ prefix for cleaner combo names
	if len(comboAxisName) > 4 && comboAxisName[:4] == "ABS_" {
		comboAxisName = comboAxisName[4:]
	}
	// Lowercase to match config normalization
	combo := strings.ToLower(comboAxisName + direction)

	// Accumulate delta (absolute value since direction is in the key)
	delta := value - prev
	if delta < 0 {
		delta = -delta
	}
	accumulators[combo] += float64(delta)
	common.LogDebug("[ABS] %s: delta=%d, accumulated=%.2f", combo, delta, accumulators[combo])

	return false // Don't suppress ABS events (Phase 2: plain accumulation only)
}

// FlushAbs is called on every SYN_REPORT frame
// Checks threshold crossings and fires commands
func FlushAbs(
	accumulators AccumulatorMap,
	absInfoMap AbsInfoMap,
	cfg *config.Config,
	execCtx executor.ExecContext,
) {
	// Default sensitivity: 10 fires per full sweep
	defaultSensitivity := 10.0

	for combo, accumulated := range accumulators {
		// Parse combo to extract axis name (strip direction suffix)
		axisName := combo[:len(combo)-1] // Remove +/-
		direction := combo[len(combo)-1:] // Get +/-

		// Find matching shortcut in config
		// Combos are stored in cfg.ParsedShortcuts as a map[string][]*ParsedShortcut
		// where the key is the normalized combo string
		comboKey := axisName + direction
		common.LogDebug("[ABS] FlushAbs: combo=%q comboKey=%q", combo, comboKey)
		var matchedShortcut *config.ParsedShortcut
		if shortcuts, exists := cfg.ParsedShortcuts[comboKey]; exists && len(shortcuts) > 0 {
			// Use the first shortcut for this combo (Phase 2: plain firing only)
			matchedShortcut = shortcuts[0]
			common.LogDebug("[ABS] FlushAbs: found shortcut for %q", comboKey)
		}

		if matchedShortcut == nil {
			common.LogDebug("[ABS] FlushAbs: NO shortcut found for %q", comboKey)
			continue // No shortcut configured for this axis direction
		}

		// Get sensitivity (use default for now, Phase 2 doesn't parse inline syntax yet)
		sensitivity := defaultSensitivity

		// Calculate threshold based on axis range
		// Find the axis code from the name (case-insensitive match)
		var threshold float64
		for code, info := range absInfoMap {
			absName := strings.ToLower(keys.GetAbsName(code))
			targetName := strings.ToLower(axisName)
			if absName == "abs_"+targetName || absName == targetName {
				axisRange := float64(info.Maximum - info.Minimum)
				threshold = axisRange / sensitivity
				common.LogDebug("[ABS] FlushAbs: axis %s range=%v threshold=%.2f", axisName, axisRange, threshold)
				break
			}
		}

		if threshold == 0 {
			common.LogDebug("[ABS] FlushAbs: threshold is 0, skipping")
			continue // Couldn't determine threshold
		}

		common.LogDebug("[ABS] FlushAbs: accumulated=%.2f threshold=%.2f", accumulated, threshold)

		// Check threshold crossing
		if accumulated >= threshold {
			// Fire command (Phase 2: use first command only, behaviors come later)
			numFires := int(accumulated / threshold)
			common.LogDebug("[ABS] %s: threshold crossed! accumulated=%.2f threshold=%.2f numFires=%d", combo, accumulated, threshold, numFires)
			if len(matchedShortcut.Commands) > 0 {
				for i := 0; i < numFires; i++ {
					executor.Run(matchedShortcut.Commands[0], execCtx)
				}
			}

			// Reset accumulator (subtract fired amount, keep remainder)
			accumulators[combo] = accumulated - (float64(numFires) * threshold)
		}
	}
}
