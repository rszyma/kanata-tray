package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/getlantern/systray"
	"github.com/k0kubun/pp/v3"
	"github.com/kirsle/configdir"
	"github.com/pelletier/go-toml/v2"
	"github.com/skratchdot/open-golang/open"

	"github.com/rszyma/kanata-tray/icon"
)

func main() {
	err := mainImpl()
	if err != nil {
		panic(err)
	}
}

type Config struct {
	Configurations []string
	Executables    []string
	General        GeneralConfigOptions
}

type GeneralConfigOptions struct {
	IncludeExecutablesFromSystemPath   bool
	IncludeConfigsFromDefaultLocations bool
	LaunchOnStart                      bool
}

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

type KanataRunner struct {
	RetCh             chan error    // Returns the error returned by `cmd.Wait()`
	ProcessSlotCh     chan struct{} // prevent race condition when restarting kanata
	cmd               *exec.Cmd
	logFile           *os.File
	manualTermination bool
}

func NewKanataRunner() KanataRunner {
	return KanataRunner{
		RetCh: make(chan error),
		// 1 denotes max numer of running kanata processes allowed at a time
		ProcessSlotCh: make(chan struct{}, 1),

		cmd:               nil,
		logFile:           nil,
		manualTermination: false,
	}
}

// Terminates running kanata process, if there is one.
func (r *KanataRunner) Stop() error {
	if r.cmd != nil {
		if r.cmd.ProcessState != nil {
			// process was already killed from outside?
		} else {
			r.manualTermination = true
			fmt.Println("Killing the currently running kanata process...")
			err := r.cmd.Process.Kill()
			if err != nil {
				return fmt.Errorf("cmd.Process.Kill failed: %v", err)
			}
		}
	}
	r.cmd = nil
	return nil
}

func (r *KanataRunner) CleanupLogs() error {
	if r.cmd != nil && r.cmd.ProcessState == nil {
		return fmt.Errorf("tried to cleanup logs while kanata process is still running")
	}

	if r.logFile != nil {
		os.RemoveAll(r.logFile.Name())
		r.logFile.Close()
		r.logFile = nil
	}

	return nil
}

func (r *KanataRunner) Run(kanataExecutablePath string, kanataConfigPath string) error {
	err := r.Stop()
	if err != nil {
		return fmt.Errorf("failed to stop the previous process: %v", err)
	}

	err = r.CleanupLogs()
	if err != nil {
		// This is non-critical, we can probably continue operating normally.
		fmt.Printf("WARN: process logs cleanup failed: %v\n", err)
	}

	r.logFile, err = os.CreateTemp("", "kanata_lastrun_*.log")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	r.cmd = exec.Command(kanataExecutablePath, "-c", kanataConfigPath)

	go func() {
		// We're waiting for previous process to be marked as finished in processing loop.
		// We will know that happens when the process slot is writable.
		r.ProcessSlotCh <- struct{}{}

		fmt.Printf("Running command: %s\n", r.cmd.String())

		err = r.cmd.Start()
		if err != nil {
			fmt.Printf("Failed to start process: %v\n", err)
			return
		}

		fmt.Printf("Started kanata (pid=%d)\n", r.cmd.Process.Pid)

		err := r.cmd.Wait()
		if r.manualTermination {
			r.manualTermination = false
			r.RetCh <- nil
		} else {
			r.RetCh <- err
		}
	}()

	return nil
}

func mainImpl() error {
	configFolder := configdir.LocalConfig("kanata-tray")
	fmt.Printf("kanata-tray config folder: %s\n", configFolder)
	err := configdir.MakePath(configFolder) // No-op if exists, create if doesn't.
	if err != nil {
		return fmt.Errorf("failed to make path for config file: %v", err)
	}
	configFile := filepath.Join(configFolder, "config.toml")

	cfg, err := readConfigOrCreateIfNotExist(configFile)
	if err != nil {
		return fmt.Errorf("loading config failed: %v", err)
	}

	menuTemplate := menuTemplateFromConfig(*cfg)
	runner := NewKanataRunner()

	onReady := func() {
		app := NewSystrayApp(&menuTemplate)
		go app.StartProcessingLoop(&runner, cfg.General.LaunchOnStart, configFolder)
	}

	onExit := func() {
		fmt.Printf("Exiting")
	}

	systray.Run(onReady, onExit)
	return nil
}

