package internal

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/deprecatedluar/akeyshually/internal/matcher"
	evdev "github.com/holoplot/go-evdev"
)

const (
	reconnectMaxAttempts = 30
	reconnectInterval    = 2 * time.Second

	virtualDeviceSuffix = " (" + appName + ")"
	injectorDeviceName  = appName + "-injector"
	injectorKeyCount    = 255
	injectorBusType     = 0x03
	injectorVendorID    = 0x0001
	injectorProductID   = 0x0001
	injectorVersion     = 1
)

var knownRemappers = []string{"keyd", "kanata", "kmonad", "xremap"}

type KeyboardPair struct {
	Physical *evdev.InputDevice
	Virtual  *evdev.InputDevice
}

type KeyHandler func(code uint16, value int32) bool

func FindKeyboards() ([]KeyboardPair, error) {
	paths, err := evdev.ListDevicePaths()
	if err != nil {
		return nil, fmt.Errorf("failed to list input devices: %w", err)
	}

	var remappers []*evdev.InputDevice
	var keyboards []*evdev.InputDevice
	var buttonDevices []*evdev.InputDevice

	LogDebug("Scanning %d input devices...", len(paths))

	for _, path := range paths {
		dev, err := evdev.Open(path.Path)
		if err != nil {
			continue
		}

		name, _ := dev.Name()

		// Skip our own virtual keyboards
		if strings.Contains(strings.ToLower(name), appName) {
			dev.Close()
			continue
		}

		// Check remappers first (they don't always have EV_REP)
		if isRemapperVirtual(dev) {
			hasKey := hasKeyCapability(dev)
			hasAlpha := hasAlphabetKeys(dev)
			LogDebug("Found remapper: %s (hasKey=%v, hasAlpha=%v)", name, hasKey, hasAlpha)
			if hasKey && hasAlpha {
				remappers = append(remappers, dev)
				continue
			}
		}

		// Button devices (phone hardware buttons, media keys)
		if isButtonDevice(dev) {
			LogDebug("Found button device: %s", name)
			buttonDevices = append(buttonDevices, dev)
			continue
		}

		// Physical keyboards need EV_REP
		if isKeyboard(dev) {
			LogDebug("Found physical keyboard: %s", name)
			keyboards = append(keyboards, dev)
			continue
		}

		dev.Close()
	}

	// Prefer remapper virtual keyboards (keyd/kanata grab physical ones)
	// Button devices are always included (they handle independent keys like brightness)
	var devicesToGrab []*evdev.InputDevice
	if len(remappers) > 0 {
		for _, dev := range keyboards {
			dev.Close()
		}
		devicesToGrab = append(remappers, buttonDevices...)
	} else {
		if len(keyboards) == 0 && len(buttonDevices) == 0 {
			return nil, fmt.Errorf("no keyboards detected")
		}
		devicesToGrab = append(keyboards, buttonDevices...)
	}

	// Grab and clone each keyboard
	var pairs []KeyboardPair
	for _, physical := range devicesToGrab {
		name, _ := physical.Name()

		// Grab for exclusive access
		if err := physical.Grab(); err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] Failed to grab %s: %v\n", name, err)
			physical.Close()
			continue
		}

		// Clone to create virtual keyboard
		virtual, err := evdev.CloneDevice(name+virtualDeviceSuffix, physical)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] Failed to clone %s: %v\n", name, err)
			physical.Ungrab()
			physical.Close()
			continue
		}

		pairs = append(pairs, KeyboardPair{
			Physical: physical,
			Virtual:  virtual,
		})
	}

	if len(pairs) == 0 {
		return nil, fmt.Errorf("no keyboards could be grabbed and cloned")
	}

	return pairs, nil
}

func isKeyboard(dev *evdev.InputDevice) bool {
	return hasKeyCapability(dev) && hasRepCapability(dev) && hasAlphabetKeys(dev)
}

// isButtonDevice detects hardware buttons (volume, power) that send EV_KEY events
// but lack EV_REP and full keyboard layout
func isButtonDevice(dev *evdev.InputDevice) bool {
	if !hasKeyCapability(dev) {
		return false
	}

	// Button devices don't have key repeat
	if hasRepCapability(dev) {
		return false
	}

	capableKeys := dev.CapableEvents(evdev.EV_KEY)
	if len(capableKeys) == 0 {
		return false
	}

	keyMap := make(map[evdev.EvCode]bool)
	for _, key := range capableKeys {
		keyMap[key] = true
	}

	// Must have at least one button-type key
	buttonKeys := []evdev.EvCode{
		evdev.KEY_VOLUMEUP,
		evdev.KEY_VOLUMEDOWN,
		evdev.KEY_POWER,
		evdev.KEY_MUTE,
		evdev.KEY_BRIGHTNESSUP,
		evdev.KEY_BRIGHTNESSDOWN,
	}

	for _, key := range buttonKeys {
		if keyMap[key] {
			return true
		}
	}

	return false
}

