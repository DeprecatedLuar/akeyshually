package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/deprecatedluar/akeyshually/internal/config"
)

const ansiBold = "\033[1m"
const ansiDim = "\033[2m"
const ansiReset = "\033[0m"

// Status shows config files grouped by enabled state, enabled first
func Status() {
	configDir, err := config.GetConfigDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get config directory: %v\n", err)
		return
	}

	enabled, err := config.ReadEnabledState()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to read enabled state: %v\n", err)
		enabled = []string{}
	}

	files, err := filepath.Glob(filepath.Join(configDir, "*.toml"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list config files: %v\n", err)
		return
	}

	enabledMap := make(map[string]bool)
	for _, e := range enabled {
		enabledMap[e] = true
	}

	var enabledNames, disabledNames []string
	for _, file := range files {
		basename := filepath.Base(file)
		if basename == "config.toml" {
			continue
		}
		name := strings.TrimSuffix(basename, ".toml")
		if enabledMap[basename] {
			enabledNames = append(enabledNames, name)
		} else {
			disabledNames = append(disabledNames, name)
		}
	}

	sort.Strings(enabledNames)
	sort.Strings(disabledNames)

	fmt.Printf("%s+ config%s\n", ansiBold, ansiReset)
	for _, name := range enabledNames {
		fmt.Printf("+ %s\n", name)
	}
	for _, name := range disabledNames {
		fmt.Printf("%s- %s%s\n", ansiDim, name, ansiReset)
	}
}
