package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	evdev "github.com/holoplot/go-evdev"

	daemon "github.com/deprecatedluar/luar-daemonator"

	"github.com/deprecatedluar/akeyshually/internal/commands"
	"github.com/deprecatedluar/akeyshually/internal/common"
	"github.com/deprecatedluar/akeyshually/internal/config"
	"github.com/deprecatedluar/akeyshually/internal/executor"
	"github.com/deprecatedluar/akeyshually/internal/handlers"
	"github.com/deprecatedluar/akeyshually/internal/listener"
	"github.com/deprecatedluar/akeyshually/internal/matcher"
	"github.com/deprecatedluar/akeyshually/internal/timers"
)

const githubRepo = "DeprecatedLuar/akeyshually"

var version = "dev"

func main() {
	// Process flags first, collect remaining args
	var remaining []string
	var configPath string

	for i := 0; i < len(os.Args[1:]); i++ {
		arg := os.Args[1:][i]
		switch arg {
		case "--debug":
			common.SetDebug(true)
		case "-c", "--config":
			if i+1 < len(os.Args[1:]) {
				configPath = os.Args[1:][i+1]
				i++ // Skip next arg
			} else {
				fmt.Fprintf(os.Stderr, "Error: %s requires a config path\n", arg)
				os.Exit(1)
			}
		default:
			remaining = append(remaining, arg)
		}
	}

	// No command given - run the keyboard listener in the foreground.
	// The service manager (systemd, systemd-run) or the user backgrounds
	// this; Run only owns single-instance enforcement and signal handling.
	if len(remaining) == 0 {
		d := daemon.New(common.AppName)
		if err := d.Run(func(ctx context.Context) error {
			return run(ctx, configPath)
		}); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

	// Handle commands
	command := remaining[0]

	switch command {
	case "stop":
		commands.Stop()
		os.Exit(0)
	case "restart":
		commands.Restart()
		os.Exit(0)
	case "update":
		if err := commands.HandleUpdate(version, githubRepo); err != nil {
			fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	case "enable":
		if len(remaining) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: akeyshually enable <file.toml>\n")
			os.Exit(1)
		}
		commands.Enable(remaining[1])
		os.Exit(0)
	case "disable":
		if len(remaining) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: akeyshually disable <file.toml>\n")
			os.Exit(1)
		}
		commands.Disable(remaining[1])
		os.Exit(0)
	case "list", "ls":
		commands.List()
		os.Exit(0)
	case "clear":
		commands.Clear()
		os.Exit(0)
	case "config", "conf", "edit":
		filename := ""
		if len(remaining) > 1 {
			filename = remaining[1]
		}
		commands.Config(filename)
		os.Exit(0)
	case "-e":
		filename := ""
		if len(remaining) > 1 {
			filename = remaining[1]
		}
		commands.Config(filename)
		os.Exit(0)
	case "help", "-h", "--help":
		commands.Help(remaining[1:]...)
		os.Exit(0)
	case "version", "-v", "--version":
		commands.Version(version)
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		commands.Help()
		os.Exit(1)
	}
}

func handleConfigError(err error) {
	// Check if it's a ValidationErrors type (possibly wrapped) and format nicely
	var ve config.ValidationErrors
	if errors.As(err, &ve) {
		ve.FormatWithGohelp()
	} else {
		// Fallback for other config errors
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
	}
	common.NotifyError("akeyshually startup failed", fmt.Sprintf("Config error: %v", err))
	os.Exit(1)
}