func hasCapability(dev *evdev.InputDevice, typ evdev.EvType) bool {
	for _, t := range dev.CapableTypes() {
		if t == typ {
			return true
		}
	}
	return false
}

func hasKeyCapability(dev *evdev.InputDevice) bool { return hasCapability(dev, evdev.EV_KEY) }
func hasRepCapability(dev *evdev.InputDevice) bool { return hasCapability(dev, evdev.EV_REP) }

func isRemapperVirtual(dev *evdev.InputDevice) bool {
	name, _ := dev.Name()
	nameLower := strings.ToLower(name)

	for _, pattern := range knownRemappers {
		if strings.Contains(nameLower, pattern) {
			return true
		}
	}
	return false
}

func hasAlphabetKeys(dev *evdev.InputDevice) bool {
	capableKeys := dev.CapableEvents(evdev.EV_KEY)
	if len(capableKeys) == 0 {
		return false
	}

	keyMap := make(map[evdev.EvCode]bool)
	for _, key := range capableKeys {
		keyMap[key] = true
	}

	// Check for a representative set of keys, e.g., A-Z
	for key := evdev.EvCode(evdev.KEY_A); key <= evdev.EvCode(evdev.KEY_Z); key++ {
		if !keyMap[key] {
			return false
		}
	}

	return true
}

func Listen(pair KeyboardPair, handler KeyHandler) error {
	for {
		event, err := pair.Physical.ReadOne()
		if err != nil {
			if err == syscall.ENODEV {
				return fmt.Errorf("device disconnected")
			}
			if err == syscall.EACCES {
				fmt.Fprintf(os.Stderr, "Permission denied. Add user to input group:\n")
				fmt.Fprintf(os.Stderr, "  sudo usermod -aG input $USER\n")
				fmt.Fprintf(os.Stderr, "Then logout and login again.\n")
				return err
			}
			return fmt.Errorf("read error: %w", err)
		}

		if event.Type == evdev.EV_KEY {
			matched := handler(uint16(event.Code), event.Value)
			if !matched {
				pair.Virtual.WriteOne(event)
			}
		} else {
			// Forward all non-key events immediately
			if event.Type == evdev.EV_ABS && IsLoggingEnabled() {
				LogKey(matcher.GetAbsName(uint16(event.Code)), uint16(event.Code))
			}
			pair.Virtual.WriteOne(event)
		}
	}
}

func Cleanup(pair KeyboardPair) {
	pair.Physical.Ungrab()
	evdev.DestroyDevice(pair.Virtual)
	pair.Physical.Close()
}

// FindDeclaredDevices finds and grabs devices whose names contain any of the given substrings.
func FindDeclaredDevices(matches []string) ([]KeyboardPair, error) {
	paths, err := evdev.ListDevicePaths()
	if err != nil {
		return nil, fmt.Errorf("failed to list input devices: %w", err)
	}

	var pairs []KeyboardPair

	for _, path := range paths {
		dev, err := evdev.Open(path.Path)
		if err != nil {
			continue
		}

		name, _ := dev.Name()
		nameLower := strings.ToLower(name)

		// Skip our own virtual devices
		if strings.Contains(nameLower, appName) {
			dev.Close()
			continue
		}

		// Check if device name matches any declared pattern (case-insensitive)
		matched := false
		for _, match := range matches {
			if strings.Contains(nameLower, strings.ToLower(match)) {
				matched = true
				break
			}
		}

		if !matched {
			dev.Close()
			continue
		}

		LogDebug("Found declared device: %s", name)

		if err := dev.Grab(); err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] Failed to grab %s: %v\n", name, err)
			dev.Close()
			continue
		}

		virtual, err := evdev.CloneDevice(name+virtualDeviceSuffix, dev)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] Failed to clone %s: %v\n", name, err)
			dev.Ungrab()
			dev.Close()
			continue
		}

		pairs = append(pairs, KeyboardPair{Physical: dev, Virtual: virtual})
	}

	return pairs, nil
}

