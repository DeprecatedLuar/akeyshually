package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	evdev "github.com/holoplot/go-evdev"

	"github.com/deprecatedluar/akeyshually/internal"
	"github.com/deprecatedluar/akeyshually/internal/commands"
	"github.com/deprecatedluar/akeyshually/internal/common"
	"github.com/deprecatedluar/akeyshually/internal/config"
	"github.com/deprecatedluar/akeyshually/internal/executor"
	"github.com/deprecatedluar/akeyshually/internal/listener"
	"github.com/deprecatedluar/akeyshually/internal/matcher"
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

	// No command given - run the keyboard listener
	if len(remaining) == 0 {
		run(configPath)
		return
	}

	// Handle commands
	command := remaining[0]

	switch command {
	case "start":
		commands.Start()
		os.Exit(0)
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

func run(configPath string) {

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

	keyboardPairs, err := listener.FindKeyboards()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Keyboard detection error: %v\n", err)
		common.NotifyError("akeyshually startup failed", fmt.Sprintf("Keyboard detection error: %v", err))
		os.Exit(1)
	}

	if len(keyboardPairs) == 0 {
		fmt.Fprintf(os.Stderr, "No keyboards detected\n")
		common.NotifyError("akeyshually startup failed", "No keyboards detected")
		os.Exit(1)
	}

	var declaredPairs []listener.KeyboardPair
	if len(cfg.Settings.Devices) > 0 {
		declaredPairs, err = listener.FindDeclaredDevices(cfg.Settings.Devices)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: declared device error: %v\n", err)
		}
	}

	allPairs := append(keyboardPairs, declaredPairs...)
	fmt.Printf("akeyshually started with %d keyboard(s):\n", len(allPairs))
	for _, pair := range allPairs {
		name, _ := pair.Physical.Name()
		fmt.Printf("  - %s\n", name)
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

	// Create shared key injector for remap output
	injector, err := listener.CreateKeyInjector()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create key injector: %v\n", err)
		os.Exit(1)
	}
	defer evdev.DestroyDevice(injector)

	// Create shared loop state
	loopState := executor.NewLoopState()

	// Signal handling for cleanup
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	var wg sync.WaitGroup

	// Launch keyboard listeners with unified handler and reconnect support
	for _, pair := range keyboardPairs {
		wg.Add(1)
		name, _ := pair.Physical.Name()
		go func(p listener.KeyboardPair, devName string) {
			defer wg.Done()
			handler := internal.CreateUnifiedHandler(m, cfg, loopState, injector, p.Virtual)
			if err := listener.ListenWithReconnect(p, handler, listener.FindKeyboards, devName); err != nil {
				fmt.Fprintf(os.Stderr, "Listener error: %v\n", err)
			}
		}(pair, name)
	}

	// Launch declared device listeners
	declaredDeviceNames := cfg.Settings.Devices
	for _, pair := range declaredPairs {
		wg.Add(1)
		name, _ := pair.Physical.Name()
		go func(p listener.KeyboardPair, devName string) {
			defer wg.Done()
			handler := internal.CreateUnifiedHandler(m, cfg, loopState, injector, p.Virtual)
			if err := listener.ListenWithReconnect(p, handler, func() ([]listener.KeyboardPair, error) {
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
				if err := listener.ListenMouse(&dev, tapState.Clear); err != nil {
					fmt.Fprintf(os.Stderr, "Mouse listener error: %v\n", err)
				}
			}(*mouse)
		}
	}

	// Wait for signal in separate goroutine
	go func() {
		<-sigChan
		fmt.Fprintf(os.Stderr, "\nShutting down...\n")
		for _, pair := range allPairs {
			listener.Cleanup(pair)
		}
		os.Exit(0)
	}()

	wg.Wait()
}
