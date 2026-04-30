package ladder

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/deprecatedluar/akeyshually/internal/common"
	"github.com/deprecatedluar/akeyshually/internal/config"
	"github.com/deprecatedluar/akeyshually/internal/executor"
	"github.com/deprecatedluar/akeyshually/internal/keys"
	"github.com/deprecatedluar/akeyshually/internal/matcher"
	"github.com/deprecatedluar/akeyshually/internal/timers"
	evdev "github.com/holoplot/go-evdev"
)

// Run is the timer ladder goroutine. Uses elimination model: prune impossible candidates
// after every event, last standing wins immediately.
func Run(
	ctx context.Context,
	state *timers.ComboState,
	combo string,
	keyCode uint16,
	value int32,
	candidates []timers.Candidate,
	cfg *config.Config,
	loopState *executor.LoopState,
	injector *evdev.InputDevice,
	virtual *evdev.InputDevice,
	modifiers matcher.ModifierState,
	stateMap *timers.StateMap,
	emittedTracker *timers.EmittedModifierTracker,
	shortcuts map[string][]*config.ParsedShortcut,
) {
	common.LogDebug(">>> LADDER GOROUTINE STARTED for %s", combo)
	defer stateMap.Delete(combo)

	// State tracking
	count := 1      // First press already happened
	pressed := true // Key is down
	phase := 0      // Current phase boundary crossed

	// Precompute context for elimination rules
	hasHold := false
	for _, c := range candidates {
		if isHoldBehavior(c.Shortcut.Behavior) {
			hasHold = true
			break
		}
	}
	common.LogDebug("Ladder %s: initial candidates=%s, hasHold=%v", combo, formatCandidates(candidates), hasHold)

	// Build timer ladder: sorted unique thresholds
	ladder := buildTimerLadder(candidates, cfg.Settings.DefaultInterval)
	common.LogDebug("Ladder %s: timer phases=%v", combo, ladder)

	var timer *time.Timer
	var timerCh <-chan time.Time

	// Start first timer if ladder exists
	if len(ladder) > 0 {
		timer = time.NewTimer(ladder[0])
		timerCh = timer.C
		common.LogDebug("Doubletap timer started: %vms", ladder[0].Milliseconds())
	}

	for {
		select {
		case <-ctx.Done():
			common.LogDebug("Ladder for %s cancelled", combo)
			if timer != nil {
				timer.Stop()
			}
			return

		case newKey := <-state.EscapeCh:
			// Foreign key pressed - migrate ladder to combo
			newCombo := combo + "+" + keys.GetKeyName(newKey)
			common.LogDebug(">>> LADDER %s: ESCAPE HATCH → migrating to %s", combo, newCombo)
			if timer != nil {
				timer.Stop()
			}
			// stateMap already has modifier entry — delete it, new combo will register itself
			stateMap.Delete(combo)

			// Build new combo string
			newShortcuts := shortcuts[newCombo] // already confirmed non-empty in HandlePress
			newCandidates := timers.BuildCandidates(newShortcuts)
			common.LogDebug(">>> LADDER %s: spawning with candidates=%s", newCombo, formatCandidates(newCandidates))

			// Spawn new ladder for the combo
			newCtx, newCancel := context.WithCancel(context.Background())
			newState := timers.NewComboState(newCancel)
			stateMap.Set(newCombo, newState)
			go Run(newCtx, newState, newCombo, newKey, value, newCandidates, cfg,
				loopState, injector, virtual, modifiers, stateMap, emittedTracker, shortcuts)
			return

		case <-state.PressCh:
			// Second press arrived
			common.LogDebug(">>> LADDER %s: PressCh (count: %d→%d, pressed: %v, phase: %d)", combo, count, count+1, true, phase)
			count++
			pressed = true

			// Prune eliminated candidates
			beforePrune := len(candidates)
			candidates = pruneCandidates(candidates, count, pressed, phase, hasHold)
			common.LogDebug("Ladder %s: pruned %d→%d survivors=%s", combo, beforePrune, len(candidates), formatCandidates(candidates))

			if len(candidates) == 0 {
				common.LogDebug(">>> LADDER %s: NO WINNER (all eliminated after press)", combo)
				if timer != nil {
					timer.Stop()
				}
				return
			}

			// Last standing wins
			if len(candidates) == 1 {
				common.LogDebug(">>> LADDER %s: WINNER=%s (last standing after press)", combo, behaviorName(candidates[0].Shortcut.Behavior))
				if timer != nil {
					timer.Stop()
				}
				fireWinner(combo, keyCode, value, &candidates[0], cfg, loopState, injector, virtual, modifiers, ctx, state)
				return
			}

		case <-state.ReleaseCh:
			// Key released
			pressed = false
			common.LogDebug(">>> LADDER %s: ReleaseCh (count: %d, pressed: %v, phase: %d)", combo, count, pressed, phase)

			// Prune eliminated candidates
			beforePrune := len(candidates)
			candidates = pruneCandidates(candidates, count, pressed, phase, hasHold)
			common.LogDebug("Ladder %s: pruned %d→%d survivors=%s", combo, beforePrune, len(candidates), formatCandidates(candidates))

			if len(candidates) == 0 {
				common.LogDebug(">>> LADDER %s: NO WINNER (all eliminated after release)", combo)
				if timer != nil {
					timer.Stop()
				}
				return
			}

			// Last standing wins
			if len(candidates) == 1 {
				common.LogDebug(">>> LADDER %s: WINNER=%s (last standing after release)", combo, behaviorName(candidates[0].Shortcut.Behavior))
				if timer != nil {
					timer.Stop()
				}
				fireWinner(combo, keyCode, value, &candidates[0], cfg, loopState, injector, virtual, modifiers, ctx, state)
				return
			}

		case <-timerCh:
			// Timer expired, advance phase
			phase++
			common.LogDebug(">>> LADDER %s: Timer expired (count: %d, pressed: %v, phase: %d)", combo, count, pressed, phase)

			// Prune eliminated candidates
			beforePrune := len(candidates)
			candidates = pruneCandidates(candidates, count, pressed, phase, hasHold)
			common.LogDebug("Ladder %s: pruned %d→%d survivors=%s", combo, beforePrune, len(candidates), formatCandidates(candidates))

			// Last standing wins
			if len(candidates) == 1 {
				common.LogDebug(">>> LADDER %s: WINNER=%s (last standing after timer)", combo, behaviorName(candidates[0].Shortcut.Behavior))
				fireWinner(combo, keyCode, value, &candidates[0], cfg, loopState, injector, virtual, modifiers, ctx, state)
				return
			}

			// No winner yet (either 0 or multiple survivors)
			if len(candidates) == 0 {
				common.LogDebug(">>> LADDER %s: NO WINNER (all eliminated at phase %d)", combo, phase)

				// If this is a modifier key that was suppressed, emit it now so system receives it
				if isModifierCombo(combo) && virtual != nil {
					common.LogDebug("Emitting unmatched modifier %s to system (pressed=%v)", combo, pressed)
					EmitModifierKey(virtual, keys.ResolveKeyCode, combo, pressed)
					if pressed {
						emittedTracker.MarkEmitted(combo)
					}
				}
				return
			}

			// Multiple survivors remain, advance to next timer threshold
			if phase < len(ladder) {
				timer.Reset(ladder[phase])
				timerCh = timer.C
				common.LogDebug("Ladder %s: advancing to next timer phase=%d duration=%vms", combo, phase+1, ladder[phase].Milliseconds())
			} else {
				// No more timers, ladder exhausted with no winner
				common.LogDebug(">>> LADDER %s: NO WINNER (ladder exhausted, %d survivors remain)", combo, len(candidates))

				// If this is a modifier key that was suppressed, emit it now so system receives it
				if isModifierCombo(combo) && virtual != nil {
					common.LogDebug("Emitting unmatched modifier %s to system (pressed=%v)", combo, pressed)
					EmitModifierKey(virtual, keys.ResolveKeyCode, combo, pressed)
					if pressed {
						emittedTracker.MarkEmitted(combo)
					}
				}
				return
			}
		}
	}
}

