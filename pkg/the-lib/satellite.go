package satellite

import (
	"fmt"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
)

const SatelliteURL = "https://raw.githubusercontent.com/DeprecatedLuar/the-satellite/main/satellite.sh"

// Version is set via ldflags during build: -X github.com/DeprecatedLuar/the-satellite/the-lib.Version=vX.Y.Z
var Version = "dev"

// Updater manages update operations for a specific repository
type Updater struct {
	RepoUser string
	RepoName string
}

// New creates a new Updater for the specified GitHub repository
func New(repoUser, repoName string) *Updater {
	return &Updater{
		RepoUser: repoUser,
		RepoName: repoName,
	}
}

// CheckForUpdate queries satellite for a newer version
// Returns the new version string if available, empty string if up-to-date
func (u *Updater) CheckForUpdate(currentVersion string) (string, error) {
	curlCmd := fmt.Sprintf(
		"curl -sSL %s | bash -s -- check-update %s %s %s",
		SatelliteURL,
		currentVersion,
		u.RepoUser,
		u.RepoName,
	)

	cmd := exec.Command("bash", "-c", curlCmd)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to check for updates: %w", err)
	}

	newVersion := strings.TrimSpace(string(output))
	return newVersion, nil
}

// RunInstaller downloads and runs the project's install script via Satellite
func (u *Updater) RunInstaller() error {
	installURL := fmt.Sprintf(
		"https://raw.githubusercontent.com/%s/%s/main/install.sh",
		u.RepoUser,
		u.RepoName,
	)

	cmd := exec.Command("bash", "-c", fmt.Sprintf("curl -sSL %s | bash", installURL))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	return nil
}

// GetVersion returns the current version set via ldflags during build
// For dev builds, appends git commit hash if available
func GetVersion() string {
	if Version != "dev" {
		return Version
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}

	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" {
			if len(setting.Value) >= 7 {
				return "dev-" + setting.Value[:7]
			}
		}
	}

	return "dev"
}
