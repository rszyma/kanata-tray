package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/getlantern/systray"
	"github.com/kirsle/configdir"
	"github.com/labstack/gommon/log"

	"github.com/rszyma/kanata-tray/app"
	"github.com/rszyma/kanata-tray/config"
	"github.com/rszyma/kanata-tray/runner"
)

var (
	buildVersion string
	buildHash    string
	buildDate    string
)

var (
	logLevel = flag.Uint("log-level", uint(log.INFO), "Set log level for kanata-tray (1-debug, 2-info, 3-warn) (note: doesn't affect kanata logging level)")
	version  = flag.Bool("version", false, "Print the version and exit")
)

func main() {
	flag.Parse()
	if *version {
		log.Printf("kanata-tray %s", buildVersion)
		log.Printf("Commit Hash: %s", buildHash)
		log.Printf("Build Date: %s", buildDate)
		log.Printf("OS: %s", runtime.GOOS)
		log.Printf("Arch: %s", runtime.GOARCH)
		os.Exit(1)
	}

	log.SetLevel(log.Lvl(*logLevel))
	log.SetOutput(os.Stderr)
	if int(*logLevel) <= int(log.DEBUG) {
		log.SetHeader(`${time_rfc3339_nano} ${level} ${short_file}:${line}`)
	} else {
		log.SetHeader(`${time_rfc3339_nano} ${level}`)
	}

	err := mainImpl()
	if err != nil {
		log.Errorf("kanata-tray exited with an error: %v", err)
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
		log.Errorf("Failed to get kanata-tray executable path, can't check if kanata-tray.toml is there. Error: %v", err)
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

	log.Infof("kanata-tray config folder: %s", configFolder)

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
		log.Printf("Exiting")
	}

	systray.Run(onReady, onExit)
	return nil
}