// buildTimerLadder extracts unique sorted timer thresholds from candidates.
// Phases are derived from behavior type, not win conditions.
func buildTimerLadder(candidates []timers.Candidate, defaultInterval float64) []time.Duration {
	thresholds := make(map[int]time.Duration)

	for _, c := range candidates {
		interval := intervalOrDefault(c.Shortcut.Interval, defaultInterval)

		switch c.Shortcut.Behavior {
		case config.BehaviorDoubleTap:
			// Needs Phase 1 timer for doubletap window
			if existing, ok := thresholds[1]; !ok || ms(interval) > existing {
				thresholds[1] = ms(interval)
			}

		case config.BehaviorHold, config.BehaviorHoldRelease, config.BehaviorLongPress:
			// Needs Phase 1 timer for hold threshold
			if existing, ok := thresholds[1]; !ok || ms(interval) > existing {
				thresholds[1] = ms(interval)
			}

		case config.BehaviorTapHold, config.BehaviorTapLongPress:
			// Needs Phase 1 timer for tap window
			if existing, ok := thresholds[1]; !ok || ms(interval) > existing {
				thresholds[1] = ms(interval)
			}
			// Needs Phase 2 timer for hold threshold
			holdInterval := intervalOrDefault(c.Shortcut.HoldInterval, defaultInterval)
			if existing, ok := thresholds[2]; !ok || ms(holdInterval) > existing {
				thresholds[2] = ms(holdInterval)
			}

		// BehaviorNormal and BehaviorPressRelease don't require timers themselves,
		// but they participate in elimination when other behaviors set timers
		}
	}

	// Convert to sorted slice
	var ladder []time.Duration
	for phase := 1; phase <= len(thresholds); phase++ {
		if duration, exists := thresholds[phase]; exists {
			ladder = append(ladder, duration)
		}
	}

	return ladder
}

