package executor

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/deprecatedluar/akeyshually/internal/config"
	evdev "github.com/holoplot/go-evdev"
)

const testEventTimeout = 250 * time.Millisecond

type recordingWriter struct {
	mu      sync.Mutex
	events  []evdev.InputEvent
	failAt  int
	written chan struct{}
}

func newRecordingWriter() *recordingWriter {
	return &recordingWriter{failAt: -1, written: make(chan struct{}, 32)}
}

func (w *recordingWriter) WriteOne(event *evdev.InputEvent) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.events) == w.failAt {
		return errors.New("injected write failure")
	}
	w.events = append(w.events, *event)
	select {
	case w.written <- struct{}{}:
	default:
	}
	return nil
}

func (w *recordingWriter) snapshot() []evdev.InputEvent {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]evdev.InputEvent(nil), w.events...)
}

func testOutputs() (Outputs, *recordingWriter, *recordingWriter) {
	keyboard := newRecordingWriter()
	pointer := newRecordingWriter()
	return Outputs{
		Keyboard: NewEventSink(keyboard),
		Pointer:  NewEventSink(pointer),
	}, keyboard, pointer
}

func TestRunRemapRoutesExistingOutputs(t *testing.T) {
	tests := []struct {
		name             string
		command          string
		wantKeyboardCode evdev.EvCode
		wantPointerCode  evdev.EvCode
		wantPointerValue int32
	}{
		{name: "keyboard", command: ">a", wantKeyboardCode: evdev.KEY_A},
		{name: "left click", command: ">lclick", wantPointerCode: evdev.BTN_LEFT, wantPointerValue: 1},
		{name: "scroll up", command: ">scrollup", wantPointerCode: evdev.REL_WHEEL, wantPointerValue: 1},
		{name: "scroll down", command: ">scrolldown", wantPointerCode: evdev.REL_WHEEL, wantPointerValue: -1},
		{name: "scroll left", command: ">scrollleft", wantPointerCode: evdev.REL_HWHEEL, wantPointerValue: -1},
		{name: "scroll right", command: ">scrollright", wantPointerCode: evdev.REL_HWHEEL, wantPointerValue: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs, keyboard, pointer := testOutputs()
			ctx := ExecContext{Outputs: outputs, LoopState: NewLoopState()}
			if err := run(tt.command, ctx); err != nil {
				t.Fatalf("run(%q): %v", tt.command, err)
			}

			keyboardEvents := keyboard.snapshot()
			pointerEvents := pointer.snapshot()
			if tt.wantKeyboardCode != 0 {
				assertFirstEvent(t, keyboardEvents, evdev.EV_KEY, tt.wantKeyboardCode, 1)
				if len(pointerEvents) != 0 {
					t.Fatalf("pointer received unexpected events: %+v", pointerEvents)
				}
				return
			}
			assertFirstEvent(t, pointerEvents, eventTypeForCode(tt.wantPointerCode), tt.wantPointerCode, tt.wantPointerValue)
			if len(keyboardEvents) != 0 {
				t.Fatalf("keyboard received unexpected events: %+v", keyboardEvents)
			}
		})
	}
}

func TestRunRemapClickUsesSeparateSyncFrames(t *testing.T) {
	outputs, _, pointer := testOutputs()
	if err := run(">lclick", ExecContext{Outputs: outputs}); err != nil {
		t.Fatalf("run(%q): %v", ">lclick", err)
	}

	events := pointer.snapshot()
	synCount := 0
	for _, e := range events {
		if e.Type == evdev.EV_SYN {
			synCount++
		}
	}
	if synCount != 2 {
		t.Fatalf("events = %+v, want 2 SYN_REPORTs (down and up in separate frames), got %d", events, synCount)
	}
	if len(events) != 4 {
		t.Fatalf("events = %+v, want [down, syn, up, syn]", events)
	}
	if events[0].Value != 1 || events[1].Type != evdev.EV_SYN || events[2].Value != 0 || events[3].Type != evdev.EV_SYN {
		t.Fatalf("events = %+v, want down/syn/up/syn ordering", events)
	}
}

