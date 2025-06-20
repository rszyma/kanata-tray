package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/getlantern/systray"
	"github.com/kirsle/configdir"
	"github.com/labstack/gommon/log"
	"github.com/spf13/pflag"

	app_pkg "github.com/rszyma/kanata-tray/app"
	"github.com/rszyma/kanata-tray/app/controlserver"
	"github.com/rszyma/kanata-tray/config"
	runner_pkg "github.com/rszyma/kanata-tray/runner"
	"github.com/rszyma/kanata-tray/status_icons"
)

var (
	buildVersion string = "not_set"
	buildHash    string = "not_set"
	buildDate    string = "not_set"
)

var (
	logLevel = pflag.Uint("log-level", uint(log.INFO), "Set log level for kanata-tray (1-debug, 2-info, 3-warn) (NOTE: doesn't affect kanata logging level).")
	version  = pflag.Bool("version", false, "Print the version and exit.")
	help     = pflag.Bool("help", false, "Print help and exit.")
)

const (
	configFileName = "kanata-tray.toml"
	logFilename    = "kanata_tray_lastrun.log"
)

const additional_help = `
Environment Variables:
      KANATA_TRAY_CONFIG_DIR - sets custom config directory
      KANATA_TRAY_LOG_DIR - sets custom log directory (default is same folder as the binary)

`

func main() {
	pflag.Parse()

	if *help {
		fmt.Println("kanata-tray: tray icon for kanata")
		fmt.Println()
		fmt.Println("Options:")
		pflag.PrintDefaults()
		fmt.Print(additional_help)
		os.Exit(1)
	}

	if *version {
		fmt.Println("kanata-tray")
		fmt.Printf("Version: %s\n", buildVersion)
		fmt.Printf("Commit Hash: %s\n", buildHash)
		fmt.Printf("Build Date: %s\n", buildDate)
		fmt.Printf("OS: %s\n", runtime.GOOS)
		fmt.Printf("Arch: %s\n", runtime.GOARCH)
		os.Exit(1)
	}

	err := mainImpl()
	if err != nil {
		log.Errorf("kanata-tray exited with an error: %v", err)
		os.Exit(1)
	}
}

// Try to resolve config dir, return first that matches. Order:
// 1. $KANATA_TRAY_CONFIG_DIR, if set
// 2. The same dir as executable, if it contains kanata-tray.toml
// 3. $XDG_CONFIG_HOME/kanata-tray (On Linux; Other OSes have their own local config path too)
func figureOutConfigDir() string {
	if v := os.Getenv("KANATA_TRAY_CONFIG_DIR"); v != "" {
		return v
	}
	exePath, err := exePath()
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

func exePath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("os.Executable: %v", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return "", fmt.Errorf("filepath.EvalSymlinks: %v", err)
	}
	return exePath, nil
}

func mainImpl() error {
	log.SetLevel(log.Lvl(*logLevel))

	if int(*logLevel) <= int(log.DEBUG) {
		log.SetHeader(`${time_rfc3339_nano} ${level} ${short_file}:${line}`)
	} else {
		log.SetHeader(`${time_rfc3339_nano} ${level}`)
	}

	var logDir string
	if v := os.Getenv("KANATA_TRAY_LOG_DIR"); v != "" {
		var err error
		logDir, err = filepath.EvalSymlinks(v)
		if err != nil {
			return fmt.Errorf("filepath.EvalSymlinks on KANATA_TRAY_LOG_DIR failed: %v", err)
		}
	} else {
		exePath, err := exePath()
		if err != nil {
			return fmt.Errorf("failed to determine kanata-tray executable path: %v", err)
		}
		exeDirPath := filepath.Dir(exePath)
		logDir = exeDirPath
	}

	logFilepath := filepath.Join(logDir, logFilename)
	logFile, err := os.Create(logFilepath)
	if err != nil {
		return fmt.Errorf("failed to create %s file: %v", logFilename, err)
	}
	// Check if stderr is available. Specifically it won't be, when running
	// Windows binary compiled with -H=windowsgui ldflag.
	_, err = os.Stderr.Stat()
	if err != nil {
		log.SetOutput(logFile)
	} else {
		// FIXME: logger lib disables color output for tee here
		// because it detects it's not directly a tty.
		log.SetOutput(io.MultiWriter(logFile, os.Stderr))
	}

	log.Infof("kanata-tray [version=%s, commit=%s, build_date=%s] starting", buildVersion, buildHash, buildDate)

	configFolder := figureOutConfigDir()
	log.Infof("kanata-tray config folder: %s", configFolder)

	// Create <configFolder> and <configFolder>/icons if needed.
	err = os.MkdirAll(filepath.Join(configFolder, "icons"), os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	err = os.Chdir(configFolder)
	if err != nil {
		return fmt.Errorf("failed to change directory: %v", err)
	}

	cfg, err := config.ReadConfigOrCreateIfNotExist(filepath.Join(configFolder, configFileName))
	if err != nil {
		return fmt.Errorf("ReadConfigOrCreateIfNotExist failed: %v", err)
	}
	menuTemplate, err := app_pkg.MenuTemplateFromConfig(*cfg)
	if err != nil {
		return fmt.Errorf("failed to create menu from config: %v", err)
	}
	layerIcons := app_pkg.ResolveIcons(configFolder, cfg)

	err = status_icons.CreateDefaultStatusIconsDirIfNotExists(configFolder)
	if err != nil {
		return fmt.Errorf("CreateDefaultStatusIconsDirIfNotExists: %v", err)
	}
	err = status_icons.LoadCustomStatusIcons(configFolder)
	if err != nil {
		return fmt.Errorf("LoadCustomStatusIcons: %v", err)
	}

	runner := runner_pkg.NewRunner()

	onReady := func() {
		app := app_pkg.NewSystrayApp(app_pkg.Opts{
			MenuTemplate:           menuTemplate,
			LayerIcons:             layerIcons,
			AllowConcurrentPresets: cfg.General.AllowConcurrentPresets,
			LogFilepath:            logFilepath,
		})

		go app.StartProcessingLoop(runner, configFolder)

		if cfg.General.ControlServerEnable {
			go func() {
				err = controlserver.RunControlServer(app, cfg.General.ControlServerPort)
				log.Errorf("app.RunControlServer failed: %v", err)
			}()
		}

		app.Autorun()
	}

	onExit := func() {
		log.Printf("Exiting")
	}

	systray.Run(onReady, onExit)
	return nil
}
