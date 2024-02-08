package main

import (
	"fmt"
	"path/filepath"

	"github.com/getlantern/systray"
	"github.com/kirsle/configdir"

	"github.com/rszyma/kanata-tray/app"
	"github.com/rszyma/kanata-tray/config"
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
	err := configdir.MakePath(configFolder) // No-op if exists, create if doesn't.
	if err != nil {
		return fmt.Errorf("failed to make path for config file: %v", err)
	}
	configFile := filepath.Join(configFolder, "config.toml")

	cfg, err := config.ReadConfigOrCreateIfNotExist(configFile)
	if err != nil {
		return fmt.Errorf("loading config failed: %v", err)
	}

	menuTemplate := app.MenuTemplateFromConfig(*cfg)
	runner := runner.NewKanataRunner()

	onReady := func() {
		app := app.NewSystrayApp(&menuTemplate)
		go app.StartProcessingLoop(&runner, cfg.General.LaunchOnStart, configFolder)
	}

	onExit := func() {
		fmt.Printf("Exiting")
	}

	systray.Run(onReady, onExit)
	return nil
}
