package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	evdev "github.com/holoplot/go-evdev"

	"github.com/deprecatedluar/akeyshually/internal"
	"github.com/deprecatedluar/akeyshually/internal/commands/handler"
	"github.com/deprecatedluar/akeyshually/internal/config"
)

func main() {
	// Parse CLI arguments and execute commands
	// Returns true only for foreground mode
	if handler.Parse(os.Args[1:]) {
		startDaemon()
	}
}

func startDaemon() {
	// Check for existing instances
	pid, err := internal.GetRunningDaemonPid()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to check daemon status: %v\n", err)
		os.Exit(1)
	}

	// If we're replacing a specific PID (restart scenario), allow it
	if pid > 0 {
		replacingPid := os.Getenv("AKEYSHUALLY_REPLACING")
		if replacingPid != "" && replacingPid == fmt.Sprintf("%d", pid) {
			// This is expected - we're replacing the old daemon
			// It might still be shutting down, just proceed
		} else {
			fmt.Fprintf(os.Stderr, "Errm... akeyshually, the daemon is already running (PID: %d)\n", pid)
			os.Exit(1)
		}
	}

	// Write PID file for current process
	currentPid := os.Getpid()
	if err := internal.WritePidFile(currentPid); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to write pidfile: %v\n", err)
	}

	if err := config.EnsureConfigExists(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize config: %v\n", err)
		os.Exit(1)
	}

	// Load enabled overlays
	enabledOverlays, err := internal.ReadEnabledState()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to read enabled state: %v\n", err)
		enabledOverlays = []string{}
	}

	// Load config with overlays
	cfg, err := config.LoadWithOverlays(enabledOverlays)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		internal.NotifyError("akeyshually startup failed", fmt.Sprintf("Config error: %v", err))
		os.Exit(1)
	}

	// Print enabled overlays
	if len(enabledOverlays) > 0 {
		fmt.Printf("Enabled overlays: %v\n", enabledOverlays)
	}

	keyboardPairs, err := internal.FindKeyboards()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Keyboard detection error: %v\n", err)
		internal.NotifyError("akeyshually startup failed", fmt.Sprintf("Keyboard detection error: %v", err))
		os.Exit(1)
	}

	if len(keyboardPairs) == 0 {
		fmt.Fprintf(os.Stderr, "No keyboards detected\n")
		internal.NotifyError("akeyshually startup failed", "No keyboards detected")
		os.Exit(1)
	}

	fmt.Printf("akeyshually started with %d keyboard(s):\n", len(keyboardPairs))
	for _, pair := range keyboardPairs {
		name, _ := pair.Physical.Name()
		fmt.Printf("  - %s\n", name)
	}

	m := internal.New(cfg.ParsedShortcuts)

	// Create shared tap state and detect mice (if tap shortcuts exist)
	var tapState *internal.TapState
	var doubleTapState *internal.DoubleTapState
	mice, err := internal.FindMice()
	if err == nil && len(mice) > 0 {
		tapState = internal.NewTapState()
		m.SetTapState(tapState)

		doubleTapState = internal.NewDoubleTapState()
		m.SetDoubleTapState(doubleTapState)

		fmt.Printf("Monitoring %d mouse device(s) for tap cancellation\n", len(mice))
	}

	// Create shared loop state
	loopState := internal.NewLoopState()

	// Signal handling for cleanup
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	var wg sync.WaitGroup

	// Launch keyboard listeners with unified handler
	for _, pair := range keyboardPairs {
		wg.Add(1)
		go func(p internal.KeyboardPair) {
			defer wg.Done()

			handler := internal.CreateUnifiedHandler(m, cfg, loopState)

			if err := internal.Listen(p, handler); err != nil {
				fmt.Fprintf(os.Stderr, "Listener error: %v\n", err)
			}
		}(pair)
	}

	// Launch mouse listeners (if tapState is active)
	if tapState != nil {
		for _, mouse := range mice {
			wg.Add(1)
			go func(dev evdev.InputDevice) {
				defer wg.Done()
				if err := internal.ListenMouse(&dev, func() {
					tapState.Clear()
					if doubleTapState != nil {
						doubleTapState.Clear()
					}
				}); err != nil {
					fmt.Fprintf(os.Stderr, "Mouse listener error: %v\n", err)
				}
			}(*mouse)
		}
	}

	// Wait for signal in separate goroutine
	go func() {
		<-sigChan
		fmt.Fprintf(os.Stderr, "\nShutting down...\n")
		for _, pair := range keyboardPairs {
			internal.Cleanup(pair)
		}
		// Clean up pidfile if running as daemon
		internal.RemovePidFile()
		os.Exit(0)
	}()

	wg.Wait()
}