// ListenWithReconnect wraps Listen with automatic reconnection on device disconnect.
// On ENODEV it calls findFn every 2 seconds (up to 30 attempts) to find the device by name.
func ListenWithReconnect(pair KeyboardPair, handler KeyHandler, findFn func() ([]KeyboardPair, error), deviceName string) error {
	for {
		err := Listen(pair, handler)
		if err == nil {
			return nil
		}

		// Only retry on device disconnect
		if !strings.Contains(err.Error(), "device disconnected") {
			return err
		}

		Cleanup(pair)
		LogDebug("Device %q disconnected, attempting reconnect...", deviceName)

		var newPair *KeyboardPair
		for attempt := 1; attempt <= reconnectMaxAttempts; attempt++ {
			time.Sleep(reconnectInterval)

			pairs, err := findFn()
			if err != nil {
				LogDebug("Reconnect attempt %d/%d: %v", attempt, reconnectMaxAttempts, err)
				continue
			}

			for _, p := range pairs {
				name, _ := p.Physical.Name()
				if name == deviceName && newPair == nil {
					found := p
					newPair = &found
				} else {
					Cleanup(p)
				}
			}

			if newPair != nil {
				break
			}
			LogDebug("Reconnect attempt %d/%d: %q not found", attempt, reconnectMaxAttempts, deviceName)
		}

		if newPair == nil {
			return fmt.Errorf("device %q did not reconnect within 1 minute", deviceName)
		}

		pair = *newPair
		fmt.Printf("  - Reconnected: %s\n", deviceName)
	}
}

// FindMice detects mouse devices (read-only, no grabbing)
func FindMice() ([]*evdev.InputDevice, error) {
	paths, err := evdev.ListDevicePaths()
	if err != nil {
		return nil, fmt.Errorf("failed to list input devices: %w", err)
	}

	var mice []*evdev.InputDevice

	for _, path := range paths {
		dev, err := evdev.Open(path.Path)
		if err != nil {
			continue
		}

		name, _ := dev.Name()

		nameLower := strings.ToLower(name)

		// Skip virtual keyboards we created
		if strings.Contains(nameLower, appName) {
			dev.Close()
			continue
		}

		// Skip remapper virtual devices (they have BTN_LEFT but aren't mice)
		if isRemapperVirtual(dev) {
			dev.Close()
			continue
		}

		// Mice have EV_KEY + mouse buttons but no EV_REP
		if isMouse(dev) {
			LogDebug("Found mouse: %s", name)
			mice = append(mice, dev)
			continue
		}

		dev.Close()
	}

	return mice, nil
}

func isMouse(dev *evdev.InputDevice) bool {
	if !hasKeyCapability(dev) {
		return false
	}

	// Mice don't have EV_REP
	if hasRepCapability(dev) {
		return false
	}

	// Check for mouse buttons
	capableKeys := dev.CapableEvents(evdev.EV_KEY)
	if len(capableKeys) == 0 {
		return false
	}

	keyMap := make(map[evdev.EvCode]bool)
	for _, key := range capableKeys {
		keyMap[key] = true
	}

	// Must have at least left mouse button
	return keyMap[evdev.EvCode(evdev.BTN_LEFT)]
}

func isClickButton(code evdev.EvCode) bool {
	switch code {
	case evdev.EvCode(evdev.BTN_LEFT), evdev.EvCode(evdev.BTN_RIGHT), evdev.EvCode(evdev.BTN_MIDDLE):
		return true
	}
	return false
}

type MouseButtonHandler func()

// ListenMouse monitors mouse button presses (read-only, no grabbing)
func ListenMouse(dev *evdev.InputDevice, handler MouseButtonHandler) error {
	for {
		event, err := dev.ReadOne()
		if err != nil {
			if err == syscall.ENODEV {
				return fmt.Errorf("device disconnected")
			}
			return fmt.Errorf("read error: %w", err)
		}

		// Trigger only on actual mouse button clicks (not BTN_TOOL_FINGER, BTN_TOUCH, etc.)
		if event.Type == evdev.EV_KEY && event.Value == 1 && isClickButton(event.Code) {
			handler()
		}
	}
}

