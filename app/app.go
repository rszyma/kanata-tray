package app

import (
	"context"
	"fmt"
	"time"

	"github.com/getlantern/systray"
	"github.com/k0kubun/pp/v3"
	"github.com/labstack/gommon/log"
	"github.com/skratchdot/open-golang/open"

	"github.com/rszyma/kanata-tray/icons"
	runner_pkg "github.com/rszyma/kanata-tray/runner"
)

type SystrayApp struct {
	concurrentPresets bool

	// Used when `concurrentPresets` is disabled.
	// Value -1 denotes that no config is scheduled to run.
	scheduledPresetIndex int

	presets           []PresetMenuEntry
	statuses          []KanataStatus
	presetCancelFuncs []context.CancelFunc // cancel functions can be nil

	currentIconData []byte
	layerIcons      LayerIcons

	statusClickedCh   chan int // the value sent in channel is an index of preset
	openLogsClickedCh chan int // the value sent in channel is an index of preset

	// Menu items

	mPresets        []*systray.MenuItem
	mPresetLogs     []*systray.MenuItem
	mPresetStatuses []*systray.MenuItem

	mOptions *systray.MenuItem
	mQuit    *systray.MenuItem
}

func NewSystrayApp(menuTemplate []PresetMenuEntry, layerIcons LayerIcons, allowConcurrentPresets bool) *SystrayApp {

	systray.SetIcon(icons.Default)
	systray.SetTooltip("kanata-tray")

	t := &SystrayApp{
		presets:              menuTemplate,
		scheduledPresetIndex: -1,
		layerIcons:           layerIcons,
		concurrentPresets:    allowConcurrentPresets,
	}

	for _, entry := range menuTemplate {
		menuItem := systray.AddMenuItem(entry.Title(statusIdle), entry.Tooltip())
		if !entry.IsSelectable {
			menuItem.Disable()
		}
		t.mPresets = append(t.mPresets, menuItem)

		statusItem := menuItem.AddSubMenuItem(string(statusIdle), "kanata status for this preset")
		t.mPresetStatuses = append(t.mPresetStatuses, statusItem)
		t.statuses = append(t.statuses, statusIdle)

		t.presetCancelFuncs = append(t.presetCancelFuncs, nil)

		openLogsItem := menuItem.AddSubMenuItem("Open Logs", "Open Logs")
		// openLogsItem.Disable()
		t.mPresetLogs = append(t.mPresetLogs, openLogsItem)
	}

	systray.AddSeparator()

	t.mOptions = systray.AddMenuItem("Configure", "Reveals kanata-tray config file")
	t.mQuit = systray.AddMenuItem("Exit tray", "Closes kanata (if running) and exits the tray")

	t.statusClickedCh = multipleMenuItemsClickListener(t.mPresetStatuses)
	t.openLogsClickedCh = multipleMenuItemsClickListener(t.mPresetLogs)

	return t
}

func (t *SystrayApp) runPreset(presetIndex int, runner *runner_pkg.Runner) {
	if !t.concurrentPresets && t.isAnyPresetRunning() {
		log.Infof("Switching preset to '%s'", t.presets[presetIndex].PresetName)
		for i := range t.presets {
			t.cancel(i)
			t.setStatus(i, statusIdle)
		}
		if t.scheduledPresetIndex != -1 {
			log.Warnf("the previously scheduled preset was not ran!")
		}
		t.scheduledPresetIndex = presetIndex
		// Preset has been scheduled to run, and will actutally be run when the previous one exits.
		return
	}

	log.Infof("Running preset '%s'", t.presets[presetIndex].PresetName)

	t.setStatus(presetIndex, statusStarting)
	kanataExecutable := t.presets[presetIndex].Preset.KanataExecutable
	kanataConfig := t.presets[presetIndex].Preset.KanataConfig
	ctx, cancel := context.WithCancel(context.Background())
	err := runner.Run(ctx, t.presets[presetIndex].PresetName, kanataExecutable, kanataConfig, t.presets[presetIndex].Preset.TcpPort, t.presets[presetIndex].Preset.Hooks)
	if err != nil {
		log.Errorf("runner.Run failed with: %v", err)
		t.setStatus(presetIndex, statusCrashed)
		cancel()
		return
	}
	t.cancel(presetIndex)
	t.setStatus(presetIndex, statusRunning)
	t.presetCancelFuncs[presetIndex] = cancel
}

