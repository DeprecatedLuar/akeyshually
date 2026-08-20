package handlers

import (
	"sync"
	"testing"

	"github.com/deprecatedluar/akeyshually/internal/config"
	"github.com/deprecatedluar/akeyshually/internal/executor"
	"github.com/deprecatedluar/akeyshually/internal/matcher"
	"github.com/deprecatedluar/akeyshually/internal/timers"
	evdev "github.com/holoplot/go-evdev"
)

// recordingWriter is a minimal executor.EventWriter fake that captures every
// event handed to it, so tests can assert on exactly what was emitted.
type recordingWriter struct {
	mu     sync.Mutex
	events []evdev.InputEvent
}

func (w *recordingWriter) WriteOne(event *evdev.InputEvent) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.events = append(w.events, *event)
	return nil
}

func (w *recordingWriter) snapshot() []evdev.InputEvent {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]evdev.InputEvent(nil), w.events...)
}

func testOutputs() (executor.Outputs, *recordingWriter, *recordingWriter) {
	keyboard := &recordingWriter{}
	pointer := &recordingWriter{}
	return executor.Outputs{
		Keyboard: executor.NewEventSink(keyboard),
		Pointer:  executor.NewEventSink(pointer),
	}, keyboard, pointer
}

// keyEvents filters a snapshot down to EV_KEY events, since WriteFrame always
// appends a trailing SYN_REPORT that would otherwise clutter assertions.
func keyEvents(events []evdev.InputEvent) []evdev.InputEvent {
	var out []evdev.InputEvent
	for _, e := range events {
		if e.Type == evdev.EV_KEY {
			out = append(out, e)
		}
	}
	return out
}

func TestTranslatorPressReleaseRoundTrip(t *testing.T) {
	cfg := &config.Config{RemapTable: map[string]string{"f2": "lclick"}}
	outputs, keyboardWriter, pointerWriter := testOutputs()
	emittedTracker := timers.NewEmittedModifierTracker()
	m := matcher.New(map[string][]*config.ParsedShortcut{})
	translator := NewTranslator(nil) // no live virtual device in unit tests

	codeF2 := uint16(evdev.KEY_F2)

	if !translator.TryPress(codeF2, "f2", cfg, m, nil, outputs, emittedTracker) {
		t.Fatal("expected f2 press to translate via RemapTable hit")
	}
	pressed := keyEvents(pointerWriter.snapshot())
	if len(pressed) != 1 || pressed[0].Code != evdev.BTN_LEFT || pressed[0].Value != 1 {
		t.Fatalf("expected one BTN_LEFT down on pointer output, got %+v", pressed)
	}
	if len(keyEvents(keyboardWriter.snapshot())) != 0 {
		t.Fatal("pointer target must never reach the keyboard output")
	}

	if !translator.TryRelease(codeF2, m, nil, emittedTracker) {
		t.Fatal("expected f2 release to find the recorded translation")
	}
	released := keyEvents(pointerWriter.snapshot())
	if len(released) != 2 || released[1].Code != evdev.BTN_LEFT || released[1].Value != 0 {
		t.Fatalf("expected a BTN_LEFT up to follow the down, got %+v", released)
	}
	if len(keyEvents(keyboardWriter.snapshot())) != 0 {
		t.Fatal("pointer target must never reach the keyboard output")
	}

	// The translation was consumed by release; releasing again must be a no-op.
	if translator.TryRelease(codeF2, m, nil, emittedTracker) {
		t.Fatal("second release of the same code should find no active translation")
	}
}

func TestTranslatorKeyboardTargetNeverReachesPointerOutput(t *testing.T) {
	cfg := &config.Config{RemapTable: map[string]string{"f3": "a"}}
	outputs, keyboardWriter, pointerWriter := testOutputs()
	emittedTracker := timers.NewEmittedModifierTracker()
	m := matcher.New(map[string][]*config.ParsedShortcut{})
	translator := NewTranslator(nil)

	codeF3 := uint16(evdev.KEY_F3)

	if !translator.TryPress(codeF3, "f3", cfg, m, nil, outputs, emittedTracker) {
		t.Fatal("expected f3 press to translate via RemapTable hit")
	}
	translator.TryRelease(codeF3, m, nil, emittedTracker)

	got := keyEvents(keyboardWriter.snapshot())
	if len(got) != 2 || got[0].Code != evdev.KEY_A || got[0].Value != 1 || got[1].Code != evdev.KEY_A || got[1].Value != 0 {
		t.Fatalf("expected KEY_A down then up on keyboard output, got %+v", got)
	}
	if len(keyEvents(pointerWriter.snapshot())) != 0 {
		t.Fatal("keyboard target must never reach the pointer output")
	}
}