// CreateKeyInjector creates a shared uinput keyboard with full key capabilities for remap injection.
func CreateKeyInjector() (*evdev.InputDevice, error) {
	codes := make([]evdev.EvCode, injectorKeyCount)
	for i := range codes {
		codes[i] = evdev.EvCode(i + 1)
	}
	return evdev.CreateDevice(injectorDeviceName, evdev.InputID{BusType: injectorBusType, Vendor: injectorVendorID, Product: injectorProductID, Version: injectorVersion},
		map[evdev.EvType][]evdev.EvCode{evdev.EV_KEY: codes})
}

// EmitKeysDown emits keydown events for all keys in a combo and returns the codes pressed.
// Skips modifier keys already held. Used for whileheld remap — caller must call EmitKeysUp to release.
func EmitKeysDown(injector *evdev.InputDevice, combo string, heldModifiers matcher.ModifierState) []uint16 {
	if injector == nil {
		return nil
	}
	parts := strings.Split(combo, "+")
	var codes []uint16
	for _, part := range parts {
		code, ok := matcher.ResolveKeyCode(strings.TrimSpace(part))
		if !ok {
			fmt.Fprintf(os.Stderr, "Remap error: unknown key %q\n", part)
			continue
		}
		if !isModifierHeld(code, heldModifiers) {
			injector.WriteOne(&evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.EvCode(code), Value: 1})
			codes = append(codes, code)
		}
	}
	injector.WriteOne(&evdev.InputEvent{Type: evdev.EV_SYN, Code: evdev.SYN_REPORT, Value: 0})
	return codes
}

// EmitKeysUp releases keys previously pressed by EmitKeysDown.
func EmitKeysUp(injector *evdev.InputDevice, codes []uint16) {
	if injector == nil {
		return
	}
	for i := len(codes) - 1; i >= 0; i-- {
		injector.WriteOne(&evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.EvCode(codes[i]), Value: 0})
	}
	injector.WriteOne(&evdev.InputEvent{Type: evdev.EV_SYN, Code: evdev.SYN_REPORT, Value: 0})
}

// EmitKeyCombo emits a key combo on the injector device, skipping modifiers already held.
func EmitKeyCombo(injector *evdev.InputDevice, combo string, heldModifiers matcher.ModifierState) error {
	parts := strings.Split(combo, "+")

	var modCodes []uint16
	var finalCode uint16

	for i, part := range parts {
		code, ok := matcher.ResolveKeyCode(strings.TrimSpace(part))
		if !ok {
			return fmt.Errorf("unknown key: %q", part)
		}
		if i < len(parts)-1 {
			modCodes = append(modCodes, code)
		} else {
			finalCode = code
		}
	}

	for _, code := range modCodes {
		if !isModifierHeld(code, heldModifiers) {
			injector.WriteOne(&evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.EvCode(code), Value: 1})
		}
	}
	injector.WriteOne(&evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.EvCode(finalCode), Value: 1})
	injector.WriteOne(&evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.EvCode(finalCode), Value: 0})
	for i := len(modCodes) - 1; i >= 0; i-- {
		if !isModifierHeld(modCodes[i], heldModifiers) {
			injector.WriteOne(&evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.EvCode(modCodes[i]), Value: 0})
		}
	}
	injector.WriteOne(&evdev.InputEvent{Type: evdev.EV_SYN, Code: evdev.SYN_REPORT, Value: 0})
	return nil
}

func isModifierHeld(code uint16, held matcher.ModifierState) bool {
	switch evdev.EvCode(code) {
	case evdev.KEY_LEFTMETA, evdev.KEY_RIGHTMETA:
		return held.Super
	case evdev.KEY_LEFTCTRL, evdev.KEY_RIGHTCTRL:
		return held.Ctrl
	case evdev.KEY_LEFTALT, evdev.KEY_RIGHTALT:
		return held.Alt
	case evdev.KEY_LEFTSHIFT, evdev.KEY_RIGHTSHIFT:
		return held.Shift
	}
	return false
}

// IsMediaKey checks if a keycode is a media key (volume, brightness, playback)
func IsMediaKey(code uint16) bool {
	switch code {
	case evdev.KEY_VOLUMEUP,
		evdev.KEY_VOLUMEDOWN,
		evdev.KEY_MUTE,
		evdev.KEY_BRIGHTNESSUP,
		evdev.KEY_BRIGHTNESSDOWN,
		evdev.KEY_PLAYPAUSE,
		evdev.KEY_NEXTSONG,
		evdev.KEY_PREVIOUSSONG,
		evdev.KEY_STOPCD,
		evdev.KEY_PLAYCD,
		evdev.KEY_PAUSECD:
		return true
	default:
		return false
	}
}
