package app

import (
	"fmt"

	"github.com/getlantern/systray"
	"github.com/skratchdot/open-golang/open"

	"github.com/rszyma/kanata-tray/app/notifications"
	"github.com/rszyma/kanata-tray/icons"
	"github.com/rszyma/kanata-tray/runner"
)

const (
	statusIdle    = "Kanata Status: Not Running (click to run)"
	statusRunning = "Kanata Status: Running (click to stop)"
	statusCrashed = "Kanata Status: Crashed (click to restart)"
)

const selectedItemPrefix = "> "

type SysTrayApp struct {
	menuTemplate   *MenuTemplate
	layerIcons     LayerIcons
	selectedConfig int
	selectedExec   int

	cfgChangeCh chan int
	exeChangeCh chan int

	// Menu items

	mStatus      *systray.MenuItem
	runnerStatus string

	mOpenKanataLogFile *systray.MenuItem

	mConfigs []*systray.MenuItem
	mExecs   []*systray.MenuItem

	mOptions *systray.MenuItem
	mQuit    *systray.MenuItem
}

func NewSystrayApp(menuTemplate *MenuTemplate, layerIcons LayerIcons) *SysTrayApp {
	t := &SysTrayApp{
		menuTemplate:   menuTemplate,
		layerIcons:     layerIcons,
		selectedConfig: -1,
		selectedExec:   -1,
	}

	systray.SetIcon(icons.Default)
	systray.SetTitle("kanata-tray")
	systray.SetTooltip("kanata-tray")

	t.mStatus = systray.AddMenuItem(statusIdle, statusIdle)
	t.runnerStatus = statusIdle

	t.mOpenKanataLogFile = systray.AddMenuItem("See Crash Log", "open location of the kanata log")
	t.mOpenKanataLogFile.Hide()

	systray.AddSeparator()

	for i, entry := range menuTemplate.Configurations {
		menuItem := systray.AddMenuItem(entry.Title, entry.Tooltip)
		t.mConfigs = append(t.mConfigs, menuItem)
		if entry.IsSelectable {
			if t.selectedConfig == -1 {
				menuItem.SetTitle(selectedItemPrefix + entry.Title)
				t.selectedConfig = i
			}
		} else {
			menuItem.Disable()
		}
	}

	systray.AddSeparator()

	for i, entry := range menuTemplate.Executables {
		menuItem := systray.AddMenuItem(entry.Title, entry.Tooltip)
		t.mExecs = append(t.mExecs, menuItem)
		if entry.IsSelectable {
			if t.selectedExec == -1 {
				menuItem.SetTitle(selectedItemPrefix + entry.Title)
				t.selectedExec = i
			}
		} else {
			menuItem.Disable()
		}
	}

	systray.AddSeparator()

	t.mOptions = systray.AddMenuItem("Options", "Reveals kanata-tray config file")
	t.mQuit = systray.AddMenuItem("Exit tray", "Closes kanata (if running) and exits the tray")

	t.cfgChangeCh = multipleMenuItemsClickListener(t.mConfigs)
	t.exeChangeCh = multipleMenuItemsClickListener(t.mExecs)

	return t
}

func (t *SysTrayApp) switchConfigAndRun(index int, runner *runner.KanataRunner) {
	oldIndex := t.selectedConfig
	t.selectedConfig = index
	oldEntry := t.menuTemplate.Configurations[oldIndex]
	newEntry := t.menuTemplate.Configurations[index]
	fmt.Printf("Switching kanata config to '%s'\n", newEntry.Value)

	// Remove selectedItemPrefix from previously selected item's title.
	t.mConfigs[oldIndex].SetTitle(oldEntry.Title)

	t.mConfigs[index].SetTitle(selectedItemPrefix + newEntry.Title)

	t.runWithSelectedOptions(runner)
}

