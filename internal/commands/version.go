package commands

import (
	"fmt"

	satellite "github.com/DeprecatedLuar/the-satellite/the-lib"
)

var updater = satellite.New("DeprecatedLuar", "akeyshually")

// Version displays version information and checks for updates
func Version() {
	version := satellite.GetVersion()
	fmt.Printf("akeyshually %s\n", version)

	if newVersion, err := updater.CheckForUpdate(version); err == nil && newVersion != "" {
		fmt.Printf("\n→ Update available: %s (run 'akeyshually update')\n", newVersion)
	}
}
