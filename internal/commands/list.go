package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/deprecatedluar/akeyshually/internal"
	"github.com/deprecatedluar/akeyshually/internal/config"
)

// List shows all config files and their enabled status
func List() {
	configDir, err := config.GetConfigDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get config directory: %v\n", err)
		return
	}

	enabled, err := internal.ReadEnabledState()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to read enabled state: %v\n", err)
		enabled = []string{}
	}

	// List all .toml files
	files, err := filepath.Glob(filepath.Join(configDir, "*.toml"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list config files: %v\n", err)
		return
	}

	enabledMap := make(map[string]bool)
	for _, e := range enabled {
		enabledMap[e] = true
	}

	fmt.Println("Configuration files:")
	fmt.Printf("  config.toml         [base - always active]\n")

	for _, file := range files {
		basename := filepath.Base(file)
		if basename == "config.toml" {
			continue
		}

		status := "[disabled]"
		if enabledMap[basename] {
			status = "[enabled]"
		}
		fmt.Printf("  %-20s %s\n", basename, status)
	}
}
