package internal

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	evdev "github.com/holoplot/go-evdev"
)

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

	debug := IsDebugEnabled()
	if debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] Scanning %d input devices...\n", len(paths))
	}

	for _, path := range paths {
		dev, err := evdev.Open(path.Path)
		if err != nil {
			continue
		}

		name, _ := dev.Name()

		// Skip our own virtual keyboards
		if strings.Contains(strings.ToLower(name), "akeyshually") {
			dev.Close()
			continue
		}

		// Check remappers first (they don't always have EV_REP)
		if isRemapperVirtual(dev) {
			hasKey := hasKeyCapability(dev)
			hasAlpha := hasAlphabetKeys(dev)
			if debug {
				fmt.Fprintf(os.Stderr, "[DEBUG] Found remapper: %s (hasKey=%v, hasAlpha=%v)\n", name, hasKey, hasAlpha)
			}
			if hasKey && hasAlpha {
				remappers = append(remappers, dev)
				continue
			}
		}

		// Button devices (phone hardware buttons, media keys)
		if isButtonDevice(dev) {
			if debug {
				fmt.Fprintf(os.Stderr, "[DEBUG] Found button device: %s\n", name)
			}
			buttonDevices = append(buttonDevices, dev)
			continue
		}

		// Physical keyboards need EV_REP
		if isKeyboard(dev) {
			if debug {
				fmt.Fprintf(os.Stderr, "[DEBUG] Found physical keyboard: %s\n", name)
			}
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
		virtual, err := evdev.CloneDevice(name+" (akeyshually)", physical)
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

func hasKeyCapability(dev *evdev.InputDevice) bool {
	types := dev.CapableTypes()
	for _, t := range types {
		if t == evdev.EV_KEY {
			return true
		}
	}
	return false
}

func hasRepCapability(dev *evdev.InputDevice) bool {
	types := dev.CapableTypes()
	for _, t := range types {
		if t == evdev.EV_REP {
			return true
		}
	}
	return false
}

func isRemapperVirtual(dev *evdev.InputDevice) bool {
	name, _ := dev.Name()
	nameLower := strings.ToLower(name)

	remappers := []string{"keyd", "kanata", "kmonad", "xremap"}
	for _, pattern := range remappers {
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
			pair.Virtual.WriteOne(event)
		}
	}
}

func Cleanup(pair KeyboardPair) {
	pair.Physical.Ungrab()
	evdev.DestroyDevice(pair.Virtual)
	pair.Physical.Close()
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

		// Skip virtual keyboards we created
		if strings.Contains(strings.ToLower(name), "akeyshually") {
			dev.Close()
			continue
		}

		// Mice have EV_KEY + mouse buttons but no EV_REP
		if isMouse(dev) {
			if IsDebugEnabled() {
				fmt.Fprintf(os.Stderr, "[DEBUG] Found mouse: %s\n", name)
			}
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

		// Trigger on any mouse button press (value == 1)
		if event.Type == evdev.EV_KEY && event.Value == 1 {
			handler()
		}
	}
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
