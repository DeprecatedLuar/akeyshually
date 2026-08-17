package listener

import (
	"errors"
	"testing"

	evdev "github.com/holoplot/go-evdev"
)

func TestDispatchEventPreservesEventType(t *testing.T) {
	events := []evdev.InputEvent{
		{Type: evdev.EV_KEY, Code: evdev.EvCode(evdev.KEY_ESC), Value: 1},
		{Type: evdev.EV_ABS, Code: evdev.EvCode(evdev.ABS_Y), Value: 42},
	}

	for _, event := range events {
		var received evdev.InputEvent
		forwarded := false
		err := dispatchEvent(&event, func(got evdev.InputEvent) bool {
			received = got
			return true
		}, func(*evdev.InputEvent) error {
			forwarded = true
			return nil
		})
		if err != nil {
			t.Fatalf("dispatchEvent() error = %v", err)
		}
		if received.Type != event.Type || received.Code != event.Code || received.Value != event.Value {
			t.Errorf("handler received %+v, want %+v", received, event)
		}
		if forwarded {
			t.Errorf("handled event %+v was forwarded", event)
		}
	}
}

func TestDispatchEventForwardsUnhandledEvent(t *testing.T) {
	event := evdev.InputEvent{Type: evdev.EV_ABS, Code: evdev.EvCode(evdev.ABS_X), Value: 10}
	var forwarded evdev.InputEvent

	err := dispatchEvent(&event, func(evdev.InputEvent) bool {
		return false
	}, func(got *evdev.InputEvent) error {
		forwarded = *got
		return nil
	})
	if err != nil {
		t.Fatalf("dispatchEvent() error = %v", err)
	}
	if forwarded.Type != event.Type || forwarded.Code != event.Code || forwarded.Value != event.Value {
		t.Errorf("forwarded %+v, want %+v", forwarded, event)
	}
}

func TestDispatchEventAlwaysForwardsSyn(t *testing.T) {
	event := evdev.InputEvent{Type: evdev.EV_SYN, Code: evdev.EvCode(evdev.SYN_REPORT)}
	handled := false
	forwarded := false

	err := dispatchEvent(&event, func(evdev.InputEvent) bool {
		handled = true
		return true
	}, func(*evdev.InputEvent) error {
		forwarded = true
		return nil
	})
	if err != nil {
		t.Fatalf("dispatchEvent() error = %v", err)
	}
	if !handled {
		t.Error("SYN event did not reach handler")
	}
	if !forwarded {
		t.Error("SYN event was not forwarded")
	}
}

func TestDispatchEventPropagatesForwardError(t *testing.T) {
	wantErr := errors.New("write failed")
	event := evdev.InputEvent{Type: evdev.EV_MSC, Code: 1, Value: 2}

	err := dispatchEvent(&event, func(evdev.InputEvent) bool {
		t.Fatal("non-key/ABS/SYN event unexpectedly reached handler")
		return false
	}, func(*evdev.InputEvent) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("dispatchEvent() error = %v, want wrapped %v", err, wantErr)
	}
}
