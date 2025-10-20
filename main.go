package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/deprecatedluar/akeyshually/internal/config"
	"github.com/deprecatedluar/akeyshually/internal/executor"
	"github.com/deprecatedluar/akeyshually/internal/listener"
	"github.com/deprecatedluar/akeyshually/internal/matcher"
	"github.com/deprecatedluar/akeyshually/internal/watcher"
)

func createPressHandler(m *matcher.Matcher, cfg *config.Config, p listener.KeyboardPair) listener.KeyHandler {
	return func(code uint16, value int32) bool {
		// Handle modifier keys for tap detection
		if matcher.IsModifierKey(code) {
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
					executor.Execute(resolvedCmd)
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
			executor.Execute(resolvedCmd)
			return true
		}

		return false
	}
}

func createReleaseHandler(m *matcher.Matcher, cfg *config.Config, p listener.KeyboardPair) listener.KeyHandler {
	var bufferedKey uint16

	return func(code uint16, value int32) bool {
		// Handle modifier keys
		if matcher.IsModifierKey(code) {
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
					executor.Execute(resolvedCmd)
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
					executor.Execute(resolvedCmd)
				}

				bufferedKey = 0
				return true // Suppress the release of the buffered key
			}
		}

		return false
	}
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		os.Exit(1)
	}

	keyboardPairs, err := listener.FindKeyboards()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Keyboard detection error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("akeyshually started with %d keyboard(s):\n", len(keyboardPairs))
	for _, pair := range keyboardPairs {
		name, _ := pair.Physical.Name()
		fmt.Printf("  - %s\n", name)
	}

	m := matcher.New(cfg.Shortcuts)
	triggerMode := cfg.GetTriggerMode()

	// Start config file watcher for automatic reload
	configDir, err := config.GetConfigDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get config directory: %v\n", err)
		os.Exit(1)
	}
	go func() {
		if err := watcher.Watch(configDir); err != nil {
			fmt.Fprintf(os.Stderr, "Watcher error: %v\n", err)
		}
	}()

	// Signal handling for cleanup
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	var wg sync.WaitGroup
	for _, pair := range keyboardPairs {
		wg.Add(1)
		go func(p listener.KeyboardPair) {
			defer wg.Done()

			var handler listener.KeyHandler

			if triggerMode == "release" {
				handler = createReleaseHandler(m, cfg, p)
			} else {
				handler = createPressHandler(m, cfg, p)
			}

			if err := listener.Listen(p, handler); err != nil {
				fmt.Fprintf(os.Stderr, "Listener error: %v\n", err)
			}
		}(pair)
	}

	// Wait for signal in separate goroutine
	go func() {
		<-sigChan
		fmt.Fprintf(os.Stderr, "\nShutting down...\n")
		for _, pair := range keyboardPairs {
			listener.Cleanup(pair)
		}
		os.Exit(0)
	}()

	wg.Wait()
}
