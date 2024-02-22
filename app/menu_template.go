package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kirsle/configdir"
	"github.com/rszyma/kanata-tray/config"
)

type MenuTemplate struct {
	Configurations []MenuEntry
	Executables    []MenuEntry
}

type MenuEntry struct {
	IsSelectable bool
	Title        string
	Tooltip      string
	Value        string
}

func MenuTemplateFromConfig(cfg config.Config) MenuTemplate {
	var result MenuTemplate

	if cfg.General.IncludeConfigsFromDefaultLocations {
		defaultKanataConfig := filepath.Join(configdir.LocalConfig("kanata"), "kanata.kbd")
		cfg.Configurations = append(cfg.Configurations, defaultKanataConfig)
	}
	for i := range cfg.Configurations {
		path := cfg.Configurations[i]
		expandedPath, err := resolveFilePath(path)
		entry := MenuEntry{
			IsSelectable: true,
			Title:        "Config: " + path,
			Tooltip:      "Switch to kanata config: " + path,
			Value:        expandedPath,
		}
		if err != nil {
			entry.IsSelectable = false
			entry.Title = "[ERR] " + entry.Title
			entry.Tooltip = fmt.Sprintf("error: %s", err)
			fmt.Printf("Error for kanata config file '%s': %v\n", path, err)
		}
		result.Configurations = append(result.Configurations, entry)
	}

	if cfg.General.IncludeExecutablesFromSystemPath {
		globalKanataPath, err := exec.LookPath("kanata")
		if err == nil {
			cfg.Executables = append(cfg.Executables, globalKanataPath)
		}
	}
	for i := range cfg.Executables {
		path := cfg.Executables[i]
		expandedPath, err := resolveFilePath(path)
		entry := MenuEntry{
			IsSelectable: true,
			Title:        "Exe: " + path,
			Tooltip:      "Switch to kanata executable: " + path,
			Value:        expandedPath,
		}
		if err != nil {
			entry.IsSelectable = false
			entry.Title = "[ERR] " + entry.Title
			entry.Tooltip = fmt.Sprintf("error: %s", err)
			fmt.Printf("Error for kanata exe '%s': %v\n", path, err)
		}
		result.Executables = append(result.Executables, entry)
	}

	return result
}

func resolveFilePath(path string) (string, error) {
	path, err := expandHomeDir(path)
	if err != nil {
		return "", fmt.Errorf("expandHomeDir: %v", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", fmt.Errorf("file doesn't exist")
	}
	return path, nil
}

func expandHomeDir(path string) (string, error) {
	if strings.Contains(path, "~") {
		dirname, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to determine user's home directory")
		}
		expandedPath := strings.Replace(path, "~", dirname, 1)
		return expandedPath, nil
	}
	return path, nil
}
