package handlers

import (
	"testing"

	"github.com/deprecatedluar/akeyshually/internal/executor"
	evdev "github.com/holoplot/go-evdev"
)

func testAnalogInfo() AbsInfoMap {
	return AbsInfoMap{
		uint16(evdev.ABS_X): {
			Minimum: -32768,
			Maximum: 32767,
		},
	}
}

func TestHandleAbsAccumulatesWithoutContactEvent(t *testing.T) {
	infos := testAnalogInfo()
	accumulators := make(AccumulatorMap)
	prevValues := make(PrevValuesMap)

	HandleAbs(uint16(evdev.ABS_X), 0, infos, accumulators, prevValues, nil, executor.ExecContext{})
	HandleAbs(uint16(evdev.ABS_X), 5000, infos, accumulators, prevValues, nil, executor.ExecContext{})

	if got := accumulators["abs_x+"]; got != 5000 {
		t.Fatalf("abs_x+ accumulation = %v, want 5000", got)
	}
}

func TestHandleAbsFirstSampleOnlySeedsPreviousValue(t *testing.T) {
	infos := testAnalogInfo()
	accumulators := make(AccumulatorMap)
	prevValues := make(PrevValuesMap)

	HandleAbs(uint16(evdev.ABS_X), 1200, infos, accumulators, prevValues, nil, executor.ExecContext{})

	if len(accumulators) != 0 {
		t.Fatalf("first sample accumulated movement: %v", accumulators)
	}
	if got := prevValues[uint16(evdev.ABS_X)]; got != 1200 {
		t.Fatalf("previous value = %d, want 1200", got)
	}
}

func TestHandleAbsPreservesFlatZoneBehavior(t *testing.T) {
	infos := testAnalogInfo()
	info := infos[uint16(evdev.ABS_X)]
	info.Flat = 500
	infos[uint16(evdev.ABS_X)] = info
	accumulators := make(AccumulatorMap)
	prevValues := PrevValuesMap{uint16(evdev.ABS_X): 0}

	HandleAbs(uint16(evdev.ABS_X), 250, infos, accumulators, prevValues, nil, executor.ExecContext{})

	if len(accumulators) != 0 {
		t.Fatalf("movement inside flat zone accumulated: %v", accumulators)
	}
	if got := prevValues[uint16(evdev.ABS_X)]; got != 250 {
		t.Fatalf("previous value = %d, want 250", got)
	}
}

func TestResetAbsStateOnContactEnd(t *testing.T) {
	tests := []struct {
		name  string
		event evdev.InputEvent
	}{
		{
			name:  "standard BTN_TOUCH release",
			event: evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.EvCode(evdev.BTN_TOUCH), Value: 0},
		},
		{
			name:  "legacy Huion ABS_MISC lift",
			event: evdev.InputEvent{Type: evdev.EV_ABS, Code: evdev.EvCode(evdev.ABS_MISC), Value: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accumulators := AccumulatorMap{"x+": 900}
			prevValues := PrevValuesMap{uint16(evdev.ABS_X): 900}

			ResetAbsStateOnContactEnd(tt.event, accumulators, prevValues)

			if len(accumulators) != 0 {
				t.Errorf("accumulators not reset: %v", accumulators)
			}
			if len(prevValues) != 0 {
				t.Errorf("previous values not reset: %v", prevValues)
			}
		})
	}
}

func TestAbsMiscNonzeroDoesNotResetOrGateAxis(t *testing.T) {
	infos := testAnalogInfo()
	accumulators := AccumulatorMap{"abs_x+": 100}
	prevValues := PrevValuesMap{uint16(evdev.ABS_X): 100}
	touchEvent := evdev.InputEvent{Type: evdev.EV_ABS, Code: evdev.EvCode(evdev.ABS_MISC), Value: 15}

	ResetAbsStateOnContactEnd(touchEvent, accumulators, prevValues)
	HandleAbs(uint16(evdev.ABS_MISC), touchEvent.Value, infos, accumulators, prevValues, nil, executor.ExecContext{})
	HandleAbs(uint16(evdev.ABS_X), 300, infos, accumulators, prevValues, nil, executor.ExecContext{})

	if got := accumulators["abs_x+"]; got != 300 {
		t.Fatalf("abs_x+ accumulation = %v, want 300", got)
	}
}

func TestFirstAxisSampleAfterContactEndDoesNotJump(t *testing.T) {
	infos := testAnalogInfo()
	accumulators := AccumulatorMap{"x+": 1000}
	prevValues := PrevValuesMap{uint16(evdev.ABS_X): 1000}
	liftEvent := evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.EvCode(evdev.BTN_TOUCH), Value: 0}

	ResetAbsStateOnContactEnd(liftEvent, accumulators, prevValues)
	HandleAbs(uint16(evdev.ABS_X), -12000, infos, accumulators, prevValues, nil, executor.ExecContext{})

	if len(accumulators) != 0 {
		t.Fatalf("first post-contact sample accumulated movement: %v", accumulators)
	}
	if got := prevValues[uint16(evdev.ABS_X)]; got != -12000 {
		t.Fatalf("previous value = %d, want -12000", got)
	}
}
