package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/deprecatedluar/akeyshually/internal/config"
)

func Execute(command string, cfg *config.Config) {
	shell := cfg.Settings.Shell
	if shell == "" {
		shell = os.Getenv("SHELL")
		if shell == "" {
			shell = "sh"
		}
	}

	fullCommand := "cd &&" + command

	if cfg.Settings.EnvFile != "" {
		envFile := expandHome(cfg.Settings.EnvFile)
		fullCommand = fmt.Sprintf("source %s && %s", envFile, fullCommand)
	}

	cmd := exec.Command(shell, "-c", fullCommand)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to execute '%s': %v\n", command, err)
		return
	}
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
