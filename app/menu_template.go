package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/rszyma/kanata-tray/config"
)

type PresetMenuEntry struct {
	IsSelectable bool
	Preset       config.Preset
	PresetName   string
}

type KanataStatus string

const (
	statusIdle     KanataStatus = "Kanata Status: Not Running (click to run)"
	statusStarting KanataStatus = "Kanata Status: Starting..."
	statusRunning  KanataStatus = "Kanata Status: Running (click to stop)"
	statusCrashed  KanataStatus = "Kanata Status: Crashed (click to restart)"
)

func (m *PresetMenuEntry) Title(status KanataStatus) string {
	switch status {
	case statusIdle:
		return "Preset: " + m.PresetName
	case statusRunning:
		return "> Preset: " + m.PresetName
	case statusCrashed:
		return "[ERR] Preset: " + m.PresetName
	}
	return "Preset: " + m.PresetName
}

func (m *PresetMenuEntry) Tooltip() string {
	return "Switch to preset: " + m.PresetName
}

func MenuTemplateFromConfig(cfg config.Config) ([]PresetMenuEntry, error) {
	presets := []PresetMenuEntry{}

	for presetName, preset := range cfg.Presets {
		// TODO: resolve path here? and put it in value?
		//
		// Resolve later could be better, since cfg can be also an empty value.
		// expandedPath, err := resolveFilePath(*p.CfgPath)
		//
		// We could also validate path ONLY if it's non empty.
		// Because if it's empty, kanata can still search default locations.
		//
		// But what about kanata executable path? should it be resolved later too?
		// Probably not. If we can catch an error here it would be good, because
		// we would be able to display it as an error in menu, whereas checking
		// when trying to run would only display an error in console. But it's very
		// likely that users want to hide console, that's why they use kanata-tray
		// in the first place.

		var err error
		preset.KanataConfig, err = expandHomeDir(preset.KanataConfig)
		if err != nil {
			return nil, err
		}
		preset.KanataExecutable, err = expandHomeDir(preset.KanataExecutable)
		if err != nil {
			return nil, err
		}

		entry := PresetMenuEntry{
			IsSelectable: true,
			Preset:       *preset,
			PresetName:   presetName,
		}

		presets = append(presets, entry)
	}

	return presets, nil
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
