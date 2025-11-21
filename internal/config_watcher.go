package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	// Spawn new daemon using shared spawning logic
	// Pass 0 since we're exiting immediately after (no race)
	_, err := SpawnDaemon(0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to restart: %v\n", err)
		NotifyError("akeyshually reload failed", fmt.Sprintf("Failed to restart daemon: %v", err))
		os.Exit(1)
	}

	// Exit current daemon - new one is now running
	os.Exit(0)
}
