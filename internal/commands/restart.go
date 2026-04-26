package commands

import (
	"fmt"
	"os"
	"os/exec"

	daemon "github.com/deprecatedluar/luar-daemonator"
	"github.com/deprecatedluar/akeyshually/internal/common"
	"github.com/deprecatedluar/akeyshually/internal/config"
)

func restartIfRunning() {
	d := daemon.New("akeyshually")
	if d.IsRunning() {
		Restart()
	}
}

func notifyOverlayChange(message string) {
	if cfg, err := config.Load(); err == nil && cfg.Settings.NotifyOnOverlayChange {
		common.NotifyInfo("akeyshually", message)
	}
}

// Restart implements the restart command
// Stops the daemon and then starts it again (silent operation)
func Restart() {
	// Check if daemon is running via systemd
	hasService, err := common.HasSystemdService()
	if err == nil && hasService {
		// Let systemd handle the restart
		cmd := exec.Command("systemctl", "--user", "restart", "akeyshually")
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to restart systemd service: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Manual mode: stop then start
	d := daemon.New("akeyshually")
	if err := d.Restart(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to restart daemon: %v\n", err)
		os.Exit(1)
	}
}
