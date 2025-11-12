package commands

import (
	"fmt"
	"os"
	"os/exec"
)

// Update checks for and installs updates via the satellite script
func Update() {
	fmt.Println("Checking for updates...")

	cmd := exec.Command("bash", "-c",
		`curl -sSL https://raw.githubusercontent.com/DeprecatedLuar/akeyshually/main/install.sh | bash -s -- update`)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
		os.Exit(1)
	}
}