// pruneCandidates removes candidates that are eliminated (can no longer win).
// Uses behavior-specific elimination rules. Last survivor wins.
func pruneCandidates(candidates []timers.Candidate, count int, pressed bool, phase int, hasHold bool) []timers.Candidate {
	pruned := make([]timers.Candidate, 0, len(candidates))
	for _, c := range candidates {
		if !isEliminated(c.Shortcut.Behavior, count, pressed, phase, hasHold) {
			pruned = append(pruned, c)
		}
	}
	return pruned
}

// isEliminated returns true if this behavior can no longer win given current ladder state.
// Implements pure elimination model: a candidate is dead when its outcome is impossible.
func isEliminated(b config.BehaviorMode, count int, pressed bool, phase int, hasHold bool) bool {
	switch b {
	case config.BehaviorNormal, config.BehaviorPressRelease:
		// Eliminated by second press (count > 1 is doubletap territory)
		if count > 1 {
			return true
		}
		// Eliminated by holding past threshold IF hold is competing
		if phase >= 1 && pressed && hasHold {
			return true
		}
		return false

	case config.BehaviorHold, config.BehaviorHoldRelease, config.BehaviorLongPress:
		// Eliminated by releasing before threshold (hold can never fire)
		if !pressed && phase < 1 {
			return true
		}
		return false

	case config.BehaviorDoubleTap:
		// Eliminated by window expiry with no second press
		if phase >= 1 && count < 2 {
			return true
		}
		return false

	case config.BehaviorTapHold, config.BehaviorTapLongPress:
		// Eliminated by releasing after tap window expired without second press
		if !pressed && count < 2 && phase >= 1 {
			return true
		}
		// Eliminated by releasing within tap window on first press (onpress territory)
		if !pressed && count == 1 && phase == 0 {
			return true
		}
		return false

	default:
		// Unknown behavior, don't eliminate
		return false
	}
}

