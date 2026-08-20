package executor

import (
	"errors"
	"fmt"
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

func runRemap(cmd string, ctx ExecContext) error {
	switch {
	case cmd == RemapReleaseAll:
		if ctx.LoopState == nil {
			return fmt.Errorf("%s requires loop state", RemapReleaseAll)
		}
		ctx.LoopState.Mu.Lock()
		defer ctx.LoopState.Mu.Unlock()
		var releaseErrors []error
		for key, held := range ctx.LoopState.PersistentHeld {
			if err := EmitKeysUp(held.Output, held.Codes); err != nil {
				releaseErrors = append(releaseErrors, fmt.Errorf("release %q: %w", key, err))
			}
			delete(ctx.LoopState.PersistentHeld, key)
		}
		return errors.Join(releaseErrors...)

	case strings.HasPrefix(cmd, RemapHoldForever):
		if ctx.LoopState == nil {
			return fmt.Errorf("%s requires loop state", RemapHoldForever)
		}
		target := cmd[2:]
		output, err := outputForTarget(ctx.Outputs, target)
		if err != nil {
			return err
		}
		ctx.LoopState.Mu.Lock()
		defer ctx.LoopState.Mu.Unlock()
		codes, err := EmitKeysDown(output, target, ctx.Modifiers)
		if err != nil {
			return err
		}
		if len(codes) > 0 {
			ctx.LoopState.PersistentHeld[target] = heldOutput{Output: output, Codes: codes}
		}
		return nil

	case strings.HasPrefix(cmd, RemapTap):
		target := cmd[1:]
		if matched, err := emitScrollWheel(ctx.Outputs.Pointer, target); matched {
			return err
		}
		return EmitKeyCombo(ctx.Outputs, target, ctx.Modifiers)

	case strings.HasPrefix(cmd, RemapKeyUp):
		target := cmd[1:]
		code, ok := keys.ResolveKeyCode(target)
		if !ok {
			return fmt.Errorf("unknown key %q", target)
		}
		output := OutputForCode(ctx.Outputs, code)
		if err := EmitKeysUp(output, []uint16{code}); err != nil {
			return err
		}
		if ctx.LoopState != nil {
			ctx.LoopState.Mu.Lock()
			delete(ctx.LoopState.PersistentHeld, target)
			ctx.LoopState.Mu.Unlock()
		}
		return nil
	}
	return fmt.Errorf("unknown remap command %q", cmd)
}

// EmitKeysDown emits keydown events for all keys in a combo and returns the codes pressed.
// Skips modifier keys already held. Used for whileheld remap — caller must call EmitKeysUp to release.
func EmitKeysDown(output *EventSink, combo string, heldModifiers matcher.ModifierState) ([]uint16, error) {
	parts := strings.Split(combo, "+")
	var codes []uint16
	var events []evdev.InputEvent
	for _, part := range parts {
		code, ok := keys.ResolveKeyCode(strings.TrimSpace(part))
		if !ok {
			return nil, fmt.Errorf("unknown key %q", part)
		}
		if !isModifierHeld(code, heldModifiers) {
			events = append(events, evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.EvCode(code), Value: 1})
			codes = append(codes, code)
		}
	}
	if err := output.WriteFrame(events...); err != nil {
		return nil, err
	}
	return codes, nil
}

// EmitKeysUp releases keys previously pressed by EmitKeysDown.
func EmitKeysUp(output *EventSink, codes []uint16) error {
	events := make([]evdev.InputEvent, 0, len(codes))
	for i := len(codes) - 1; i >= 0; i-- {
		events = append(events, evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.EvCode(codes[i]), Value: 0})
	}
	return output.WriteFrame(events...)
}

