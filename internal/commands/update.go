package commands

import (
	"fmt"
	"os"

	satellite "github.com/DeprecatedLuar/the-satellite/the-lib"
)

// Update checks for and installs updates via the satellite library
func Update() {
	currentVersion := satellite.GetVersion()
	fmt.Printf("Current version: %s\n", currentVersion)
	fmt.Println("Checking for updates...")

	newVersion, err := updater.CheckForUpdate(currentVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to check for updates: %v\n", err)
		os.Exit(1)
	}

	if newVersion == "" {
		fmt.Println("✓ Already on latest version")
		return
	}

	fmt.Printf("Update available: %s → %s\n", currentVersion, newVersion)
	fmt.Println("Installing update...")

	if err := updater.RunInstaller(); err != nil {
		fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Update complete!")
}
