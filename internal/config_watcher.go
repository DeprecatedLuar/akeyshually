package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watch monitors the config directory for changes and restarts the daemon
func Watch(configDir string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	if err := watcher.Add(configDir); err != nil {
		return fmt.Errorf("failed to watch directory %s: %w", configDir, err)
	}

	// If config.toml is a symlink, also watch the target directory
	configPath := filepath.Join(configDir, "config.toml")
	if realPath, err := filepath.EvalSymlinks(configPath); err == nil && realPath != configPath {
		realDir := filepath.Dir(realPath)
		if realDir != configDir {
			if err := watcher.Add(realDir); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to watch symlink target directory %s: %v\n", realDir, err)
			}
		}
	}

	var lastRestart time.Time
	debounce := 1 * time.Second

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			// Only trigger on write/create events for .toml files
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				if strings.HasSuffix(event.Name, ".toml") {
					basename := filepath.Base(event.Name)

					// Check if this file should trigger reload
					shouldReload := basename == "config.toml"
					if !shouldReload {
						// Check if it's an enabled overlay
						enabled, _ := ReadEnabledState()
						for _, e := range enabled {
							if basename == e {
								shouldReload = true
								break
							}
						}
					}

					if shouldReload {
						// Debounce: ignore if we just restarted
						if time.Since(lastRestart) < debounce {
							continue
						}
						lastRestart = time.Now()
						restartSelf()
						return nil
					}
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "Watcher error: %v\n", err)
		}
	}
}

// restartSelf re-executes the current process and exits
func restartSelf() {
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get executable path: %v\n", err)
		os.Exit(1)
	}

	// Resolve symlinks to get actual binary path
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to resolve executable path: %v\n", err)
		os.Exit(1)
	}

	// Execute the same binary with the same arguments
	cmd := exec.Command(executable, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Start in new session to detach from current process group
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to restart: %v\n", err)
		NotifyError("akeyshually reload failed", fmt.Sprintf("Failed to restart daemon: %v", err))
		os.Exit(1)
	}

	os.Exit(0)
}
