package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/deprecatedluar/akeyshually/internal/common"
	"github.com/deprecatedluar/akeyshually/internal/config"
)

type heldOutput struct {
	Output *EventSink
	Codes  []uint16
}

type activeLoop struct {
	cancel context.CancelFunc
	id     uint64
}

// LoopState tracks active repeat loops and sustained processes across key events.
type LoopState struct {
	Mu             sync.Mutex
	Active         map[string]activeLoop // repeat loops
	HeldProcesses  map[string]*exec.Cmd  // sustained whileheld processes
	HeldKeys       map[string]heldOutput // sustained remap hold keys
	PersistentHeld map[string]heldOutput // >> persistent remap keys
	nextLoopID     uint64
}

func NewLoopState() *LoopState {
	return &LoopState{
		Active:         make(map[string]activeLoop),
		HeldProcesses:  make(map[string]*exec.Cmd),
		HeldKeys:       make(map[string]heldOutput),
		PersistentHeld: make(map[string]heldOutput),
	}
}

// StartLoop starts a repeat loop for the given combo
func (s *LoopState) StartLoop(combo string, shortcut *config.ParsedShortcut, execCtx ExecContext) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if active, exists := s.Active[combo]; exists {
		active.cancel()
	}

	interval := intervalOrDefault(shortcut.Interval, execCtx.Config.Settings.DefaultInterval)
	ctx, cancel := context.WithCancel(context.Background())
	s.nextLoopID++
	loopID := s.nextLoopID
	s.Active[combo] = activeLoop{cancel: cancel, id: loopID}

	resolvedCmd := execCtx.Config.ResolveCommand(shortcut.Commands[0])
	common.LogTrigger(resolvedCmd)
	go func() {
		err := runTickerLoop(ctx, interval, func() error { return run(resolvedCmd, execCtx) })
		if err != nil {
			fmt.Fprintf(os.Stderr, "Repeat loop stopped: %v\n", err)
		}

		s.Mu.Lock()
		if active, exists := s.Active[combo]; exists && active.id == loopID {
			delete(s.Active, combo)
		}
		s.Mu.Unlock()
	}()
}

// StopLoop stops a repeat loop for the given combo
func (s *LoopState) StopLoop(combo string) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if active, exists := s.Active[combo]; exists {
		active.cancel()
		delete(s.Active, combo)
	}
}

// StartHeldProcess starts a sustained process or remap for the given combo
func (s *LoopState) StartHeldProcess(combo string, shortcut *config.ParsedShortcut, execCtx ExecContext) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	resolvedCmd := execCtx.Config.ResolveCommand(shortcut.Commands[0])

	// Check if this is a remap command (">>target" or ">target" under a span trigger)
	if target, ok := remapHoldTarget(resolvedCmd); ok {
		output, err := outputForTarget(execCtx.Outputs, target)
		if err != nil {
			return err
		}
		if held, exists := s.HeldKeys[combo]; exists {
			if err := EmitKeysUp(held.Output, held.Codes); err != nil {
				return err
			}
		}
		codes, err := EmitKeysDown(output, target, execCtx.Modifiers)
		if err != nil {
			return err
		}
		if len(codes) > 0 {
			s.HeldKeys[combo] = heldOutput{Output: output, Codes: codes}
		}
		return nil
	}

	// Shell command
	if cmd, exists := s.HeldProcesses[combo]; exists {
		StopProcess(cmd)
	}

	common.LogTrigger(resolvedCmd)
	cmd := ExecuteTracked(resolvedCmd, execCtx.Config)
	if cmd != nil {
		s.HeldProcesses[combo] = cmd
	}
	return nil
}

// StopHeldProcess stops a sustained process or remap for the given combo
func (s *LoopState) StopHeldProcess(combo string) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if held, exists := s.HeldKeys[combo]; exists {
		if err := EmitKeysUp(held.Output, held.Codes); err != nil {
			return err
		}
		delete(s.HeldKeys, combo)
		return nil
	}

	// Fallback: match by base key in case modifier state drifted
	baseKey := baseKeyFromCombo(combo)
	for storedCombo, held := range s.HeldKeys {
		if baseKeyFromCombo(storedCombo) == baseKey {
			if err := EmitKeysUp(held.Output, held.Codes); err != nil {
				return err
			}
			delete(s.HeldKeys, storedCombo)
			return nil
		}
	}

	if cmd, exists := s.HeldProcesses[combo]; exists {
		StopProcess(cmd)
		delete(s.HeldProcesses, combo)
		return nil
	}

	for storedCombo, cmd := range s.HeldProcesses {
		if baseKeyFromCombo(storedCombo) == baseKey {
			StopProcess(cmd)
			delete(s.HeldProcesses, storedCombo)
			return nil
		}
	}
	return nil
}

// ToggleLoop toggles a repeat loop on/off for the given combo
func (s *LoopState) ToggleLoop(combo string, shortcut *config.ParsedShortcut, execCtx ExecContext) {
	s.Mu.Lock()
	_, running := s.Active[combo]
	s.Mu.Unlock()

	if running {
		s.StopLoop(combo)
	} else {
		s.StartLoop(combo, shortcut, execCtx)
	}
}

// --- Helper functions ---

func runTickerLoop(ctx context.Context, interval float64, fn func() error) error {
	ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := fn(); err != nil {
				return err
			}
		}
	}
}

func baseKeyFromCombo(combo string) string {
	parts := strings.Split(combo, "+")
	return parts[len(parts)-1]
}

func intervalOrDefault(interval, def float64) float64 {
	if interval > 0 {
		return interval
	}
	return def
}
