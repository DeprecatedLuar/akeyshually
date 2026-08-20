package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/deprecatedluar/akeyshually/internal/config"
)

// List shows all config files
func List() {
	configDir, err := config.GetConfigDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get config directory: %v\n", err)
		return
	}

	// List all .toml files
	files, err := filepath.Glob(filepath.Join(configDir, "*.toml"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list config files: %v\n", err)
		return
	}

	fmt.Println("config.toml")

	for _, file := range files {
		basename := filepath.Base(file)
		if basename == "config.toml" {
			continue
		}
		fmt.Println(basename)
	}
}
