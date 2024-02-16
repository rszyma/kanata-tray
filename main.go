package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/getlantern/systray"
	"github.com/kirsle/configdir"

	"github.com/rszyma/kanata-tray/app"
	"github.com/rszyma/kanata-tray/app/notifications"
	"github.com/rszyma/kanata-tray/config"
	"github.com/rszyma/kanata-tray/icons"
	"github.com/rszyma/kanata-tray/runner"
)

var (
	buildVersion string
	buildHash    string
	buildDate    string
)

var version = flag.Bool("version", false, "Print the version and exit")

func main() {
	flag.Parse()
	if *version {
		fmt.Printf("kanata-tray %s\n", buildVersion)
		fmt.Printf("Commit Hash: %s\n", buildHash)
		fmt.Printf("Build Date: %s\n", buildDate)
		fmt.Printf("OS: %s\n", runtime.GOOS)
		fmt.Printf("Arch: %s\n", runtime.GOARCH)
		os.Exit(1)
	}

	err := mainImpl()
	if err != nil {
		fmt.Printf("kanata-tray exited with an error: %v\n", err)
		os.Exit(1)
	}
}

func mainImpl() error {
	configFolder := configdir.LocalConfig("kanata-tray")
	fmt.Printf("kanata-tray config folder: %s\n", configFolder)

	// Create folder. No-op if exists.
	err := configdir.MakePath(configFolder)
	if err != nil {
		return fmt.Errorf("failed to create folder: %v", err)
	}

	// Make sure "icons" folder exists too.
	err = configdir.MakePath(filepath.Join(configFolder, "icons"))
	if err != nil {
		return fmt.Errorf("failed to create folder: %v", err)
	}

	configFile := filepath.Join(configFolder, "config.toml")

	cfg, err := config.ReadConfigOrCreateIfNotExist(configFile)
	if err != nil {
		return fmt.Errorf("loading config failed: %v", err)
	}

	menuTemplate := app.MenuTemplateFromConfig(*cfg)
	layerIcons := app.ResolveIcons(configFolder, cfg.LayerIcons, icons.Default)
	runner := runner.NewKanataRunner()

	var notifier notifications.INotifier = &notifications.Disabled{}
	if cfg.Overlay.Enable {
		n, err := notifications.InitGtkOverlay(
			cfg.Overlay.Width, cfg.Overlay.Height,
			cfg.Overlay.OffsetX, cfg.Overlay.OffsetY, cfg.Overlay.Duration,
		)
		if err != nil {
			fmt.Printf("Failed to initialize gtk notifications window. "+
				"Layer change notifications will be disabled. Error: %v\n", err)
		} else {
			notifier = n
		}
	}

	onReady := func() {
		app := app.NewSystrayApp(&menuTemplate, layerIcons)
		go app.StartProcessingLoop(&runner, notifier, cfg.General.LaunchOnStart, configFolder)
	}

	onExit := func() {
		fmt.Printf("Exiting")
	}

	systray.Run(onReady, onExit)
	return nil
}
