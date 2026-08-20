package executor

import (
	"testing"

	"github.com/deprecatedluar/akeyshually/internal/config"
	evdev "github.com/holoplot/go-evdev"
)

func TestStartHeldProcessSustainsSingleArrowRemap(t *testing.T) {
	outputs, keyboard, _ := testOutputs()
	loopState := NewLoopState()
	cfg := &config.Config{}
	shortcut := &config.ParsedShortcut{Commands: []string{">ctrl"}}
	execCtx := ExecContext{Outputs: outputs, LoopState: loopState, Config: cfg}

	if err := loopState.StartHeldProcess("btn_1", shortcut, execCtx); err != nil {
		t.Fatalf("StartHeldProcess: %v", err)
	}

	events := keyboard.snapshot()
	if len(events) != 2 {
		t.Fatalf("events after start = %+v, want down + SYN only (no release)", events)
	}
	if events[0].Code != evdev.KEY_LEFTCTRL || events[0].Value != 1 {
		t.Fatalf("first event = %+v, want ctrl down", events[0])
	}

	if err := loopState.StopHeldProcess("btn_1"); err != nil {
		t.Fatalf("StopHeldProcess: %v", err)
	}

	events = keyboard.snapshot()
	if len(events) != 4 {
		t.Fatalf("events after stop = %+v, want down+syn+up+syn", events)
	}
	if events[2].Code != evdev.KEY_LEFTCTRL || events[2].Value != 0 {
		t.Fatalf("release event = %+v, want ctrl up", events[2])
	}
	if len(loopState.HeldKeys) != 0 {
		t.Fatalf("held key state not cleared: %+v", loopState.HeldKeys)
	}
}
