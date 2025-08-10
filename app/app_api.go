package app

import (
	"fmt"
)

func (a *SystrayApp) StopPreset(presetName string) error {
	i, err := a.indexFromPresetName(presetName)
	if err != nil {
		return fmt.Errorf("app.indexFromPresetName: %v", err)
	}
	a.stopPresetChan <- i
	return nil
}

func (a *SystrayApp) StopAllPresets() error {
	for i, status := range a.statuses {
		switch status {
		case statusRunning, statusStarting:
			a.stopPresetChan <- i
		}
	}
	return nil
}

func (a *SystrayApp) StartPreset(presetName string) error {
	i, err := a.indexFromPresetName(presetName)
	if err != nil {
		return fmt.Errorf("app.indexFromPresetName: %v", err)
	}
	a.startPresetCh <- i
	return nil
}

func (a *SystrayApp) StartAllDefaultPresets() error {
	a.Autorun()
	return nil
}

func (a *SystrayApp) TogglePreset(presetName string) (msg string, err error) {
	i, err := a.indexFromPresetName(presetName)
	if err != nil {
		return "", fmt.Errorf("app.indexFromPresetName: %v", err)
	}
	switch a.statuses[i] {
	case statusRunning, statusStarting:
		a.startPresetCh <- i
		return "started", nil
	case statusIdle, statusCrashed:
		a.stopPresetChan <- i
		return "stopped", nil
	}
	panic("unreachable")
}

// If any default preset is running, stop them.
// If 0 default presets are running, start all default presets.
func (a *SystrayApp) ToggleAllDefaultPresets() (msg string, err error) {
	stoppedPresetsCount := 0
	for i, preset := range a.presets {
		status := a.statuses[i]
		if preset.Preset.Autorun && (status == statusRunning || status == statusStarting) {
			a.stopPresetChan <- i
			stoppedPresetsCount += 1
		}
	}

	if stoppedPresetsCount > 0 {
		return fmt.Sprintf("stopped %d presets", stoppedPresetsCount), nil
	}

	a.Autorun()

	return "started all default presets", nil
}
