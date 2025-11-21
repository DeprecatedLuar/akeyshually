package internal

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/deprecatedluar/akeyshually/internal/config"
)

// GetEnabledStatePath returns the path to the .enabled state file
func GetEnabledStatePath() (string, error) {
	configDir, err := config.GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, ".enabled"), nil
}

// ReadEnabledState reads the list of enabled overlay filenames
func ReadEnabledState() ([]string, error) {
	enabledPath, err := GetEnabledStatePath()
	if err != nil {
		return nil, err
	}

	// If file doesn't exist, return empty list
	if _, err := os.Stat(enabledPath); os.IsNotExist(err) {
		return []string{}, nil
	}

	file, err := os.Open(enabledPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var files []string
	seen := make(map[string]bool)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !seen[line] {
			files = append(files, line)
			seen[line] = true
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return files, nil
}

// WriteEnabledState atomically writes the list of enabled overlays
func WriteEnabledState(files []string) error {
	enabledPath, err := GetEnabledStatePath()
	if err != nil {
		return err
	}

	tmpPath := enabledPath + ".tmp"

	// Deduplicate files
	seen := make(map[string]bool)
	var unique []string
	for _, f := range files {
		if !seen[f] && f != "" {
			unique = append(unique, f)
			seen[f] = true
		}
	}

	content := strings.Join(unique, "\n")
	if content != "" {
		content += "\n"
	}

	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		return err
	}

	return os.Rename(tmpPath, enabledPath)
}

// AddOverlay adds an overlay to the enabled list
func AddOverlay(filename string) error {
	files, err := ReadEnabledState()
	if err != nil {
		return err
	}

	// Check if already enabled
	for _, f := range files {
		if f == filename {
			return nil
		}
	}

	files = append(files, filename)
	return WriteEnabledState(files)
}

// RemoveOverlay removes an overlay from the enabled list
func RemoveOverlay(filename string) error {
	files, err := ReadEnabledState()
	if err != nil {
		return err
	}

	var filtered []string
	for _, f := range files {
		if f != filename {
			filtered = append(filtered, f)
		}
	}

	return WriteEnabledState(filtered)
}

// ClearAllOverlays removes all enabled overlays
func ClearAllOverlays() error {
	return WriteEnabledState([]string{})
}
