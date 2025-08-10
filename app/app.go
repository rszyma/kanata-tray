package app

import (
	"context"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/getlantern/systray"
	"github.com/k0kubun/pp/v3"
	"github.com/labstack/gommon/log"
	"github.com/skratchdot/open-golang/open"

	runner_pkg "github.com/rszyma/kanata-tray/runner"
	"github.com/rszyma/kanata-tray/status_icons"
)

type SystrayApp struct {
	logFilepath string

	concurrentPresets bool

	// Used when `concurrentPresets` is disabled.
	// Value -1 denotes that no config is scheduled to run.
	scheduledPresetIndex int

	presets                  []PresetMenuEntry
	statuses                 []KanataStatus
	presetCancelFuncs        []context.CancelFunc // cancel functions can be nil
	presetAutorestartLimiter []RestartLimiter
	presetLogFiles           []*os.File

	currentIconData []byte
	layerIcons      LayerIcons

	togglePresetCh   chan int // the value sent in channel is an index of preset
	startPresetCh    chan int // the value sent in channel is an index of preset
	stopPresetChan   chan int // the value sent in channel is an index of preset
	openPresetLogsCh chan int // the value sent in channel is an index of preset

	// Menu items

	mPresets        []*systray.MenuItem
	mPresetLogs     []*systray.MenuItem
	mPresetStatuses []*systray.MenuItem

	mOptions  *systray.MenuItem
	mShowLogs *systray.MenuItem
	mQuit     *systray.MenuItem
}

type Opts struct {
	MenuTemplate           []PresetMenuEntry
	LayerIcons             LayerIcons
	AllowConcurrentPresets bool
	LogFilepath            string
}

func NewSystrayApp(opts Opts) *SystrayApp {
	return &SystrayApp{
		logFilepath:          opts.LogFilepath,
		presets:              opts.MenuTemplate,
		scheduledPresetIndex: -1,
		layerIcons:           opts.LayerIcons,
		concurrentPresets:    opts.AllowConcurrentPresets,
	}
}

func (a *SystrayApp) InitSystray() *SystrayApp {
	if a == nil || a.scheduledPresetIndex != -1 {
		panic("InitSystray must be called on a freshly created instance")
	}

	systray.SetIcon(status_icons.Default)
	systray.SetTooltip("kanata-tray")

	for _, entry := range a.presets {
		menuItem := systray.AddMenuItem(entry.Title(statusIdle), entry.Tooltip())
		if !entry.IsSelectable {
			menuItem.Disable()
		}
		a.mPresets = append(a.mPresets, menuItem)

		statusItem := menuItem.AddSubMenuItem(string(statusIdle), "kanata status for this preset")
		a.mPresetStatuses = append(a.mPresetStatuses, statusItem)
		a.statuses = append(a.statuses, statusIdle)

		a.presetCancelFuncs = append(a.presetCancelFuncs, nil)

		openLogsItem := menuItem.AddSubMenuItem("Open kanata logs", "Open kanata log file")
		a.mPresetLogs = append(a.mPresetLogs, openLogsItem)

		a.presetAutorestartLimiter = append(a.presetAutorestartLimiter, RestartLimiter{})

		a.presetLogFiles = append(a.presetLogFiles, nil)
	}

	systray.AddSeparator()

	a.mOptions = systray.AddMenuItem("Configure", "Reveals kanata-tray config file")
	a.mShowLogs = systray.AddMenuItem("Open logs", "Reveals kanata-tray log file")
	a.mQuit = systray.AddMenuItem("Exit tray", "Closes kanata (if running) and exits the tray")

	a.togglePresetCh = multipleMenuItemsClickListener(a.mPresetStatuses)
	a.startPresetCh = make(chan int)
	a.stopPresetChan = make(chan int)
	a.openPresetLogsCh = multipleMenuItemsClickListener(a.mPresetLogs)

	return a
}

