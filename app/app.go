package app

import (
	"fmt"

	"github.com/getlantern/systray"
	"github.com/skratchdot/open-golang/open"

	"github.com/rszyma/kanata-tray/icons"
	runner_pkg "github.com/rszyma/kanata-tray/runner"
)

type SysTrayApp struct {
	concurrentPresets bool

	presets  []PresetMenuEntry
	statuses []KanataStatus

	layerIcons LayerIcons
	tcpPort    int

	presetClickedCh chan int // the value sent in channel is an index of preset

	// Menu items

	mPresets []*systray.MenuItem
	mOptions *systray.MenuItem
	mQuit    *systray.MenuItem
}

func NewSystrayApp(menuTemplate []PresetMenuEntry, layerIcons LayerIcons, allowConcurrentPresets bool, tcpPort int) *SysTrayApp {

	t := &SysTrayApp{
		presets:           menuTemplate,
		layerIcons:        layerIcons,
		concurrentPresets: allowConcurrentPresets,
		tcpPort:           tcpPort,
	}

	for range menuTemplate {
		t.statuses = append(t.statuses, statusIdle)
	}

	systray.SetIcon(icons.Default)
	systray.SetTitle("kanata-tray")
	systray.SetTooltip("kanata-tray")

	for _, entry := range menuTemplate {
		menuItem := systray.AddMenuItem(entry.Title(statusIdle), entry.Tooltip())
		if !entry.IsSelectable {
			menuItem.Disable()
		}
		t.mPresets = append(t.mPresets, menuItem)
	}

	systray.AddSeparator()

	t.mOptions = systray.AddMenuItem("Options", "Reveals kanata-tray config file")
	t.mQuit = systray.AddMenuItem("Exit tray", "Closes kanata (if running) and exits the tray")

	t.presetClickedCh = multipleMenuItemsClickListener(t.mPresets)

	return t
}

func (t *SysTrayApp) runPreset(presetIndex int, runner *runner_pkg.Runner) {
	if t.concurrentPresets {
		fmt.Printf("Switching preset to '%s'\n", t.presets[presetIndex].PresetName)
	} else {
		fmt.Printf("Running preset '%s'\n", t.presets[presetIndex].PresetName)
	}
	t.statuses[presetIndex] = statusStarting
	t.mPresets[presetIndex].SetTitle(t.presets[presetIndex].Title(statusStarting))
	systray.SetIcon(icons.Default)

	kanataExecutable := t.presets[presetIndex].Preset.KanataExecutable
	kanataConfig := t.presets[presetIndex].Preset.KanataConfig
	err := runner.Run(t.presets[presetIndex].PresetName, kanataExecutable, kanataConfig, t.tcpPort)
	if err != nil {
		fmt.Printf("runner.Run failed with: %v\n", err)
		t.statuses[presetIndex] = statusCrashed
		t.mPresets[presetIndex].SetTitle(t.presets[presetIndex].Title(statusCrashed))
		return
	}
	t.statuses[presetIndex] = statusRunning
	t.mPresets[presetIndex].SetTitle(t.presets[presetIndex].Title(statusStarting))
}

func (app *SysTrayApp) StartProcessingLoop(runner *runner_pkg.Runner, allowConcurrentPresets bool, configFolder string) {
	systray.SetIcon(icons.Pause)
	for i, preset := range app.presets {
		if preset.Preset.Autorun {
			app.runPreset(i, runner)
			if allowConcurrentPresets {
				// Execute only the first preset if multi-exec is disabled.
				break
			}
		}
	}

	serverMessageCh := runner.ServerMessageCh()
	serverRetCh := runner.RetCh()

	for {
		select {
		case event := <-serverMessageCh:
			// fmt.Printf("Received an event from kanata: %v\n", pp.Sprint(event))
			if event.Item.LayerChange != nil {
				icon := app.layerIcons.IconForLayerName(event.PresetName, event.Item.LayerChange.NewLayer)
				if icon == nil {
					icon = icons.Default
				}
				systray.SetIcon(icon)
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
						fmt.Printf("Layer '%s' is mapped to an icon, but doesn't exist in the loaded kanata config\n", mappedLayerName)
					}
				}
			}
		case ret := <-serverRetCh:
			err := ret.Item
			i, err1 := app.indexFromPresetName(ret.PresetName)
			if err1 != nil {
				fmt.Printf("ERROR: Preset not found: %s\n", ret.PresetName)
				continue
			}
			if err != nil {
				fmt.Printf("Kanata process terminated with an error: %v\n", err)
				app.statuses[i] = statusCrashed
				app.mPresets[i].SetTitle(app.presets[i].Title(statusCrashed))
				systray.SetIcon(icons.Crash)
			} else {
				fmt.Println("Kanata process terminated successfully")
				app.statuses[i] = statusIdle
				app.mPresets[i].SetTitle(app.presets[i].Title(statusIdle))
				if app.isAnyPresetRunning() {
					systray.SetIcon(icons.Default)
				} else {
					// no running presets
					systray.SetIcon(icons.Pause)
				}
			}

			// TODO: is this even needed anymore? We don't even validate if that's process slot for the preset in `ret.PresetName`.
			<-runner.ProcessSlotCh // free 1 slot
		// case <-app.mStatus.ClickedCh:
		// 	switch app.runnerStatus {
		// 	case statusIdle:
		// 		// run kanata
		// 		app.runPreset(runner)
		// 	case statusRunning:
		// 		// stop kanata
		// 		err := runner.StopNonblocking()
		// 		if err != nil {
		// 			fmt.Printf("Failed to stop kanata process: %v", err)
		// 		} else {
		// 			app.runnerStatus = statusIdle
		// 			app.mStatus.SetTitle(statusIdle)
		// 			systray.SetIcon(icons.Pause)
		// 		}
		// 	case statusCrashed:
		// 		// restart kanata
		// 		fmt.Println("Restarting kanata")
		// 		app.runPreset(runner)
		// 	}
		// case <-app.mOpenKanataLogFile.ClickedCh:
		// 	logFile, err := runner.LogFile()
		// 	if err != nil {
		// 		fmt.Printf("Can't open log file: %v\n", err)
		// 	} else {
		// 		fmt.Printf("Opening log file '%s'\n", logFile)
		// 		open.Start(logFile)
		// 	}
		case i := <-app.presetClickedCh:
			app.runPreset(i, runner)
		case <-app.mOptions.ClickedCh:
			open.Start(configFolder)
		case <-app.mQuit.ClickedCh:
			fmt.Println("Exiting...")
			for _, preset := range app.presets {
				err := runner.StopNonblocking(preset.PresetName)
				if err != nil {
					fmt.Printf("failed to stop kanata process: %v", err)
				}
			}
			for _, preset := range app.presets {
				// When ProcessSlotCh becomes writable, it mean that the process
				// was successfully stopped.
				runner.ProcessSlotCh <- runner_pkg.ItemAndPresetName[struct{}]{
					Item:       struct{}{},
					PresetName: preset.PresetName,
				}
			}

			systray.Quit()
			return
		}
	}
}

func (t *SysTrayApp) indexFromPresetName(presetName string) (int, error) {
	for i, p := range t.presets {
		if p.PresetName == presetName {
			return i, nil
		}
	}
	return 0, fmt.Errorf("not found")
}

func (t *SysTrayApp) isAnyPresetRunning() bool {
	for _, status := range t.statuses {
		if status == statusRunning {
			return true
		}
	}
	return false
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
