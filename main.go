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
)

func createPressHandler(m *matcher.Matcher, cfg *config.Config, p listener.KeyboardPair) listener.KeyHandler {
	return func(code uint16, value int32) bool {
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
		// Update modifier state
		if matcher.IsModifierKey(code) {
			m.UpdateModifierState(code, value == 1)
			return false // Forward modifiers normally
		}

		// Key press
		if value == 1 {
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
