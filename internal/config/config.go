package config

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

//go:embed defaults/*
var embeddedConfigs embed.FS

type Settings struct {
	EnableMediaKeys bool   `toml:"enable_media_keys"`
	TriggerOn       string `toml:"trigger_on"` // "press" or "release"
	Shell           string `toml:"shell"`      // Optional: override $SHELL
	EnvFile         string `toml:"env_file"`   // Optional: source before commands
}

type Config struct {
	Settings  Settings          `toml:"settings"`
	Shortcuts map[string]string `toml:"shortcuts"`
	Commands  map[string]string `toml:"commands"`
}

func Load() (*Config, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Shortcuts: make(map[string]string),
		Commands:  make(map[string]string),
	}

	settingsPath := filepath.Join(configDir, "config.toml")
	if _, err := os.Stat(settingsPath); err == nil {
		if _, err := toml.DecodeFile(settingsPath, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config.toml: %w", err)
		}
	}

	shortcutsPath := filepath.Join(configDir, "shortcuts.toml")
	if _, err := os.Stat(shortcutsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("shortcuts.toml not found: %s", shortcutsPath)
	}

	var shortcuts Config
	if _, err := toml.DecodeFile(shortcutsPath, &shortcuts); err != nil {
		return nil, fmt.Errorf("failed to parse shortcuts.toml: %w", err)
	}

	for k, v := range shortcuts.Shortcuts {
		cfg.Shortcuts[k] = v
	}
	for k, v := range shortcuts.Commands {
		cfg.Commands[k] = v
	}

	if cfg.Settings.EnableMediaKeys {
		mediaKeysPath := filepath.Join(configDir, "media-keys.toml")
		if _, err := os.Stat(mediaKeysPath); err == nil {
			var mediaKeys Config
			if _, err := toml.DecodeFile(mediaKeysPath, &mediaKeys); err != nil {
				return nil, fmt.Errorf("failed to parse media-keys.toml: %w", err)
			}

			for k, v := range mediaKeys.Shortcuts {
				cfg.Shortcuts[k] = v
			}
			for k, v := range mediaKeys.Commands {
				cfg.Commands[k] = v
			}
		}
	}

	if len(cfg.Shortcuts) == 0 {
		return nil, fmt.Errorf("no shortcuts defined in config")
	}

	return cfg, nil
}

func (c *Config) ResolveCommand(ref string) string {
	if cmd, ok := c.Commands[ref]; ok {
		return cmd
	}
	return ref
}

func (c *Config) GetTriggerMode() string {
	mode := c.Settings.TriggerOn
	if mode == "" {
		return "press" // Default
	}
	if mode != "press" && mode != "release" {
		fmt.Fprintf(os.Stderr, "[WARN] Invalid trigger_on value '%s', using 'press'\n", mode)
		return "press"
	}
	return mode
}

func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".config", "akeyshually"), nil
}

func getConfigDir() (string, error) {
	return GetConfigDir()
}

func EnsureConfigExists() error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	files := []string{"config.toml", "shortcuts.toml", "media-keys.toml", "akeyshually.service"}
	for _, filename := range files {
		destPath := filepath.Join(configDir, filename)
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			data, err := embeddedConfigs.ReadFile("defaults/" + filename)
			if err != nil {
				return fmt.Errorf("failed to read embedded %s: %w", filename, err)
			}
			if err := os.WriteFile(destPath, data, 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", filename, err)
			}
		}
	}

	return nil
}
