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

	// Signal handling for cleanup
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	var wg sync.WaitGroup
	for _, pair := range keyboardPairs {
		wg.Add(1)
		go func(p listener.KeyboardPair) {
			defer wg.Done()

			handler := func(code uint16, value int32) bool {
				if command, matched := m.HandleKeyEvent(code, value); matched {
					resolvedCmd := cfg.ResolveCommand(command)
					executor.Execute(resolvedCmd)
					return true
				}
				return false
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
