package handlers

import (
	"strings"

	"github.com/deprecatedluar/akeyshually/internal/common"
	"github.com/deprecatedluar/akeyshually/internal/config"
	"github.com/deprecatedluar/akeyshually/internal/executor"
	"github.com/deprecatedluar/akeyshually/internal/keys"
	"github.com/deprecatedluar/akeyshually/internal/ladder"
	"github.com/deprecatedluar/akeyshually/internal/matcher"
	"github.com/deprecatedluar/akeyshually/internal/timers"
	evdev "github.com/holoplot/go-evdev"
)

// translationSink identifies which device a translated key's down/up pair
// was written to, so release can replay to the same place press used.
type translationSink int

const (
	sinkKeyboard translationSink = iota // outputs.Keyboard (focused keyboard injector)
	sinkPointer                         // outputs.Pointer (focused pointer injector)
	sinkVirtual                         // the device's own virtual clone
)

// activeTranslation records what a physical key was translated into, so
// release can key off the recorded target instead of re-evaluating the
// combo (which may no longer match once a modifier prefix has been
// released).
type activeTranslation struct {
	code   uint16
	sink   translationSink
	output *executor.EventSink // valid when sink is sinkKeyboard or sinkPointer
}

// Translator performs input-stage remap translation: a physical key whose
// combo resolves through Config.RemapTable is translated into its target
// key at press/release time, before the matcher or ladder ever see it. One
// Translator belongs to a single keyboard, mirroring Matcher's lifecycle.
type Translator struct {
	active            map[uint16]activeTranslation
	virtualKeyCapable map[uint16]bool
}

// NewTranslator creates a Translator for the given keyboard's virtual clone.
// virtual's capabilities mirror the physical device (minus output-only
// types, see cloneWithoutOutputCapabilities), so not every remap target is
// necessarily writable there.
func NewTranslator(virtual *evdev.InputDevice) *Translator {
	capable := make(map[uint16]bool)
	if virtual != nil {
		for _, code := range virtual.CapableEvents(evdev.EV_KEY) {
			capable[uint16(code)] = true
		}
	}
	return &Translator{active: make(map[uint16]activeTranslation), virtualKeyCapable: capable}
}

// TryPress looks up combo in cfg.RemapTable. On a hit it consumes any held
// modifier prefix so it does not leak into the translated output, emits the
// target's keydown, records the translation for release, and returns true.
// Returns false on a miss, leaving the event untouched for normal
// matcher/ladder handling.
//
// Pointer targets always go to outputs.Pointer. Keyboard targets go to the
// device's own virtual clone when it supports the code - staying consistent
// with EmitModifierKey and EmittedModifierTracker, which both already write
// there, since splitting a modifier's down and up across two devices risks
// desync - falling back to outputs.Keyboard otherwise.
//
// When the target is itself one of the four combo modifiers (e.g. "capslock"
// = ">super"), m's modifier state and emittedTracker are updated as if that
// modifier had been pressed for real, so a later "super+m" is recognized by
// this same keyboard's combo matching and its consumption is tracked like
// any other forwarded modifier.
func (t *Translator) TryPress(code uint16, combo string, cfg *config.Config, m *matcher.Matcher, virtual *evdev.InputDevice, outputs executor.Outputs, emittedTracker *timers.EmittedModifierTracker) bool {
	target, ok := cfg.RemapTable[combo]
	if !ok {
		return false
	}
	targetCode, ok := keys.ResolveKeyCode(target)
	if !ok {
		return false
	}

	consumeTranslationModifiers(virtual, combo, emittedTracker)

	sink, output := t.resolveSink(targetCode, virtual, outputs)

	var err error
	switch sink {
	case sinkVirtual:
		err = emitKeyToVirtual(virtual, targetCode, true)
	default:
		err = output.WriteFrame(evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.EvCode(targetCode), Value: 1})
	}
	if err != nil {
		common.LogDebug("remap translate: press %s -> %s failed: %v", combo, target, err)
		return false
	}

	if matcher.IsModifierKey(targetCode) {
		m.UpdateModifierState(targetCode, true)
		emittedTracker.MarkDown(keys.GetKeyName(targetCode))
	}

	t.active[code] = activeTranslation{code: targetCode, sink: sink, output: output}
	common.LogKey(target, targetCode)
	return true
}

// TryRelease emits the release for a code previously translated by TryPress,
// to the same sink press used, restores any modifier consumed during
// translation that is still physically held, and clears the recorded
// translation. Returns false if code has no active translation.
func (t *Translator) TryRelease(code uint16, m *matcher.Matcher, virtual *evdev.InputDevice, emittedTracker *timers.EmittedModifierTracker) bool {
	translation, ok := t.active[code]
	if !ok {
		return false
	}
	delete(t.active, code)

	var err error
	switch translation.sink {
	case sinkVirtual:
		err = emitKeyToVirtual(virtual, translation.code, false)
	default:
		err = translation.output.WriteFrame(evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.EvCode(translation.code), Value: 0})
	}
	if err != nil {
		common.LogDebug("remap translate: release code=%d failed: %v", translation.code, err)
	}

	if matcher.IsModifierKey(translation.code) {
		m.UpdateModifierState(translation.code, false)
		emittedTracker.MarkUp(keys.GetKeyName(translation.code))
	}

	restoreConsumedModifiers(virtual, m.GetCurrentModifiers(), emittedTracker)
	return true
}

// resolveSink decides where a translated target key should be written:
// pointer buttons always go to outputs.Pointer; keyboard codes go to the
// virtual clone when it declares the capability, falling back to
// outputs.Keyboard (which registers every code) otherwise.
func (t *Translator) resolveSink(targetCode uint16, virtual *evdev.InputDevice, outputs executor.Outputs) (translationSink, *executor.EventSink) {
	if output := executor.OutputForCode(outputs, targetCode); output == outputs.Pointer {
		return sinkPointer, output
	}
	if virtual != nil && t.virtualKeyCapable[targetCode] {
		return sinkVirtual, nil
	}
	return sinkKeyboard, outputs.Keyboard
}

// emitKeyToVirtual writes a single key event followed by SYN_REPORT to the
// device's own virtual clone, mirroring ladder.EmitModifierKey's framing.
func emitKeyToVirtual(virtual *evdev.InputDevice, code uint16, down bool) error {
	value := int32(0)
	if down {
		value = 1
	}
	if err := virtual.WriteOne(&evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.EvCode(code), Value: value}); err != nil {
		return err
	}
	return virtual.WriteOne(&evdev.InputEvent{Type: evdev.EV_SYN, Code: evdev.SYN_REPORT, Value: 0})
}

// consumeTranslationModifiers releases, on the virtual keyboard, any
// modifier prefix of combo that the system currently sees as held. Mirrors
// ladder.consumeComboModifiers so a translated key never leaks its trigger
// modifiers into the output (e.g. "ctrl+r" = ">somekey" must not send
// ctrl+somekey).
func consumeTranslationModifiers(virtual *evdev.InputDevice, combo string, emittedTracker *timers.EmittedModifierTracker) {
	parts := strings.Split(combo, "+")
	if len(parts) < 2 {
		return
	}
	for _, name := range parts[:len(parts)-1] {
		if !emittedTracker.IsDown(name) {
			continue
		}
		if virtual != nil {
			ladder.EmitModifierKey(virtual, keys.ResolveKeyCode, name, false)
		}
		emittedTracker.MarkUp(name)
	}
}
