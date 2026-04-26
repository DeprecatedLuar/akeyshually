package commands

import (
	"fmt"
	"os"
	"os/exec"

	daemon "github.com/deprecatedluar/luar-daemonator"
	"github.com/deprecatedluar/akeyshually/internal"
)

// Stop implements the stop command
// Stops both systemd service AND manual daemon instances
func Stop() {
	stopped := false

	// Stop systemd service if active
	hasService, err := internal.HasSystemdService()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to check systemd service: %v\n", err)
	} else if hasService {
		cmd := exec.Command("systemctl", "--user", "stop", "akeyshually")
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to stop systemd service: %v\n", err)
		} else {
			fmt.Println("Stopped systemd service")
			internal.NotifyInfo("akeyshually", "Daemon stopped")
			stopped = true
		}
	}

	// Also check for manual daemon and stop it
	d := daemon.New("akeyshually")
	if d.IsRunning() {
		if err := d.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to stop daemon: %v\n", err)
		} else {
			fmt.Printf("Stopped manual daemon\n")
			internal.NotifyInfo("akeyshually", "Daemon stopped")
			stopped = true
		}
	}

	// If nothing was stopped, exit with error
	if !stopped {
		fmt.Fprintf(os.Stderr, "akeyshually, there is nothing running\n")
		os.Exit(1)
	}
}