func run(ctx context.Context, configPath string) error {

	// Only ensure default config exists if not using custom config
	if configPath == "" {
		if err := config.EnsureConfigExists(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize config: %v\n", err)
			os.Exit(1)
		}
	}

	// Load config
	var cfg *config.Config
	var err error
	if configPath != "" {
		// Custom config - no overlays
		cfg, err = config.LoadFromPath(configPath)
		if err != nil {
			handleConfigError(err)
		}
	} else {
		// Default config with overlays
		enabledOverlays, err := config.ReadEnabledState()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to read enabled state: %v\n", err)
			enabledOverlays = []string{}
		}
		cfg, err = config.LoadWithOverlays(enabledOverlays)
		if err != nil {
			handleConfigError(err)
		}
		if len(enabledOverlays) > 0 {
			fmt.Printf("Enabled overlays: %v\n", enabledOverlays)
		}
	}

	result, err := listener.FindKeyboards()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Keyboard detection error: %v\n", err)
		common.NotifyError("akeyshually startup failed", fmt.Sprintf("Keyboard detection error: %v", err))
		os.Exit(1)
	}

	if len(result.Pairs) == 0 {
		fmt.Fprintf(os.Stderr, "No keyboards detected\n")
		common.NotifyError("akeyshually startup failed", "No keyboards detected")
		os.Exit(1)
	}

	var declaredResult listener.DeviceResult
	if len(cfg.Settings.Devices) > 0 {
		declaredResult, err = listener.FindDeclaredDevices(cfg.Settings.Devices)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: declared device error: %v\n", err)
		}
	}

	allPairs := append(result.Pairs, declaredResult.Pairs...)
	allFailures := append(result.Failures, declaredResult.Failures...)

	// ANSI color codes
	green := "\033[32m"
	purple := "\033[35m"
	dim := "\033[2m"
	reset := "\033[0m"

	fmt.Printf("Devices:\n")
	for _, pair := range allPairs {
		name, _ := pair.Physical.Name()
		fmt.Printf("  %s+ %s%s\n", green, name, reset)
	}
	for _, fail := range allFailures {
		fmt.Printf("  %s- %s%s %s(%s)%s\n", purple, fail.Name, reset, dim, fail.Reason, reset)
	}

	m := matcher.New(cfg.ParsedShortcuts)

	// Create shared tap state and detect mice (if tap shortcuts exist)
	var tapState *matcher.TapState
	mice, err := listener.FindMice()
	if err == nil && len(mice) > 0 {
		tapState = matcher.NewTapState()
		m.SetTapState(tapState)

		fmt.Printf("Monitoring %d mouse device(s) for tap cancellation\n", len(mice))
	}

	// Create focused output devices so keyboard remappers cannot capture pointer events.
	keyboardInjector, err := listener.CreateKeyboardInjector()
	if err != nil {
		return fmt.Errorf("create keyboard injector: %w", err)
	}
	defer destroyInjector("keyboard", keyboardInjector)

	pointerInjector, err := listener.CreatePointerInjector()
	if err != nil {
		return fmt.Errorf("create pointer injector: %w", err)
	}
	defer destroyInjector("pointer", pointerInjector)

	outputs := executor.Outputs{
		Keyboard: executor.NewEventSink(keyboardInjector),
		Pointer:  executor.NewEventSink(pointerInjector),
	}

	// Create shared loop state
	loopState := executor.NewLoopState()

	var wg sync.WaitGroup

	// Registry for thread-safe StateMap collection and mouse click cancellation
	registry := timers.NewStateMapRegistry()

	// Launch keyboard listeners with unified handler and reconnect support
	for _, pair := range result.Pairs {
		wg.Add(1)
		name, _ := pair.Physical.Name()
		go func(p listener.KeyboardPair, devName string) {
			defer wg.Done()
			stateMap := timers.NewStateMap()
			registry.Register(stateMap)
			emittedTracker := timers.NewEmittedModifierTracker()

			// ABS handler state (per device)
			absInfoMap := handlers.BuildAbsInfoMap(p.Physical)
			accumulators := make(handlers.AccumulatorMap)
			prevValues := make(handlers.PrevValuesMap)
			contactState := false

			execCtx := executor.ExecContext{
				Modifiers: m.GetCurrentModifiers(),
				LoopState: loopState,
				Outputs:   outputs,
				Virtual:   p.Virtual,
				Config:    cfg,
			}

			handler := func(code uint16, value int32) bool {
				// SYN event (flush accumulators)
				if code == 0xFFFF {
					handlers.FlushAbs(accumulators, absInfoMap, cfg, execCtx)
					return false
				}

				// ABS event (check if code exists in device's ABS capabilities)
				if _, isAbs := absInfoMap[code]; isAbs {
					return handlers.HandleAbs(code, value, absInfoMap, accumulators, prevValues, &contactState, cfg, execCtx)
				}

				// KEY event
				if cfg.Settings.DisableMediaKeys && listener.IsMediaKey(code) {
					return false
				}
				if value == 1 {
					return handlers.HandlePress(code, value, m, cfg, loopState, outputs, p.Virtual, stateMap, emittedTracker)
				}
				if value == 0 {
					return handlers.HandleRelease(code, value, m, cfg, loopState, outputs, p.Virtual, stateMap, emittedTracker)
				}
				return false
			}
			if err := listener.ListenWithReconnect(p, handler, listener.FindKeyboards, devName); err != nil {
				fmt.Fprintf(os.Stderr, "Listener error: %v\n", err)
			}
		}(pair, name)
	}

	// Launch declared device listeners
	declaredDeviceNames := cfg.Settings.Devices
	for _, pair := range declaredResult.Pairs {
		wg.Add(1)
		name, _ := pair.Physical.Name()
		go func(p listener.KeyboardPair, devName string) {
			defer wg.Done()
			stateMap := timers.NewStateMap()
			registry.Register(stateMap)
			emittedTracker := timers.NewEmittedModifierTracker()

			// ABS handler state (per device)
			absInfoMap := handlers.BuildAbsInfoMap(p.Physical)
			accumulators := make(handlers.AccumulatorMap)
			prevValues := make(handlers.PrevValuesMap)
			contactState := false

			execCtx := executor.ExecContext{
				Modifiers: m.GetCurrentModifiers(),
				LoopState: loopState,
				Outputs:   outputs,
				Virtual:   p.Virtual,
				Config:    cfg,
			}

			handler := func(code uint16, value int32) bool {
				// SYN event (flush accumulators)
				if code == 0xFFFF {
					handlers.FlushAbs(accumulators, absInfoMap, cfg, execCtx)
					return false
				}

				// ABS event (check if code exists in device's ABS capabilities)
				if _, isAbs := absInfoMap[code]; isAbs {
					return handlers.HandleAbs(code, value, absInfoMap, accumulators, prevValues, &contactState, cfg, execCtx)
				}

				// KEY event
				if cfg.Settings.DisableMediaKeys && listener.IsMediaKey(code) {
					return false
				}
				if value == 1 {
					return handlers.HandlePress(code, value, m, cfg, loopState, outputs, p.Virtual, stateMap, emittedTracker)
				}
				if value == 0 {
					return handlers.HandleRelease(code, value, m, cfg, loopState, outputs, p.Virtual, stateMap, emittedTracker)
				}
				return false
			}
			if err := listener.ListenWithReconnect(p, handler, func() (listener.DeviceResult, error) {
				return listener.FindDeclaredDevices(declaredDeviceNames)
			}, devName); err != nil {
				fmt.Fprintf(os.Stderr, "Listener error: %v\n", err)
			}
		}(pair, name)
	}

	// Launch mouse listeners (if tapState is active)
	if tapState != nil {
		for _, mouse := range mice {
			wg.Add(1)
			go func(dev evdev.InputDevice) {
				defer wg.Done()
				if err := listener.ListenMouse(&dev, func() {
					tapState.Clear()
					registry.CancelAllModifierLadders()
				}); err != nil {
					fmt.Fprintf(os.Stderr, "Mouse listener error: %v\n", err)
				}
			}(*mouse)
		}
	}

	// Listener goroutines block on device reads and only return on their
	// own (e.g. every device disconnected); ctx.Done() is what actually
	// signals a normal shutdown (SIGTERM/SIGINT). Either way, returning
	// here lets the deferred injector cleanup above run - unlike os.Exit,
	// which would skip it.
	listenersDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(listenersDone)
	}()

	select {
	case <-ctx.Done():
		fmt.Fprintf(os.Stderr, "\nShutting down...\n")
		for _, pair := range allPairs {
			listener.Cleanup(pair)
		}
	case <-listenersDone:
	}

	return nil
}

func destroyInjector(kind string, device *evdev.InputDevice) {
	if err := evdev.DestroyDevice(device); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to destroy %s injector: %v\n", kind, err)
	}
}