// A combo re-evaluated after its modifier prefix releases first would no
// longer match RemapTable ("ctrl+r" needs ctrl held), which is exactly why
// release must key off the translation recorded at press time rather than
// recomputing the combo - otherwise the translated target would be left
// stuck down forever. See implementation-plan.md's "Decisions" section.
func TestTranslatorReleaseUsesRecordedTranslationNotRecomputedCombo(t *testing.T) {
	cfg := &config.Config{RemapTable: map[string]string{"ctrl+r": "a"}}
	outputs, keyboardWriter, _ := testOutputs()
	emittedTracker := timers.NewEmittedModifierTracker()
	m := matcher.New(map[string][]*config.ParsedShortcut{})

	ctrlCode := uint16(evdev.KEY_LEFTCTRL)
	rCode := uint16(evdev.KEY_R)

	// ctrl held physically and forwarded transparently, as HandlePress would
	// leave it for a modifier with no lone shortcut of its own.
	m.UpdateModifierState(ctrlCode, true)
	emittedTracker.MarkDown("ctrl")

	translator := NewTranslator(nil)
	if !translator.TryPress(rCode, "ctrl+r", cfg, m, nil, outputs, emittedTracker) {
		t.Fatal("expected ctrl+r press to translate via RemapTable hit")
	}
	if emittedTracker.IsDown("ctrl") {
		t.Fatal("ctrl should have been consumed so it does not leak into the translated key")
	}

	// User releases ctrl before r. Recomputing "the combo" at this point
	// would yield bare "r", which is not in RemapTable.
	m.UpdateModifierState(ctrlCode, false)
	emittedTracker.MarkUp("ctrl")

	if !translator.TryRelease(rCode, m, nil, emittedTracker) {
		t.Fatal("release must still find the translation recorded at press time")
	}
	got := keyEvents(keyboardWriter.snapshot())
	if len(got) != 2 || got[1].Code != evdev.KEY_A || got[1].Value != 0 {
		t.Fatalf("expected the translated target to be released, got %+v", got)
	}
}

// "capslock" = ">super" must make a subsequent physical "super+m" combo
// recognized by this same keyboard's matcher, not just forward a raw
// KEY_LEFTMETA to the system - otherwise "super+m" = "cmd" would never be
// detected, since combo matching reads Matcher's own modifier state.
func TestTranslatorPressUpdatesMatcherModifierState(t *testing.T) {
	cfg := &config.Config{RemapTable: map[string]string{"capslock": "super"}}
	outputs, _, _ := testOutputs()
	emittedTracker := timers.NewEmittedModifierTracker()
	m := matcher.New(map[string][]*config.ParsedShortcut{})
	translator := NewTranslator(nil)

	capslockCode := uint16(evdev.KEY_CAPSLOCK)
	rCode := uint16(evdev.KEY_R)

	if !translator.TryPress(capslockCode, "capslock", cfg, m, nil, outputs, emittedTracker) {
		t.Fatal("expected capslock press to translate via RemapTable hit")
	}
	if !m.GetCurrentModifiers().Super {
		t.Fatal("translating capslock to super should set Matcher's Super state")
	}
	if !emittedTracker.IsDown("super") {
		t.Fatal("translating capslock to super should mark super as emitted-down")
	}

	// With Super now tracked, an unrelated key must see it in its combo.
	if got := m.GetCurrentCombo(rCode); got != "super+r" {
		t.Fatalf("GetCurrentCombo(r) = %q, want \"super+r\"", got)
	}

	if !translator.TryRelease(capslockCode, m, nil, emittedTracker) {
		t.Fatal("expected capslock release to find the recorded translation")
	}
	if m.GetCurrentModifiers().Super {
		t.Fatal("releasing the translated capslock should clear Matcher's Super state")
	}
	if emittedTracker.IsDown("super") {
		t.Fatal("releasing the translated capslock should mark super as emitted-up")
	}
}
