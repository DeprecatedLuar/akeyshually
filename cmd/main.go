package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	evdev "github.com/holoplot/go-evdev"

	"github.com/deprecatedluar/akeyshually/internal"
	"github.com/deprecatedluar/akeyshually/internal/config"
)

func isLoggingEnabled() bool {
	val := strings.ToLower(os.Getenv("LOGGING"))
	return val == "1" || val == "true" || val == "yes"
}

func createPressHandler(m *internal.Matcher, cfg *config.Config, p internal.KeyboardPair) internal.KeyHandler {
	logging := isLoggingEnabled()

	return func(code uint16, value int32) bool {
		// Handle modifier keys for tap detection
		if internal.IsModifierKey(code) {
			if value == 1 {
				// Modifier pressed
				m.UpdateModifierState(code, true)

				// Check if pressed alone (no other modifiers held)
				modifiers := m.GetCurrentModifiers()
				isAlone := true
				if isAlone {
					// Check no other modifiers are held
					if modifiers.Super {
						isAlone = !modifiers.Ctrl && !modifiers.Alt && !modifiers.Shift
					} else if modifiers.Ctrl {
						isAlone = !modifiers.Super && !modifiers.Alt && !modifiers.Shift
					} else if modifiers.Alt {
						isAlone = !modifiers.Super && !modifiers.Ctrl && !modifiers.Shift
					} else if modifiers.Shift {
						isAlone = !modifiers.Super && !modifiers.Ctrl && !modifiers.Alt
					}
				}

				if isAlone {
					m.MarkTapCandidate(code)
				}
			} else if value == 0 {
				// Modifier released - check for tap
				if command, matched := m.CheckTap(code); matched {
					resolvedCmd := cfg.ResolveCommand(command)
					if logging {
						fmt.Fprintf(os.Stderr, "[SHORTCUT] %s\n", resolvedCmd)
					}
					internal.Execute(resolvedCmd, cfg)
				}
				m.UpdateModifierState(code, false)
			}
			return false // Forward modifiers normally
		}

		// Regular key handling
		if value == 1 {
			// Any non-modifier key press clears tap candidate
			m.ClearTapCandidate()
		}

		command, matched := m.HandleKeyEvent(code, value)

		if matched {
			resolvedCmd := cfg.ResolveCommand(command)
			if logging {
				fmt.Fprintf(os.Stderr, "[SHORTCUT] %s\n", resolvedCmd)
			}
			internal.Execute(resolvedCmd, cfg)
			return true
		}

		return false
	}
}

func createReleaseHandler(m *internal.Matcher, cfg *config.Config, p internal.KeyboardPair) internal.KeyHandler {
	logging := isLoggingEnabled()
	var bufferedKey uint16

	return func(code uint16, value int32) bool {
		// Handle modifier keys
		if internal.IsModifierKey(code) {
			if value == 1 {
				// Modifier pressed
				m.UpdateModifierState(code, true)

				// Check if pressed alone (no other modifiers held)
				modifiers := m.GetCurrentModifiers()
				isAlone := bufferedKey == 0
				if isAlone {
					// Check no other modifiers are held
					if modifiers.Super {
						isAlone = !modifiers.Ctrl && !modifiers.Alt && !modifiers.Shift
					} else if modifiers.Ctrl {
						isAlone = !modifiers.Super && !modifiers.Alt && !modifiers.Shift
					} else if modifiers.Alt {
						isAlone = !modifiers.Super && !modifiers.Ctrl && !modifiers.Shift
					} else if modifiers.Shift {
						isAlone = !modifiers.Super && !modifiers.Ctrl && !modifiers.Alt
					}
				}

				if isAlone && bufferedKey == 0 {
					m.MarkTapCandidate(code)
				}
			} else if value == 0 {
				// Modifier released - check for tap
				if command, matched := m.CheckTap(code); matched {
					resolvedCmd := cfg.ResolveCommand(command)
					if logging {
						fmt.Fprintf(os.Stderr, "[SHORTCUT] %s\n", resolvedCmd)
					}
					internal.Execute(resolvedCmd, cfg)
				}
				m.UpdateModifierState(code, false)
			}
			return false // Forward modifiers normally
		}

		// Key press
		if value == 1 {
			// Any non-modifier key press clears tap candidate (regardless of match)
			m.ClearTapCandidate()

			// Check if this would match a shortcut
			if _, matched := m.WouldMatch(code); matched {
				bufferedKey = code
				return true // Buffer it, don't forward
			}
			return false // Not a match, forward normally
		}

		// Key release
		if value == 0 {
			// Check if this is the release of a buffered key
			if code == bufferedKey && bufferedKey != 0 {
				// Execute the shortcut
				if command, matched := m.WouldMatch(code); matched {
					resolvedCmd := cfg.ResolveCommand(command)
					if logging {
						fmt.Fprintf(os.Stderr, "[SHORTCUT] %s\n", resolvedCmd)
					}
					internal.Execute(resolvedCmd, cfg)
				}

				bufferedKey = 0
				return true // Suppress the release of the buffered key
			}
		}

		return false
	}
}

func main() {
	if err := config.EnsureConfigExists(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize config: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		os.Exit(1)
	}

	keyboardPairs, err := internal.FindKeyboards()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Keyboard detection error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("akeyshually started with %d keyboard(s):\n", len(keyboardPairs))
	for _, pair := range keyboardPairs {
		name, _ := pair.Physical.Name()
		fmt.Printf("  - %s\n", name)
	}

	m := internal.New(cfg.Shortcuts)
	triggerMode := cfg.GetTriggerMode()

	// Create shared tap state and detect mice (if tap shortcuts exist)
	var tapState *internal.TapState
	mice, err := internal.FindMice()
	if err == nil && len(mice) > 0 {
		tapState = internal.NewTapState()
		m.SetTapState(tapState)
		fmt.Printf("Monitoring %d mouse device(s) for tap cancellation\n", len(mice))
	}

	// Start config file watcher for automatic reload
	configDir, err := config.GetConfigDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get config directory: %v\n", err)
		os.Exit(1)
	}
	go func() {
		if err := internal.Watch(configDir); err != nil {
			fmt.Fprintf(os.Stderr, "Watcher error: %v\n", err)
		}
	}()

	// Signal handling for cleanup
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	var wg sync.WaitGroup

	// Launch keyboard listeners
	for _, pair := range keyboardPairs {
		wg.Add(1)
		go func(p internal.KeyboardPair) {
			defer wg.Done()

			var handler internal.KeyHandler

			if triggerMode == "release" {
				handler = createReleaseHandler(m, cfg, p)
			} else {
				handler = createPressHandler(m, cfg, p)
			}

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
		os.Exit(0)
	}()

	wg.Wait()
}
