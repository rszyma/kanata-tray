package app

import (
	"context"
	"fmt"
	"time"

	"github.com/getlantern/systray"
	"github.com/skratchdot/open-golang/open"

	"github.com/rszyma/kanata-tray/icons"
	runner_pkg "github.com/rszyma/kanata-tray/runner"
)

type SysTrayApp struct {
	concurrentPresets bool

	presets           []PresetMenuEntry
	statuses          []KanataStatus
	presetCancelFuncs []context.CancelFunc // cancel functions can be nil

	layerIcons LayerIcons

	statusClickedCh   chan int // the value sent in channel is an index of preset
	openLogsClickedCh chan int // the value sent in channel is an index of preset

	// Menu items

	mPresets        []*systray.MenuItem
	mPresetLogs     []*systray.MenuItem
	mPresetStatuses []*systray.MenuItem

	mOptions *systray.MenuItem
	mQuit    *systray.MenuItem
}

func NewSystrayApp(menuTemplate []PresetMenuEntry, layerIcons LayerIcons, allowConcurrentPresets bool) *SysTrayApp {

	t := &SysTrayApp{
		presets:           menuTemplate,
		layerIcons:        layerIcons,
		concurrentPresets: allowConcurrentPresets,
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

func (t *SysTrayApp) runPreset(presetIndex int, runner *runner_pkg.Runner) {
	if t.concurrentPresets {
		fmt.Printf("Switching preset to '%s'\n", t.presets[presetIndex].PresetName)
	} else {
		fmt.Printf("Running preset '%s'\n", t.presets[presetIndex].PresetName)
	}
	t.setStatus(presetIndex, statusStarting)
	kanataExecutable := t.presets[presetIndex].Preset.KanataExecutable
	kanataConfig := t.presets[presetIndex].Preset.KanataConfig
	ctx, cancel := context.WithCancel(context.Background())
	err := runner.Run(ctx, t.presets[presetIndex].PresetName, kanataExecutable, kanataConfig, t.presets[presetIndex].Preset.TcpPort)
	if err != nil {
		fmt.Printf("runner.Run failed with: %v\n", err)
		t.setStatus(presetIndex, statusCrashed)
		cancel()
		return
	}
	t.setStatus(presetIndex, statusRunning)
	t.cancel(presetIndex)
	t.presetCancelFuncs[presetIndex] = cancel
}

func (app *SysTrayApp) StartProcessingLoop(runner *runner_pkg.Runner, configFolder string) {
	systray.SetIcon(icons.Pause)
	for i, preset := range app.presets {
		if preset.Preset.Autorun {
			app.runPreset(i, runner)
			if app.concurrentPresets {
				// Execute only the first preset if multi-exec is disabled.
				break
			}
		}
	}

	serverMessageCh := runner.ServerMessageCh()
	retCh := runner.RetCh()

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
		case ret := <-retCh:
			err := ret.Item
			i, err1 := app.indexFromPresetName(ret.PresetName)
			if err1 != nil {
				fmt.Printf("ERROR: Preset not found: %s\n", ret.PresetName)
				continue
			}
			if err != nil {
				fmt.Printf("Kanata process terminated with an error: %v\n", err)
				app.setStatus(i, statusCrashed)
				systray.SetIcon(icons.Crash)
			} else {
				fmt.Println("Kanata process terminated successfully")
				app.setStatus(i, statusIdle)
				if app.isAnyPresetRunning() {
					systray.SetIcon(icons.Default)
				} else {
					systray.SetIcon(icons.Pause)
				}
			}
			app.cancel(i)
		case i := <-app.statusClickedCh:
			switch app.statuses[i] {
			case statusIdle:
				// run kanata
				app.runPreset(i, runner)
			case statusRunning:
				// stop kanata
				app.cancel(i)
				app.setStatus(i, statusIdle)
			case statusCrashed:
				// restart kanata (from crashed state)
				fmt.Println("Restarting kanata")
				app.runPreset(i, runner)
			}
		case i := <-app.openLogsClickedCh:
			presetName := app.presets[i].PresetName
			logFile, err := runner.LogFile(presetName)
			if err != nil {
				fmt.Printf("Can't open log file for preset '%s': %v\n", presetName, err)
			} else {
				fmt.Printf("Opening log file for preset '%s': '%s'\n", presetName, logFile)
				open.Start(logFile)
			}
		case <-app.mOptions.ClickedCh:
			open.Start(configFolder)
		case <-app.mQuit.ClickedCh:
			fmt.Println("Exiting...")
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

func (t *SysTrayApp) setStatus(presetIndex int, status KanataStatus) {
	t.statuses[presetIndex] = status
	t.mPresetStatuses[presetIndex].SetTitle(string(status))
	t.mPresets[presetIndex].SetTitle(t.presets[presetIndex].Title(status))
}

// Cancels (stops) preset at given index.
func (t *SysTrayApp) cancel(presetIndex int) {
	cancel := t.presetCancelFuncs[presetIndex]
	if cancel != nil {
		cancel()
	}
	t.presetCancelFuncs[presetIndex] = nil
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