func (a *SystrayApp) runPreset(presetIndex int, runner *runner_pkg.Runner) {
	if !a.concurrentPresets && a.isAnyPresetRunning() {
		log.Infof("Switching preset to '%s'", a.presets[presetIndex].PresetName)
		for i := range a.presets {
			a.cancel(i)
			a.setStatus(i, statusIdle)
		}
		if a.scheduledPresetIndex != -1 {
			log.Warnf("the previously scheduled preset was not ran!")
		}
		a.scheduledPresetIndex = presetIndex
		// Preset has been scheduled to run, and will actutally be run when the previous one exits.
		return
	}

	log.Infof("Running preset '%s'", a.presets[presetIndex].PresetName)
	a.setStatus(presetIndex, statusStarting)

	a.presetLogFiles[presetIndex].Close()
	var err error
	a.presetLogFiles[presetIndex], err = os.CreateTemp("", "kanata_lastrun_*.log")
	if err != nil {
		log.Errorf("failed to create temp log file: %v", err)
		a.setStatus(presetIndex, statusCrashed)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	err = runner.Run(
		ctx,
		a.presets[presetIndex].PresetName,
		a.presets[presetIndex].Preset.KanataExecutable,
		a.presets[presetIndex].Preset.KanataConfig,
		a.presets[presetIndex].Preset.TcpPort,
		a.presets[presetIndex].Preset.Hooks,
		a.presets[presetIndex].Preset.ExtraArgs,
		a.presetLogFiles[presetIndex],
	)
	if err != nil {
		log.Errorf("runner.Run failed with: %v", err)
		a.setStatus(presetIndex, statusCrashed)
		cancel()
		return
	}
	a.cancel(presetIndex)
	a.setStatus(presetIndex, statusRunning)
	a.presetCancelFuncs[presetIndex] = cancel
}

func (a *SystrayApp) StartProcessingLoop(runner *runner_pkg.Runner, configFolder string) {
	a.setIcon(status_icons.Pause)

	serverMessageCh := runner.ServerMessageCh()
	retCh := runner.RetCh()

	for {
		select {
		case event := <-serverMessageCh:
			log.Debugf("Received an event from kanata (preset=%s): %v, ", event.PresetName, pp.Sprint(event.Item))

			// fmt.Printf("Received an event from kanata: %v\n", pp.Sprint(event))
			if event.Item.LayerChange != nil {
				icon := a.layerIcons.IconForLayerName(event.PresetName, event.Item.LayerChange.NewLayer)
				if icon == nil {
					icon = status_icons.Default
				}
				a.setIcon(icon)
			}
			if event.Item.LayerNames != nil {
				mappedLayers := a.layerIcons.MappedLayers(event.PresetName)
				for _, mappedLayerName := range mappedLayers {
					found := slices.Contains(event.Item.LayerNames.Names, mappedLayerName)
					if !found {
						log.Warnf("Layer '%s' is mapped to an icon, but doesn't exist in the loaded kanata config", mappedLayerName)
					}
				}
			}
			if event.Item.ConfigFileReload != nil {
				prevIcon := a.currentIconData
				a.setIcon(status_icons.LiveReload)
				time.Sleep(150 * time.Millisecond)
				a.setIcon(prevIcon)
			}
		case ret := <-retCh:
			runnerPipelineErr := ret.Item
			i, err := a.indexFromPresetName(ret.PresetName)
			if err != nil {
				log.Errorf("Preset not found: %s", ret.PresetName)
				continue
			}
			a.cancel(i)
			if runnerPipelineErr != nil {
				log.Errorf("Kanata runner terminated with an error: %v", runnerPipelineErr)
				a.setStatus(i, statusCrashed)
				a.setIcon(status_icons.Crash)

				if a.presets[i].Preset.AutorestartOnCrash {
					attemptCount, isAllowed := a.presetAutorestartLimiter[i].BeginAttempt()
					if isAllowed {
						log.Infof("[autorestart-on-crash] Restarting [%d/%d]", attemptCount, AutorestartLimit)
						a.runPreset(i, runner)
					} else {
						log.Warnf("[autorestart-on-crash] Restarts have been triggering too rapidly. Stopping futher attempts.")
						a.presetAutorestartLimiter[i].Clear()
					}
				}
			} else {
				log.Infof("Previous kanata process terminated successfully")
				a.setStatus(i, statusIdle)
				if a.isAnyPresetRunning() {
					a.setIcon(status_icons.Default)
				} else {
					a.setIcon(status_icons.Pause)
				}
			}
			if a.scheduledPresetIndex != -1 {
				a.runPreset(a.scheduledPresetIndex, runner)
				a.scheduledPresetIndex = -1
			}
		case i := <-a.togglePresetCh:
			switch a.statuses[i] {
			case statusIdle:
				// run kanata
				a.runPreset(i, runner)
			case statusRunning:
				// stop kanata
				a.cancel(i)
			case statusCrashed:
				// restart kanata (from crashed state)
				a.presetAutorestartLimiter[i].Clear()
				a.runPreset(i, runner)
			}
		case i := <-a.stopPresetChan:
			switch a.statuses[i] {
			case statusIdle:
				// already not running, do nothing
			case statusRunning:
				// stop kanata
				a.cancel(i)
			case statusCrashed:
				// already not running, do nothing
			}
		case i := <-a.startPresetCh:
			switch a.statuses[i] {
			case statusIdle:
				// run kanata
				a.runPreset(i, runner)
			case statusRunning:
				// alredy running, do nothing
			case statusCrashed:
				// restart kanata (from crashed state)
				a.presetAutorestartLimiter[i].Clear()
				a.runPreset(i, runner)
			}
		case i := <-a.openPresetLogsCh:
			presetName := a.presets[i].PresetName
			f := a.presetLogFiles[i]
			if f == nil {
				log.Warnf("No log file found for preset '%s'", presetName)
			} else {
				filename := f.Name()
				log.Debugf("Opening log file for preset '%s': '%s'", presetName, filename)
				open.Start(filename)
			}
		case <-a.mOptions.ClickedCh:
			open.Start(configFolder)
		case <-a.mShowLogs.ClickedCh:
			open.Start(a.logFilepath)
		case <-a.mQuit.ClickedCh:
			log.Info("Clicked \"Exit tray button\", exiting.")
			a.Cleanup()
			systray.Quit()
			return
		}
	}
}

// Run all presets with autorun=true. NOOP if they are running already.
func (a *SystrayApp) Autorun() {
	autoranOnePreset := false
	for i, preset := range a.presets {
		if preset.Preset.Autorun {
			if !a.concurrentPresets {
				if !autoranOnePreset {
					autoranOnePreset = true
				} else {
					log.Warnf("more than 1 preset has autorun enabled, but " +
						"can't run them all, because `allow_concurrent_presets` is not enabled.")
					break
				}
			}
			a.startPresetCh <- i
		}
	}
}

func (a *SystrayApp) Cleanup() {
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		anyIsRunning := false
		for i := range a.presets {
			switch a.statuses[i] {
			case statusRunning, statusStarting:
				anyIsRunning = true
				a.cancel(i)
			case statusIdle, statusCrashed: // noop
			}
		}
		if anyIsRunning {
			time.Sleep(10 * time.Millisecond)
		} else {
			return
		}
	}
	log.Warn("Cleanup deadline exceeded, releasing block")
}