func (t *SysTrayApp) switchExeAndRun(index int, runner *runner.KanataRunner) {
	oldIndex := t.selectedExec
	t.selectedExec = index
	oldEntry := t.menuTemplate.Executables[oldIndex]
	newEntry := t.menuTemplate.Executables[index]
	fmt.Printf("Switching kanata executable to '%s'\n", newEntry.Value)

	// Remove selectedItemPrefix from previously selected item's title.
	t.mExecs[oldIndex].SetTitle(oldEntry.Title)

	t.mExecs[index].SetTitle(selectedItemPrefix + newEntry.Title)

	t.runWithSelectedOptions(runner)
}

func (t *SysTrayApp) runWithSelectedOptions(runner *runner.KanataRunner) {
	t.mOpenKanataLogFile.Hide()

	if t.selectedExec == -1 {
		fmt.Println("failed to run: no kanata executables available")
		return
	}

	if t.selectedConfig == -1 {
		fmt.Println("failed to run: no kanata configs available")
		return
	}

	systray.SetIcon(icons.Default)

	execPath := t.menuTemplate.Executables[t.selectedExec].Value
	configPath := t.menuTemplate.Configurations[t.selectedConfig].Value
	err := runner.Run(execPath, configPath)
	if err != nil {
		fmt.Printf("runner.Run failed with: %v\n", err)
		t.runnerStatus = statusCrashed
		t.mStatus.SetTitle(statusCrashed)
	} else {
		t.runnerStatus = statusRunning
		t.mStatus.SetTitle(statusRunning)
	}
}

func (t *SysTrayApp) StartProcessingLoop(runner *runner.KanataRunner, notifier notifications.INotifier, runRightAway bool, configFolder string) {
	if runRightAway {
		t.runWithSelectedOptions(runner)
	} else {
		systray.SetIcon(icons.Pause)
	}

	serverMessageCh := runner.ServerMessageCh()

	for {
		select {
		case event := <-serverMessageCh:
			// fmt.Println("Received an event from kanata!")
			if event.LayerChange != nil {
				newLayer := event.LayerChange.NewLayer
				icon := t.layerIcons.IconForLayerName(newLayer)
				systray.SetIcon(icon)
				notifier.LayerChange(newLayer)
			}
		case err := <-runner.RetCh:
			if err != nil {
				fmt.Printf("Kanata process terminated with an error: %v\n", err)
				t.runnerStatus = statusCrashed
				t.mStatus.SetTitle(statusCrashed)
				t.mOpenKanataLogFile.Show()
				systray.SetIcon(icons.Crash)
			} else {
				fmt.Println("Kanata process terminated successfully")
			}
			<-runner.ProcessSlotCh // free 1 slot
		case <-t.mStatus.ClickedCh:
			switch t.runnerStatus {
			case statusIdle:
				// run kanata
				t.runWithSelectedOptions(runner)
			case statusRunning:
				// stop kanata
				err := runner.Stop()
				if err != nil {
					fmt.Printf("Failed to stop kanata process: %v", err)
				} else {
					t.runnerStatus = statusIdle
					t.mStatus.SetTitle(statusIdle)
					systray.SetIcon(icons.Pause)
				}
			case statusCrashed:
				// restart kanata
				fmt.Println("Restarting kanata")
				t.runWithSelectedOptions(runner)
			}
		case <-t.mOpenKanataLogFile.ClickedCh:
			logFile, err := runner.LogFile()
			if err != nil {
				fmt.Printf("Can't open log file: %v\n", err)
			} else {
				fmt.Printf("Opening log file '%s'\n", logFile)
				open.Start(logFile)
			}
		case i := <-t.cfgChangeCh:
			t.switchConfigAndRun(i, runner)
		case i := <-t.exeChangeCh:
			t.switchExeAndRun(i, runner)
		case <-t.mOptions.ClickedCh:
			open.Start(configFolder)
		case <-t.mQuit.ClickedCh:
			fmt.Println("Exiting...")
			err := runner.Stop()
			if err != nil {
				fmt.Printf("failed to stop kanata process: %v", err)
			}
			err = runner.CleanupLogs()
			if err != nil {
				fmt.Printf("failed to cleanup logs: %v", err)
			}
			systray.Quit()
			return
		}
	}
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
