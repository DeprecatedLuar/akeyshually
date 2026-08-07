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
	d := daemon.New(common.AppName)
	if d.IsRunning() {
		Restart()
	}
}

func notifyOverlayChange(message string) {
	if cfg, err := config.Load(); err == nil && cfg.Settings.NotifyOnOverlayChange {
		common.NotifyInfo(common.AppName, message)
	}
}

// Restart restarts the daemon via the systemd unit. There is no manual
// restart path: the daemon has no "start itself in the background" mode
// to return to, only foreground - a manually-run instance has to be
// stopped and re-run by whoever started it.
func Restart() {
	hasService, err := common.HasSystemdService()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to check systemd service: %v\n", err)
		os.Exit(1)
	}
	if !hasService {
		fmt.Fprintf(os.Stderr, "akeyshually isn't running under systemd - stop it (Ctrl-C or 'akeyshually stop') and run it again for changes to take effect\n")
		os.Exit(1)
	}

	cmd := exec.Command("systemctl", "--user", "restart", common.AppName)
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to restart systemd service: %v\n", err)
		os.Exit(1)
	}
}