func (app *SystrayApp) StartProcessingLoop(runner *runner_pkg.Runner, configFolder string) {
	app.setIcon(icons.Pause)

	// handle autoruns
	autoranOnePreset := false
	for i, preset := range app.presets {
		if preset.Preset.Autorun {
			if !app.concurrentPresets {
				if !autoranOnePreset {
					autoranOnePreset = true
				} else {
					log.Warnf("more than 1 preset has autorun enabled, but " +
						"can't run them all, because `allow_concurrent_presets` is not enabled.")
					break
				}
			}
			app.runPreset(i, runner)
		}
	}

	serverMessageCh := runner.ServerMessageCh()
	retCh := runner.RetCh()

	for {
		select {
		case event := <-serverMessageCh:
			log.Debugf("Received an event from kanata (preset=%s): %v, ", event.PresetName, pp.Sprint(event.Item))

			// fmt.Printf("Received an event from kanata: %v\n", pp.Sprint(event))
			if event.Item.LayerChange != nil {
				icon := app.layerIcons.IconForLayerName(event.PresetName, event.Item.LayerChange.NewLayer)
				if icon == nil {
					icon = icons.Default
				}
				app.setIcon(icon)
			}
			if event.Item.LayerNames != nil {
				mappedLayers := app.layerIcons.MappedLayers(event.PresetName)
				for _, mappedLayerName := range mappedLayers {
					found := false
					for _, kanataLayerName := range event.Item.LayerNames.Names {
						if mappedLayerName == kanataLayerName {
							found = true
							break
						}
					}
					if !found {
						log.Warnf("Layer '%s' is mapped to an icon, but doesn't exist in the loaded kanata config", mappedLayerName)
					}
				}
			}
			if event.Item.ConfigFileReload != nil {
				prevIcon := app.currentIconData
				app.setIcon(icons.LiveReload)
				time.Sleep(150 * time.Millisecond)
				app.setIcon(prevIcon)
			}
		case ret := <-retCh:
			runnerPipelineErr := ret.Item
			i, err := app.indexFromPresetName(ret.PresetName)
			if err != nil {
				log.Errorf("Preset not found: %s", ret.PresetName)
				continue
			}
			app.cancel(i)
			if runnerPipelineErr != nil {
				log.Errorf("Kanata runner terminated with an error: %v", runnerPipelineErr)
				app.setStatus(i, statusCrashed)
				app.setIcon(icons.Crash)
			} else {
				log.Infof("Previous kanata process terminated successfully")
				app.setStatus(i, statusIdle)
				if app.isAnyPresetRunning() {
					app.setIcon(icons.Default)
				} else {
					app.setIcon(icons.Pause)
				}
			}
			if app.scheduledPresetIndex != -1 {
				app.runPreset(app.scheduledPresetIndex, runner)
				app.scheduledPresetIndex = -1
			}
		case i := <-app.statusClickedCh:
			switch app.statuses[i] {
			case statusIdle:
				// run kanata
				app.runPreset(i, runner)
			case statusRunning:
				// stop kanata
				app.cancel(i)
			case statusCrashed:
				// restart kanata (from crashed state)
				app.runPreset(i, runner)
			}
		case i := <-app.openLogsClickedCh:
			presetName := app.presets[i].PresetName
			logFile, err := runner.LogFile(presetName)
			if err != nil {
				log.Warnf("Can't open log file for preset '%s': %v", presetName, err)
			} else {
				log.Debugf("Opening log file for preset '%s': '%s'", presetName, logFile)
				open.Start(logFile)
			}
		case <-app.mOptions.ClickedCh:
			open.Start(configFolder)
		case <-app.mQuit.ClickedCh:
			log.Print("Exiting...")
			for _, cancel := range app.presetCancelFuncs {
				if cancel != nil {
					cancel()
				}
			}
			time.Sleep(1 * time.Second)
			// TODO: ensure all kanata processes stopped?
			systray.Quit()
			return
		}
	}
}

func (t *SystrayApp) indexFromPresetName(presetName string) (int, error) {
	for i, p := range t.presets {
		if p.PresetName == presetName {
			return i, nil
		}
	}
	return 0, fmt.Errorf("not found")
}

func (t *SystrayApp) isAnyPresetRunning() bool {
	for _, status := range t.statuses {
		if status == statusRunning {
			return true
		}
	}
	return false
}

func (t *SystrayApp) setStatus(presetIndex int, status KanataStatus) {
	t.statuses[presetIndex] = status
	t.mPresetStatuses[presetIndex].SetTitle(string(status))
	t.mPresets[presetIndex].SetTitle(t.presets[presetIndex].Title(status))
}

// Cancels (stops) preset at given index.
func (t *SystrayApp) cancel(presetIndex int) {
	cancel := t.presetCancelFuncs[presetIndex]
	if cancel != nil {
		cancel()
	}
	t.presetCancelFuncs[presetIndex] = nil
}

func (t *SystrayApp) setIcon(iconBytes []byte) {
	t.currentIconData = iconBytes
	systray.SetIcon(iconBytes)
}

// Returns a channel that sends an index of item that was clicked.
// TODO: pass ctx and cleanup on ctx cancel.
func multipleMenuItemsClickListener(menuItems []*systray.MenuItem) chan int {
	ch := make(chan int)
	for i := range menuItems {
		var i = i
		go func() {
			for range menuItems[i].ClickedCh {
				ch <- i
			}
		}()
	}
	return ch
}
