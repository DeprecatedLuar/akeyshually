package commands

import (
	"fmt"
	"os"

	daemon "github.com/deprecatedluar/luar-daemonator"

	"github.com/deprecatedluar/akeyshually/internal/common"
)

// Stop stops the running daemon, however it was started - the library's
// flock-based IsRunning/Stop see it whether it's under systemd, under
// systemd-run, or just running in a terminal. If a supervisor relaunches
// it before Stop can confirm the stop, the library's own error says so.
func Stop() {
	d := daemon.New(common.AppName)

	if !d.IsRunning() {
		fmt.Fprintf(os.Stderr, "akeyshually, there is nothing running\n")
		os.Exit(1)
	}

	if err := d.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	fmt.Println("Stopped")
	common.NotifyInfo(common.AppName, "Daemon stopped")
}
