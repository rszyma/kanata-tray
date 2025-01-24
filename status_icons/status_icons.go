package status_icons

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/labstack/gommon/log"
)

//go:embed default.ico
var Default []byte

//go:embed crash.ico
var Crash []byte

//go:embed pause.ico
var Pause []byte

//go:embed live-reload.ico
var LiveReload []byte

//////////////////////////////////////////////

var statusIconsDir string = "status_icons"

func LoadCustomStatusIcons(configDir string) error {
	prefixes := []string{"default", "crash", "pause", "live-reload"}
	for i, prefix := range prefixes {
		matches, err := filepath.Glob(filepath.Join(
			configDir, statusIconsDir, fmt.Sprintf("%s.*", prefix),
		))
		if err != nil {
			return fmt.Errorf("filepath.Glob: %v", err)
		}
		if len(matches) < 1 {
			continue
		}

		// Take only first match, ignore others. There should be only 1 matching
		// icon name anyway.
		match := matches[0]

		log.Infof("loading status icon: %s", match)
		fileContent, err := os.ReadFile(match)
		if err != nil {
			log.Errorf("LoadCustomStatusIcons: os.ReadFile: %v", err)
			continue
		}

		switch i {
		case 0:
			Default = fileContent
		case 1:
			Crash = fileContent
		case 2:
			Pause = fileContent
		case 3:
			LiveReload = fileContent
		default:
			panic("out of range")
		}
	}

	return nil
}

func CreateDefaultStatusIconsDirIfNotExists(configDir string) error {
	customIconsPath := filepath.Join(configDir, statusIconsDir)
	_, err := os.Stat(customIconsPath)

	if errors.Is(err, fs.ErrNotExist) {
		log.Infof("status_icons dir doesn't exist. Creating it and populating with the default icons.")
		err := os.MkdirAll(customIconsPath, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create folder: %v", err)
		}
		names := []string{"default.ico", "crash.ico", "pause.ico", "live-reload.ico"}
		data := [][]byte{Default, Crash, Pause, LiveReload}
		for i, name := range names {
			path := filepath.Join(customIconsPath, name)
			err := os.WriteFile(path, data[i], 0o644)
			if err != nil {
				return fmt.Errorf("writing file %s failed", path)
			}
		}
	} else if err != nil {
		return fmt.Errorf("error checking if %s dir exists", customIconsPath)
	}
	// already exists, do nothing.
	return nil
}