func (a *SystrayApp) indexFromPresetName(presetName string) (int, error) {
	for i, p := range a.presets {
		if p.PresetName == presetName {
			return i, nil
		}
	}
	return 0, fmt.Errorf("preset with the specified name doesn't exist")
}

func (a *SystrayApp) isAnyPresetRunning() bool {
	return slices.Contains(a.statuses, statusRunning)
}

func (a *SystrayApp) setStatus(presetIndex int, status KanataStatus) {
	a.statuses[presetIndex] = status
	a.mPresetStatuses[presetIndex].SetTitle(string(status))
	a.mPresets[presetIndex].SetTitle(a.presets[presetIndex].Title(status))
}

// Cancels (stops) preset at given index, releasing immediately (non-blocking).
func (a *SystrayApp) cancel(presetIndex int) {
	cancel := a.presetCancelFuncs[presetIndex]
	if cancel != nil {
		cancel()
	}
	a.presetCancelFuncs[presetIndex] = nil
}

func (a *SystrayApp) setIcon(iconBytes []byte) {
	a.currentIconData = iconBytes
	systray.SetIcon(iconBytes)
}

// Returns a channel that sends an index of item that was clicked.
// TODO: pass ctx and cleanup on ctx cancel.
func multipleMenuItemsClickListener(menuItems []*systray.MenuItem) chan int {
	ch := make(chan int)
	for i := range menuItems {
		i := i
		go func() {
			for range menuItems[i].ClickedCh {
				ch <- i
			}
		}()
	}
	return ch
}