func TestRunRemapRoutesModifiedClickAcrossOutputs(t *testing.T) {
	outputs, keyboard, pointer := testOutputs()
	if err := run(">ctrl+lclick", ExecContext{Outputs: outputs}); err != nil {
		t.Fatalf("run modified click: %v", err)
	}

	keyboardEvents := keyboard.snapshot()
	pointerEvents := pointer.snapshot()
	assertFirstEvent(t, keyboardEvents, evdev.EV_KEY, evdev.KEY_LEFTCTRL, 1)
	assertFirstEvent(t, pointerEvents, evdev.EV_KEY, evdev.BTN_LEFT, 1)
	if got := keyboardEvents[len(keyboardEvents)-2]; got.Code != evdev.KEY_LEFTCTRL || got.Value != 0 {
		t.Fatalf("modifier release = %+v, want ctrl release", got)
	}
}

func TestRunRemapPropagatesWriteFailure(t *testing.T) {
	outputs, _, pointer := testOutputs()
	pointer.failAt = 0
	err := run(">scrollup", ExecContext{Outputs: outputs})
	if err == nil || !strings.Contains(err.Error(), "injected write failure") {
		t.Fatalf("error = %v, want injected write failure", err)
	}
}

func TestHeldPointerButtonUsesPointerOutput(t *testing.T) {
	outputs, keyboard, pointer := testOutputs()
	loopState := NewLoopState()
	ctx := ExecContext{Outputs: outputs, LoopState: loopState}

	if err := run(">>lclick", ctx); err != nil {
		t.Fatalf("hold button: %v", err)
	}
	if err := run("<lclick", ctx); err != nil {
		t.Fatalf("release button: %v", err)
	}

	if len(keyboard.snapshot()) != 0 {
		t.Fatal("keyboard output received pointer-button events")
	}
	events := pointer.snapshot()
	if len(events) < 4 {
		t.Fatalf("pointer events = %+v, want press and release frames", events)
	}
	if events[0].Code != evdev.BTN_LEFT || events[0].Value != 1 {
		t.Fatalf("press event = %+v", events[0])
	}
	if events[2].Code != evdev.BTN_LEFT || events[2].Value != 0 {
		t.Fatalf("release event = %+v", events[2])
	}
	if len(loopState.PersistentHeld) != 0 {
		t.Fatalf("persistent held state not cleared: %+v", loopState.PersistentHeld)
	}
}

func TestRepeatRemapUsesPointerOutput(t *testing.T) {
	outputs, keyboard, pointer := testOutputs()
	loopState := NewLoopState()
	cfg := &config.Config{Settings: config.Settings{DefaultInterval: 10}}
	shortcut := &config.ParsedShortcut{Interval: 10, Commands: []string{">scrollup"}}
	ctx := ExecContext{Outputs: outputs, LoopState: loopState, Config: cfg}

	loopState.StartLoop("1", shortcut, ctx)
	defer loopState.StopLoop("1")

	select {
	case <-pointer.written:
	case <-time.After(testEventTimeout):
		t.Fatal("repeat loop did not write to pointer output")
	}
	if len(keyboard.snapshot()) != 0 {
		t.Fatal("repeat remap wrote to keyboard output")
	}
}

func assertFirstEvent(t *testing.T, events []evdev.InputEvent, typ evdev.EvType, code evdev.EvCode, value int32) {
	t.Helper()
	if len(events) == 0 {
		t.Fatal("no events written")
	}
	got := events[0]
	if got.Type != typ || got.Code != code || got.Value != value {
		t.Fatalf("first event = %+v, want type=%d code=%d value=%d", got, typ, code, value)
	}
	if events[len(events)-1].Type != evdev.EV_SYN {
		t.Fatalf("last event = %+v, want SYN_REPORT", events[len(events)-1])
	}
}

func eventTypeForCode(code evdev.EvCode) evdev.EvType {
	if code == evdev.BTN_LEFT {
		return evdev.EV_KEY
	}
	return evdev.EV_REL
}
