package app

import (
	"fmt"
)

func (app *SystrayApp) StopPreset(presetName string) error {
	i, err := app.indexFromPresetName(presetName)
	if err != nil {
		return fmt.Errorf("app.indexFromPresetName: %v", err)
	}
	app.stopPresetChan <- i
	return nil
}

func (app *SystrayApp) StopAllPresets() error {
	for i, status := range app.statuses {
		switch status {
		case statusRunning, statusStarting:
			app.stopPresetChan <- i
		}
	}
	return nil
}

func (app *SystrayApp) StartPreset(presetName string) error {
	i, err := app.indexFromPresetName(presetName)
	if err != nil {
		return fmt.Errorf("app.indexFromPresetName: %v", err)
	}
	app.startPresetCh <- i
	return nil
}

func (app *SystrayApp) StartAllDefaultPresets() error {
	app.Autorun()
	return nil
}

func (app *SystrayApp) TogglePreset(presetName string) (msg string, err error) {
	i, err := app.indexFromPresetName(presetName)
	if err != nil {
		return "", fmt.Errorf("app.indexFromPresetName: %v", err)
	}
	switch app.statuses[i] {
	case statusRunning, statusStarting:
		app.startPresetCh <- i
		return "started", nil
	case statusIdle, statusCrashed:
		app.stopPresetChan <- i
		return "stopped", nil
	}
	panic("unreachable")
}

// If any default preset is running, stop them.
// If 0 default presets are running, start all default presets.
func (app *SystrayApp) ToggleAllDefaultPresets() (msg string, err error) {
	stoppedPresetsCount := 0
	for i, preset := range app.presets {
		status := app.statuses[i]
		if preset.Preset.Autorun && (status == statusRunning || status == statusStarting) {
			app.stopPresetChan <- i
			stoppedPresetsCount += 1
		}
	}

	if stoppedPresetsCount > 0 {
		return fmt.Sprintf("stopped %d presets", stoppedPresetsCount), nil
	}

	app.Autorun()

	return "started all default presets", nil
}
