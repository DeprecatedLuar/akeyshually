package executor

import (
	"fmt"
	"sync"

	evdev "github.com/holoplot/go-evdev"
)

// EventWriter is the uinput write port used by remap outputs.
type EventWriter interface {
	WriteOne(event *evdev.InputEvent) error
}

// EventSink serializes complete event frames to one output device.
type EventSink struct {
	mu     sync.Mutex
	writer EventWriter
}

// Outputs groups the focused keyboard and pointer output ports.
type Outputs struct {
	Keyboard *EventSink
	Pointer  *EventSink
}

func NewEventSink(writer EventWriter) *EventSink {
	if writer == nil {
		return nil
	}
	return &EventSink{writer: writer}
}

func (s *EventSink) WriteFrame(events ...evdev.InputEvent) error {
	if s == nil || s.writer == nil {
		return fmt.Errorf("output device is unavailable")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range events {
		if err := s.writer.WriteOne(&events[i]); err != nil {
			return fmt.Errorf("write event type=%d code=%d value=%d: %w",
				events[i].Type, events[i].Code, events[i].Value, err)
		}
	}

	syncEvent := evdev.InputEvent{Type: evdev.EV_SYN, Code: evdev.SYN_REPORT}
	if err := s.writer.WriteOne(&syncEvent); err != nil {
		return fmt.Errorf("write SYN_REPORT: %w", err)
	}
	return nil
}
