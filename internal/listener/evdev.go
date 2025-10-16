package listener

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	evdev "github.com/holoplot/go-evdev"
)

type KeyHandler func(code uint16, value int32)

func FindKeyboards() ([]*evdev.InputDevice, error) {
	paths, err := evdev.ListDevicePaths()
	if err != nil {
		return nil, fmt.Errorf("failed to list input devices: %w", err)
	}

	var remappers []*evdev.InputDevice
	var keyboards []*evdev.InputDevice

	fmt.Fprintf(os.Stderr, "[DEBUG] Scanning %d input devices...\n", len(paths))

	for _, path := range paths {
		dev, err := evdev.Open(path.Path)
		if err != nil {
			continue
		}

		name, _ := dev.Name()

		// Check remappers first (they don't always have EV_REP)
		if isRemapperVirtual(dev) {
			hasKey := hasKeyCapability(dev)
			hasAlpha := hasAlphabetKeys(dev)
			fmt.Fprintf(os.Stderr, "[DEBUG] Found remapper: %s (hasKey=%v, hasAlpha=%v)\n", name, hasKey, hasAlpha)
			if hasKey && hasAlpha {
				remappers = append(remappers, dev)
				continue
			}
		}

		// Physical keyboards need EV_REP
		if isKeyboard(dev) {
			fmt.Fprintf(os.Stderr, "[DEBUG] Found physical keyboard: %s\n", name)
			keyboards = append(keyboards, dev)
			continue
		}

		dev.Close()
	}

	// Prefer remapper virtual keyboards (keyd/kanata grab physical ones)
	if len(remappers) > 0 {
		for _, dev := range keyboards {
			dev.Close()
		}
		return remappers, nil
	}

	if len(keyboards) == 0 {
		return nil, fmt.Errorf("no keyboards detected")
	}

	return keyboards, nil
}

func isKeyboard(dev *evdev.InputDevice) bool {
	return hasKeyCapability(dev) && hasRepCapability(dev) && hasAlphabetKeys(dev)
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

func Listen(dev *evdev.InputDevice, handler KeyHandler) error {
	for {
		event, err := dev.ReadOne()
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
			handler(uint16(event.Code), event.Value)
		}
	}
}