func menuTemplateFromConfig(cfg Config) MenuTemplate {
	var result MenuTemplate

	if cfg.General.IncludeExecutablesFromSystemPath {
		defaultKanataConfig := path.Join(configdir.LocalConfig("kanata"), "kanata.kbd")
		cfg.Configurations = append(cfg.Configurations, defaultKanataConfig)
	}
	for i := range cfg.Configurations {
		path := cfg.Configurations[i]
		expandedPath, err := resolveConfigPath(path)
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
		expandedPath, err := resolveConfigPath(path)
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
			fmt.Printf("Error for kanata config file '%s': %v\n", path, err)
		}
		result.Executables = append(result.Executables, entry)
	}

	return result
}

func resolveConfigPath(path string) (string, error) {
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

// Returns a channel that sends an index of item that was clicked.
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

var (
	statusIdle    = "Kanata Status: Not Running (click to run)"
	statusRunning = "Kanata Status: Running (click to stop)"
	statusCrashed = "Kanata Status: Crashed (click to restart)"
)

type SysTrayApp struct {
	menuTemplate   *MenuTemplate
	selectedConfig int
	selectedExec   int
	cfgChangeCh    chan int
	exeChangeCh    chan int

	// Menu items

	mStatus      *systray.MenuItem
	runnerStatus string

	mOpenCrashLog *systray.MenuItem

	mConfigs []*systray.MenuItem
	mExecs   []*systray.MenuItem

	mOptions *systray.MenuItem
	mQuit    *systray.MenuItem
}

const selectedItemPrefix = "> "

func NewSystrayApp(menuTemplate *MenuTemplate) *SysTrayApp {
	t := &SysTrayApp{menuTemplate: menuTemplate, selectedConfig: -1, selectedExec: -1}

	systray.SetTemplateIcon(icon.Data, icon.Data)
	systray.SetTitle("kanata-tray")
	systray.SetTooltip("kanata-tray")

	t.mStatus = systray.AddMenuItem(statusIdle, statusIdle)
	t.runnerStatus = statusIdle

	t.mOpenCrashLog = systray.AddMenuItem("See Crash Log", "open location of the crash log")
	t.mOpenCrashLog.Hide()

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

// Switches config, but it doesn't run it.
func (t *SysTrayApp) switchConfigAndRun(index int, runner *KanataRunner) {
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

func (t *SysTrayApp) switchExeAndRun(index int, runner *KanataRunner) {
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

func (t *SysTrayApp) runWithSelectedOptions(runner *KanataRunner) {
	t.mOpenCrashLog.Hide()
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

func (t *SysTrayApp) StartProcessingLoop(runner *KanataRunner, runRightAway bool, configFolder string) {
	if runRightAway {
		t.runWithSelectedOptions(runner)
	}

	for {
		select {
		case err := <-runner.RetCh:
			if err != nil {
				fmt.Printf("Kanata process terminated with an error: %v\n", err)
				t.runnerStatus = statusCrashed
				t.mStatus.SetTitle(statusCrashed)
				t.mOpenCrashLog.Show()
				// todo: change tray icon to a one with warning sign or something.
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
				}
			case statusCrashed:
				// restart kanata
				fmt.Println("Restarting kanata")
				t.runWithSelectedOptions(runner)
			}
		case <-t.mOpenCrashLog.ClickedCh:
			fmt.Printf("Opening crash log file '%s'\n", runner.logFile.Name())
			open.Start(runner.logFile.Name())
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

func readConfigOrCreateIfNotExist(configFilePath string) (*Config, error) {
	var cfg *Config = &Config{
		General: GeneralConfigOptions{
			IncludeExecutablesFromSystemPath:   true,
			IncludeConfigsFromDefaultLocations: true,
			LaunchOnStart:                      true,
		},
	}
	// Does the file not exist?
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		err := toml.Unmarshal([]byte(defaultCfg), &cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to parse default config: %v", err)
		}
		fmt.Printf("Config file doesn't exist. Creating default config. Path: '%s'\n", configFilePath)
		os.WriteFile(configFilePath, []byte(defaultCfg), os.FileMode(0600))
	} else {
		// Load the existing file.
		fh, err := os.Open(configFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to open file '%s': %v", configFilePath, err)
		}
		defer fh.Close()
		decoder := toml.NewDecoder(fh)
		err = decoder.Decode(&cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to parse config file '%s': %v", configFilePath, err)
		}
	}

	pp.Println("%v", cfg)
	return cfg, nil
}

var defaultCfg = `
# See https://github.com/rszyma/kanata-tray for help with configuration.

configurations = [
    
]

executables = [
    
]

[general]
include_executables_from_system_path = true
include_configs_from_default_locations = true
launch_on_start = true
`