// EmitKeyCombo emits a key combo, routing the final key or button to its
// focused output while skipping modifiers already held by the user.
func EmitKeyCombo(outputs Outputs, combo string, heldModifiers matcher.ModifierState) error {
	parts := strings.Split(combo, "+")

	var modCodes []uint16
	var finalCode uint16

	for i, part := range parts {
		code, ok := keys.ResolveKeyCode(strings.TrimSpace(part))
		if !ok {
			return fmt.Errorf("unknown key: %q", part)
		}
		if i < len(parts)-1 {
			if isPointerButton(code) {
				return fmt.Errorf("pointer button %q cannot be a modifier", part)
			}
			modCodes = append(modCodes, code)
		} else {
			finalCode = code
		}
	}

	finalOutput := OutputForCode(outputs, finalCode)
	if finalOutput == outputs.Keyboard {
		events := make([]evdev.InputEvent, 0, len(modCodes)*2+2)
		for _, code := range modCodes {
			if !isModifierHeld(code, heldModifiers) {
				events = append(events, evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.EvCode(code), Value: 1})
			}
		}
		events = append(events,
			evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.EvCode(finalCode), Value: 1},
			evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.EvCode(finalCode), Value: 0},
		)
		for i := len(modCodes) - 1; i >= 0; i-- {
			if !isModifierHeld(modCodes[i], heldModifiers) {
				events = append(events, evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.EvCode(modCodes[i]), Value: 0})
			}
		}
		return finalOutput.WriteFrame(events...)
	}

	var pressedModifiers []uint16
	var modifierDownEvents []evdev.InputEvent
	for _, code := range modCodes {
		if !isModifierHeld(code, heldModifiers) {
			pressedModifiers = append(pressedModifiers, code)
			modifierDownEvents = append(modifierDownEvents,
				evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.EvCode(code), Value: 1})
		}
	}
	if len(modifierDownEvents) > 0 {
		if err := outputs.Keyboard.WriteFrame(modifierDownEvents...); err != nil {
			return err
		}
	}

	pointerErr := finalOutput.WriteFrame(evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.EvCode(finalCode), Value: 1})
	if pointerErr == nil {
		pointerErr = finalOutput.WriteFrame(evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.EvCode(finalCode), Value: 0})
	}
	if len(pressedModifiers) == 0 {
		return pointerErr
	}

	modifierUpEvents := make([]evdev.InputEvent, 0, len(pressedModifiers))
	for i := len(pressedModifiers) - 1; i >= 0; i-- {
		modifierUpEvents = append(modifierUpEvents,
			evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.EvCode(pressedModifiers[i]), Value: 0})
	}
	releaseErr := outputs.Keyboard.WriteFrame(modifierUpEvents...)
	return errors.Join(pointerErr, releaseErr)
}

// remapHoldTarget returns the key/button target for a sustained hold remap
// (either ">>target" or plain ">target" used under a span trigger) and
// whether cmd is such a remap.
func remapHoldTarget(cmd string) (target string, ok bool) {
	switch {
	case strings.HasPrefix(cmd, RemapHoldForever):
		return cmd[2:], true
	case strings.HasPrefix(cmd, RemapTap):
		return cmd[1:], true
	default:
		return "", false
	}
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
func emitScrollWheel(output *EventSink, target string) (bool, error) {
	target = strings.ToLower(strings.TrimSpace(target))

	switch target {
	case "scrollup", "wheelup":
		return true, output.WriteFrame(
			evdev.InputEvent{Type: evdev.EV_REL, Code: evdev.REL_WHEEL, Value: 1},
			evdev.InputEvent{Type: evdev.EV_REL, Code: evdev.REL_WHEEL_HI_RES, Value: 120},
		)

	case "scrolldown", "wheeldown":
		return true, output.WriteFrame(
			evdev.InputEvent{Type: evdev.EV_REL, Code: evdev.REL_WHEEL, Value: -1},
			evdev.InputEvent{Type: evdev.EV_REL, Code: evdev.REL_WHEEL_HI_RES, Value: -120},
		)

	case "scrollleft", "wheelleft":
		return true, output.WriteFrame(
			evdev.InputEvent{Type: evdev.EV_REL, Code: evdev.REL_HWHEEL, Value: -1},
		)

	case "scrollright", "wheelright":
		return true, output.WriteFrame(
			evdev.InputEvent{Type: evdev.EV_REL, Code: evdev.REL_HWHEEL, Value: 1},
		)
	}

	return false, nil
}

func outputForTarget(outputs Outputs, target string) (*EventSink, error) {
	parts := strings.Split(target, "+")
	var output *EventSink
	for _, part := range parts {
		code, ok := keys.ResolveKeyCode(strings.TrimSpace(part))
		if !ok {
			return nil, fmt.Errorf("unknown key %q", part)
		}
		candidate := OutputForCode(outputs, code)
		if output != nil && output != candidate {
			return nil, fmt.Errorf("cannot mix keyboard and pointer outputs in %q", target)
		}
		output = candidate
	}
	if output == nil {
		return nil, fmt.Errorf("empty remap target")
	}
	return output, nil
}

// OutputForCode returns the output device that a translated key code should be written to:
// the pointer for mouse buttons, the keyboard for everything else.
func OutputForCode(outputs Outputs, code uint16) *EventSink {
	if isPointerButton(code) {
		return outputs.Pointer
	}
	return outputs.Keyboard
}

func isPointerButton(code uint16) bool {
	switch evdev.EvCode(code) {
	case evdev.BTN_LEFT, evdev.BTN_RIGHT, evdev.BTN_MIDDLE,
		evdev.BTN_SIDE, evdev.BTN_EXTRA, evdev.BTN_FORWARD, evdev.BTN_BACK:
		return true
	default:
		return false
	}
}
