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

const configFileName = "kanata-tray.toml"

func figureOutConfigDir() (configFolder string) {
	if v := os.Getenv("KANATA_TRAY_CONFIG_DIR"); v != "" {
		return v
	}
	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("Failed to get kanata-tray executable path, can't check if kanata-tray.toml is there. Error: %v", err)
	} else {
		exeDir := filepath.Dir(exePath)
		if _, err := os.Stat(filepath.Join(exeDir, configFileName)); !os.IsNotExist(err) {
			return exeDir
		}
	}
	return configdir.LocalConfig("kanata-tray")
}

func mainImpl() error {
	configFolder := figureOutConfigDir()

	fmt.Printf("kanata-tray config folder: %s\n", configFolder)

	err := os.MkdirAll(filepath.Join(configFolder, "icons"), os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create folder: %v", err)
	}

	cfg, err := config.ReadConfigOrCreateIfNotExist(filepath.Join(configFolder, configFileName))
	if err != nil {
		return fmt.Errorf("loading config failed: %v", err)
	}
	menuTemplate, err := app.MenuTemplateFromConfig(*cfg)
	if err != nil {
		return fmt.Errorf("failed to create menu from config: %v", err)
	}
	layerIcons := app.ResolveIcons(configFolder, cfg)

	runner := runner.NewRunner()

	onReady := func() {
		app := app.NewSystrayApp(menuTemplate, layerIcons, cfg.General.AllowConcurrentPresets)
		go app.StartProcessingLoop(runner, configFolder)
	}

	onExit := func() {
		fmt.Printf("Exiting")
	}

	systray.Run(onReady, onExit)
	return nil
}
