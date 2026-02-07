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
	ExecuteTracked(command, cfg)
}

// ExecuteTracked starts a command and returns the exec.Cmd for process lifecycle management.
// Returns nil if the command fails to start.
func ExecuteTracked(command string, cfg *config.Config) *exec.Cmd {
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
		return nil
	}

	go cmd.Wait()

	return cmd
}

// StopProcess sends SIGTERM to a tracked process.
func StopProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	cmd.Process.Signal(syscall.SIGTERM)
	cmd.Wait()
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
