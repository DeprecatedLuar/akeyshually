package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

func GetEnabledStatePath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, ".enabled"), nil
}

func ReadEnabledState() ([]string, error) {
	enabledPath, err := GetEnabledStatePath()
	if err != nil {
		return nil, err
	}

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

	return files, scanner.Err()
}

func WriteEnabledState(files []string) error {
	enabledPath, err := GetEnabledStatePath()
	if err != nil {
		return err
	}

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

	tmpPath := enabledPath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		return err
	}

	return os.Rename(tmpPath, enabledPath)
}

func AddOverlay(filename string) error {
	files, err := ReadEnabledState()
	if err != nil {
		return err
	}

	for _, f := range files {
		if f == filename {
			return nil
		}
	}

	return WriteEnabledState(append(files, filename))
}

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

func ClearAllOverlays() error {
	return WriteEnabledState([]string{})
}