// isHoldBehavior returns true if behavior is hold-family (needs hold threshold)
func isHoldBehavior(b config.BehaviorMode) bool {
	return b == config.BehaviorHold || b == config.BehaviorHoldRelease || b == config.BehaviorLongPress
}

// fireWinner executes the winning candidate's shortcut
func fireWinner(
	combo string,
	keyCode uint16,
	value int32,
	winner *timers.Candidate,
	cfg *config.Config,
	loopState *executor.LoopState,
	injector *evdev.InputDevice,
	virtual *evdev.InputDevice,
	modifiers matcher.ModifierState,
	ctx context.Context,
	state *timers.ComboState,
) {
	s := winner.Shortcut

	// Build execution context
	execCtx := executor.ExecContext{
		KeyCode:   keyCode,
		Value:     value,
		Virtual:   virtual,
		Injector:  injector,
		Modifiers: modifiers,
		Config:    cfg,
		LoopState: loopState,
	}

	switch s.Behavior {
	case config.BehaviorNormal:
		common.LogMatch(combo, combo)
		if s.Repeat {
			loopState.ToggleLoop(combo, s, cfg)
		} else {
			resolvedCmd := cfg.ResolveCommand(s.Commands[0])
			common.LogTrigger(resolvedCmd)
			executor.Run(resolvedCmd, execCtx)
		}

	case config.BehaviorPressRelease:
		common.LogMatch(combo+".pressrelease", combo)
		if s.Commands[0] != "" {
			resolvedCmd := cfg.ResolveCommand(s.Commands[0])
			common.LogTrigger(resolvedCmd)
			executor.Run(resolvedCmd, execCtx)
		}
		// 10ms gap between press and release commands
		time.AfterFunc(10*time.Millisecond, func() {
			if s.Commands[1] != "" {
				resolvedCmd := cfg.ResolveCommand(s.Commands[1])
				common.LogTrigger(resolvedCmd)
				executor.Run(resolvedCmd, execCtx)
			}
		})

	case config.BehaviorHold, config.BehaviorLongPress:
		common.LogMatch(combo+".hold", combo)
		resolvedCmd := cfg.ResolveCommand(s.Commands[0])
		if s.Repeat {
			loopState.StartLoop(combo, s, cfg)
			select {
			case <-ctx.Done():
			case <-state.ReleaseCh:
			}
			loopState.StopLoop(combo)
		} else if strings.HasPrefix(resolvedCmd, ">>") {
			// Remap hold forever - needs special lifecycle management
			loopState.StartHeldProcess(combo, s, cfg, injector, modifiers)
			select {
			case <-ctx.Done():
			case <-state.ReleaseCh:
			}
			loopState.StopHeldProcess(combo, injector)
		} else {
			common.LogTrigger(resolvedCmd)
			executor.Run(resolvedCmd, execCtx)
			// For longpress, exit immediately
			if s.Behavior != config.BehaviorLongPress {
				select {
				case <-ctx.Done():
				case <-state.ReleaseCh:
				}
			}
		}

	case config.BehaviorHoldRelease:
		common.LogMatch(combo+".holdrelease", combo)
		if s.Commands[0] != "" {
			resolvedCmd := cfg.ResolveCommand(s.Commands[0])
			common.LogTrigger(resolvedCmd)
			executor.Run(resolvedCmd, execCtx)
		}
		select {
		case <-ctx.Done():
		case <-state.ReleaseCh:
		}
		if s.Commands[1] != "" {
			resolvedCmd := cfg.ResolveCommand(s.Commands[1])
			common.LogMatch(combo+".holdrelease.release", combo)
			common.LogTrigger(resolvedCmd)
			executor.Run(resolvedCmd, execCtx)
		}

	case config.BehaviorDoubleTap:
		fire(combo+".doubletap", s.Commands[0], cfg, execCtx)

	case config.BehaviorTapHold:
		common.LogMatch(combo+".taphold", combo)
		resolvedCmd := cfg.ResolveCommand(s.Commands[0])
		common.LogTrigger(resolvedCmd)
		cmd := executor.ExecuteTracked(resolvedCmd, cfg)
		if cmd != nil {
			loopState.Mu.Lock()
			loopState.HeldProcesses[combo] = cmd
			loopState.Mu.Unlock()
		}
		select {
		case <-ctx.Done():
		case <-state.ReleaseCh:
		}
		loopState.Mu.Lock()
		if c, exists := loopState.HeldProcesses[combo]; exists {
			executor.StopProcess(c)
			delete(loopState.HeldProcesses, combo)
		}
		loopState.Mu.Unlock()

	case config.BehaviorTapLongPress:
		common.LogMatch(combo+".taplongpress", combo)
		resolvedCmd := cfg.ResolveCommand(s.Commands[1])
		common.LogTrigger(resolvedCmd)
		executor.Run(resolvedCmd, execCtx)
	}
}

