package main

import (
	"fmt"
	"path/filepath"

	"github.com/getlantern/systray"
	"github.com/kirsle/configdir"

	"github.com/rszyma/kanata-tray/app"
	"github.com/rszyma/kanata-tray/config"
	"github.com/rszyma/kanata-tray/icons"
	"github.com/rszyma/kanata-tray/runner"
)

func main() {
	err := mainImpl()
	if err != nil {
		panic(err)
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

	onReady := func() {
		app := app.NewSystrayApp(&menuTemplate, layerIcons)
		go app.StartProcessingLoop(&runner, cfg.General.LaunchOnStart, configFolder)
	}

	onExit := func() {
		fmt.Printf("Exiting")
	}

	systray.Run(onReady, onExit)
	return nil
}
