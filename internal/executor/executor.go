package executor

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func Execute(command string) {
	cmd := exec.Command("sh", "-c", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to execute '%s': %v\n", command, err)
		return
	}
}
