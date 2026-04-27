package executor

import (
	"context"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/deprecatedluar/akeyshually/internal/common"
	"github.com/deprecatedluar/akeyshually/internal/config"
	"github.com/deprecatedluar/akeyshually/internal/matcher"
	evdev "github.com/holoplot/go-evdev"
)

// LoopState tracks active repeat loops and sustained processes across key events.
type LoopState struct {
	Mu             sync.Mutex
	Active         map[string]context.CancelFunc // repeat loops
	HeldProcesses  map[string]*exec.Cmd          // sustained whileheld processes
	HeldKeys       map[string][]uint16           // sustained remap hold keys
	PersistentHeld map[string][]uint16           // >> persistent remap keys
}

func NewLoopState() *LoopState {
	return &LoopState{
		Active:         make(map[string]context.CancelFunc),
		HeldProcesses:  make(map[string]*exec.Cmd),
		HeldKeys:       make(map[string][]uint16),
		PersistentHeld: make(map[string][]uint16),
	}
}

// StartLoop starts a repeat loop for the given combo
func (s *LoopState) StartLoop(combo string, shortcut *config.ParsedShortcut, cfg *config.Config) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if cancel, exists := s.Active[combo]; exists {
		cancel()
	}

	interval := intervalOrDefault(shortcut.Interval, cfg.Settings.DefaultInterval)
	ctx, cancel := context.WithCancel(context.Background())
	s.Active[combo] = cancel

	resolvedCmd := cfg.ResolveCommand(shortcut.Commands[0])
	common.LogTrigger(resolvedCmd)
	go runTickerLoop(ctx, interval, func() { Execute(resolvedCmd, cfg) })
}

// StopLoop stops a repeat loop for the given combo
func (s *LoopState) StopLoop(combo string) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if cancel, exists := s.Active[combo]; exists {
		cancel()
		delete(s.Active, combo)
	}
}

// StartHeldProcess starts a sustained process or remap for the given combo
func (s *LoopState) StartHeldProcess(combo string, shortcut *config.ParsedShortcut, cfg *config.Config, injector *evdev.InputDevice, heldModifiers matcher.ModifierState) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	resolvedCmd := cfg.ResolveCommand(shortcut.Commands[0])

	// Check if this is a remap command (starts with >>)
	if strings.HasPrefix(resolvedCmd, ">>") {
		target := resolvedCmd[2:]
		if codes, exists := s.HeldKeys[combo]; exists {
			EmitKeysUp(injector, codes)
		}
		codes := EmitKeysDown(injector, target, heldModifiers)
		if len(codes) > 0 {
			s.HeldKeys[combo] = codes
		}
		return
	}

	// Shell command
	if cmd, exists := s.HeldProcesses[combo]; exists {
		StopProcess(cmd)
	}

	common.LogTrigger(resolvedCmd)
	cmd := ExecuteTracked(resolvedCmd, cfg)
	if cmd != nil {
		s.HeldProcesses[combo] = cmd
	}
}

// StopHeldProcess stops a sustained process or remap for the given combo
func (s *LoopState) StopHeldProcess(combo string, injector *evdev.InputDevice) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if codes, exists := s.HeldKeys[combo]; exists {
		EmitKeysUp(injector, codes)
		delete(s.HeldKeys, combo)
		return
	}

	// Fallback: match by base key in case modifier state drifted
	baseKey := baseKeyFromCombo(combo)
	for storedCombo, codes := range s.HeldKeys {
		if baseKeyFromCombo(storedCombo) == baseKey {
			EmitKeysUp(injector, codes)
			delete(s.HeldKeys, storedCombo)
			return
		}
	}

	if cmd, exists := s.HeldProcesses[combo]; exists {
		StopProcess(cmd)
		delete(s.HeldProcesses, combo)
		return
	}

	for storedCombo, cmd := range s.HeldProcesses {
		if baseKeyFromCombo(storedCombo) == baseKey {
			StopProcess(cmd)
			delete(s.HeldProcesses, storedCombo)
			return
		}
	}
}

// ToggleLoop toggles a repeat loop on/off for the given combo
func (s *LoopState) ToggleLoop(combo string, shortcut *config.ParsedShortcut, cfg *config.Config) {
	s.Mu.Lock()
	_, running := s.Active[combo]
	s.Mu.Unlock()

	if running {
		s.StopLoop(combo)
	} else {
		s.StartLoop(combo, shortcut, cfg)
	}
}

// --- Helper functions ---

func runTickerLoop(ctx context.Context, interval float64, fn func()) {
	ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fn()
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
