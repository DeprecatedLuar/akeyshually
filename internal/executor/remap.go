package executor

import (
	"fmt"
	"os"
	"strings"

	"github.com/deprecatedluar/akeyshually/internal/keys"
	"github.com/deprecatedluar/akeyshually/internal/matcher"
	evdev "github.com/holoplot/go-evdev"
)

// Remap modes
const (
	RemapTap         = ">"
	RemapHoldForever = ">>"
	RemapKeyUp       = "<"
	RemapReleaseAll  = "<<"
)

func runRemap(cmd string, ctx ExecContext) {
	if ctx.Injector == nil {
		fmt.Fprintf(os.Stderr, "Remap error: no injector device available\n")
		return
	}

	switch {
	case cmd == RemapReleaseAll:
		// Release all persistent held keys
		if ctx.LoopState == nil {
			return
		}
		ctx.LoopState.Mu.Lock()
		for key, codes := range ctx.LoopState.PersistentHeld {
			EmitKeysUp(ctx.Injector, codes)
			delete(ctx.LoopState.PersistentHeld, key)
		}
		ctx.LoopState.Mu.Unlock()

	case strings.HasPrefix(cmd, RemapHoldForever):
		// Hold keys forever (>>)
		if ctx.LoopState == nil {
			fmt.Fprintf(os.Stderr, "Remap error: >> requires loopState\n")
			return
		}
		target := cmd[2:]
		ctx.LoopState.Mu.Lock()
		codes := EmitKeysDown(ctx.Injector, target, ctx.Modifiers)
		if len(codes) > 0 {
			ctx.LoopState.PersistentHeld[target] = codes
		}
		ctx.LoopState.Mu.Unlock()

	case strings.HasPrefix(cmd, RemapTap):
		// Tap key combo (>)
		target := cmd[1:]
		// Check for scroll/wheel output aliases
		if emitScrollWheel(ctx.Injector, target) {
			return
		}
		if err := EmitKeyCombo(ctx.Injector, target, ctx.Modifiers); err != nil {
			fmt.Fprintf(os.Stderr, "Remap error: %v\n", err)
		}

	case strings.HasPrefix(cmd, RemapKeyUp):
		// Release single key (<)
		target := cmd[1:]
		code, ok := keys.ResolveKeyCode(target)
		if !ok {
			fmt.Fprintf(os.Stderr, "Remap error: unknown key %q\n", target)
			return
		}
		EmitKeysUp(ctx.Injector, []uint16{code})
	}
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
		code, ok := keys.ResolveKeyCode(strings.TrimSpace(part))
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
		code, ok := keys.ResolveKeyCode(strings.TrimSpace(part))
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

// emitScrollWheel emits REL_WHEEL/REL_HWHEEL events for scroll/wheel output aliases
// Returns true if the target was a scroll/wheel alias, false otherwise
func emitScrollWheel(injector *evdev.InputDevice, target string) bool {
	if injector == nil {
		return false
	}

	target = strings.ToLower(strings.TrimSpace(target))

	switch target {
	case "scrollup", "wheelup":
		injector.WriteOne(&evdev.InputEvent{Type: evdev.EV_REL, Code: evdev.REL_WHEEL, Value: 1})
		injector.WriteOne(&evdev.InputEvent{Type: evdev.EV_REL, Code: evdev.REL_WHEEL_HI_RES, Value: 120})
		injector.WriteOne(&evdev.InputEvent{Type: evdev.EV_SYN, Code: evdev.SYN_REPORT, Value: 0})
		return true

	case "scrolldown", "wheeldown":
		injector.WriteOne(&evdev.InputEvent{Type: evdev.EV_REL, Code: evdev.REL_WHEEL, Value: -1})
		injector.WriteOne(&evdev.InputEvent{Type: evdev.EV_REL, Code: evdev.REL_WHEEL_HI_RES, Value: -120})
		injector.WriteOne(&evdev.InputEvent{Type: evdev.EV_SYN, Code: evdev.SYN_REPORT, Value: 0})
		return true

	case "scrollleft", "wheelleft":
		injector.WriteOne(&evdev.InputEvent{Type: evdev.EV_REL, Code: evdev.REL_HWHEEL, Value: -1})
		injector.WriteOne(&evdev.InputEvent{Type: evdev.EV_SYN, Code: evdev.SYN_REPORT, Value: 0})
		return true

	case "scrollright", "wheelright":
		injector.WriteOne(&evdev.InputEvent{Type: evdev.EV_REL, Code: evdev.REL_HWHEEL, Value: 1})
		injector.WriteOne(&evdev.InputEvent{Type: evdev.EV_SYN, Code: evdev.SYN_REPORT, Value: 0})
		return true
	}

	return false
}
