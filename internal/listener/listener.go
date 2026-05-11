package listener

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/deprecatedluar/akeyshually/internal/common"
	evdev "github.com/holoplot/go-evdev"
)

const (
	reconnectMaxAttempts = 30
	reconnectInterval    = 2 * time.Second

	virtualDeviceSuffix = " (" + common.AppName + ")"
	injectorDeviceName  = common.AppName + "-injector"
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

	common.LogDebug("Scanning %d input devices...", len(paths))

	for _, path := range paths {
		dev, err := evdev.Open(path.Path)
		if err != nil {
			continue
		}

		name, _ := dev.Name()

		// Skip our own virtual keyboards
		if strings.Contains(strings.ToLower(name), common.AppName) {
			dev.Close()
			continue
		}

		// Check remappers first (they don't always have EV_REP)
		if isRemapperVirtual(dev) {
			hasKey := hasKeyCapability(dev)
			hasAlpha := hasAlphabetKeys(dev)
			common.LogDebug("Found remapper: %s (hasKey=%v, hasAlpha=%v)", name, hasKey, hasAlpha)
			if hasKey && hasAlpha {
				remappers = append(remappers, dev)
				continue
			}
		}

		// Button devices (phone hardware buttons, media keys)
		if isButtonDevice(dev) {
			common.LogDebug("Found button device: %s", name)
			buttonDevices = append(buttonDevices, dev)
			continue
		}

		// Physical keyboards need EV_REP
		if isKeyboard(dev) {
			common.LogDebug("Found physical keyboard: %s", name)
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

		switch event.Type {
		case evdev.EV_KEY, evdev.EV_ABS:
			matched := handler(uint16(event.Code), event.Value)
			if !matched {
				pair.Virtual.WriteOne(event)
			}
		case evdev.EV_SYN:
			// Use sentinel value 0xFFFF for SYN events (not a valid key/abs code)
			handler(0xFFFF, event.Value)
			// Always forward SYN regardless of handler return
			pair.Virtual.WriteOne(event)
		default:
			// Forward all other events immediately
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
		if strings.Contains(nameLower, common.AppName) {
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

		common.LogDebug("Found declared device: %s", name)

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
		common.LogDebug("Device %q disconnected, attempting reconnect...", deviceName)

		var newPair *KeyboardPair
		for attempt := 1; attempt <= reconnectMaxAttempts; attempt++ {
			time.Sleep(reconnectInterval)

			pairs, err := findFn()
			if err != nil {
				common.LogDebug("Reconnect attempt %d/%d: %v", attempt, reconnectMaxAttempts, err)
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
			common.LogDebug("Reconnect attempt %d/%d: %q not found", attempt, reconnectMaxAttempts, deviceName)
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
		if strings.Contains(nameLower, common.AppName) {
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
			common.LogDebug("Found mouse: %s", name)
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
	case evdev.EvCode(evdev.BTN_LEFT), evdev.EvCode(evdev.BTN_RIGHT), evdev.EvCode(evdev.BTN_MIDDLE),
		evdev.EvCode(evdev.BTN_TOUCH), evdev.EvCode(evdev.BTN_TOOL_FINGER):
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
	// Add mouse button codes (BTN_LEFT through BTN_EXTRA: 0x110-0x116)
	codes = append(codes, evdev.BTN_LEFT, evdev.BTN_RIGHT, evdev.BTN_MIDDLE,
		evdev.BTN_SIDE, evdev.BTN_EXTRA, evdev.BTN_FORWARD, evdev.BTN_BACK)
	return evdev.CreateDevice(injectorDeviceName, evdev.InputID{BusType: injectorBusType, Vendor: injectorVendorID, Product: injectorProductID, Version: injectorVersion},
		map[evdev.EvType][]evdev.EvCode{
			evdev.EV_KEY: codes,
			evdev.EV_REL: []evdev.EvCode{evdev.REL_WHEEL, evdev.REL_HWHEEL, evdev.REL_WHEEL_HI_RES, evdev.REL_HWHEEL_HI_RES},
		})
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