// fire is a helper for simple one-shot command execution with logging.
func fire(label, command string, cfg *config.Config, execCtx executor.ExecContext) {
	resolvedCmd := cfg.ResolveCommand(command)
	common.LogMatch(label, label)
	common.LogTrigger(resolvedCmd)
	executor.Run(resolvedCmd, execCtx)
}

// ms converts float64 milliseconds to time.Duration.
func ms(d float64) time.Duration {
	return time.Duration(d) * time.Millisecond
}

func intervalOrDefault(interval, def float64) float64 {
	if interval > 0 {
		return interval
	}
	return def
}

// isModifierCombo checks if a combo is a lone modifier key
func isModifierCombo(combo string) bool {
	return combo == "super" || combo == "ctrl" || combo == "alt" || combo == "shift"
}

// EmitModifierKey emits a modifier key event to the virtual keyboard
func EmitModifierKey(virtual *evdev.InputDevice, resolver func(string) (uint16, bool), keyName string, down bool) {
	code, ok := resolver(keyName)
	if !ok {
		fmt.Fprintf(os.Stderr, "Failed to resolve modifier key: %s\n", keyName)
		return
	}

	value := int32(0)
	if down {
		value = 1
	}

	common.LogDebug("emitModifierKey: writing %s (code=%d, value=%d) to virtual device", keyName, code, value)
	err1 := virtual.WriteOne(&evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.EvCode(code), Value: value})
	err2 := virtual.WriteOne(&evdev.InputEvent{Type: evdev.EV_SYN, Code: evdev.SYN_REPORT, Value: 0})
	common.LogDebug("emitModifierKey: write results: key_err=%v, syn_err=%v", err1, err2)
}

// formatCandidates returns a compact string representation of candidate behaviors for debug logging
func formatCandidates(candidates []timers.Candidate) string {
	if len(candidates) == 0 {
		return "[]"
	}
	var parts []string
	for _, c := range candidates {
		parts = append(parts, behaviorName(c.Shortcut.Behavior))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// behaviorName converts BehaviorMode to string for debug logging
func behaviorName(b config.BehaviorMode) string {
	switch b {
	case config.BehaviorNormal:
		return "normal"
	case config.BehaviorHold:
		return "hold"
	case config.BehaviorLongPress:
		return "longpress"
	case config.BehaviorSwitch:
		return "switch"
	case config.BehaviorDoubleTap:
		return "doubletap"
	case config.BehaviorPressRelease:
		return "pressrelease"
	case config.BehaviorHoldRelease:
		return "holdrelease"
	case config.BehaviorTapHold:
		return "taphold"
	case config.BehaviorTapLongPress:
		return "taplongpress"
	default:
		return "unknown"
	}
}
