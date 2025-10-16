package main

import (
	"fmt"
	"os"
	"sync"

	evdev "github.com/holoplot/go-evdev"

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

	keyboards, err := listener.FindKeyboards()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Keyboard detection error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("akeyshually started with %d keyboard(s):\n", len(keyboards))
	for _, kbd := range keyboards {
		name, _ := kbd.Name()
		fmt.Printf("  - %s\n", name)
	}

	m := matcher.New(cfg.Shortcuts)

	var wg sync.WaitGroup
	for _, kbd := range keyboards {
		wg.Add(1)
		go func(dev *evdev.InputDevice) {
			defer wg.Done()

			handler := func(code uint16, value int32) {
				if command, matched := m.HandleKeyEvent(code, value); matched {
					resolvedCmd := cfg.ResolveCommand(command)
					executor.Execute(resolvedCmd)
				}
			}

			if err := listener.Listen(dev, handler); err != nil {
				fmt.Fprintf(os.Stderr, "Listener error: %v\n", err)
			}
		}(kbd)
	}

	wg.Wait()
}
