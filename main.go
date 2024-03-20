package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/getlantern/systray"
	"github.com/kirsle/configdir"

	"github.com/rszyma/kanata-tray/app"
	"github.com/rszyma/kanata-tray/config"
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
	menuTemplate, err := app.MenuTemplateFromConfig(*cfg)
	if err != nil {
		return fmt.Errorf("failed to create menu from config: %v", err)
	}
	layerIcons := app.ResolveIcons(configFolder, cfg)

	// Actually we don't really use ctx right now to control kanata-tray termination
	// so normal contex without cancel will do.
	ctx := context.Background()
	runner := runner.NewRunner(ctx, cfg.General.AllowConcurrentPresets)

	onReady := func() {
		app := app.NewSystrayApp(menuTemplate, layerIcons, cfg.General.AllowConcurrentPresets)
		go app.StartProcessingLoop(runner, cfg.General.AllowConcurrentPresets, configFolder)
	}

	onExit := func() {
		fmt.Printf("Exiting")
	}

	systray.Run(onReady, onExit)
	return nil
}
