package status_icons

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/labstack/gommon/log"
)

// Icon is a tray icon along with the way it should be rendered.
type Icon struct {
	Data []byte
	// IsTemplate marks the icon as a macOS template image: the system ignores
	// its colors and tints the shape from the alpha channel to match a light
	// or dark menu bar. Ignored on other platforms.
	IsTemplate bool
}

//go:embed default.ico
var defaultIconData []byte

//go:embed crash.ico
var crashIconData []byte

//go:embed pause.ico
var pauseIconData []byte

//go:embed live-reload.ico
var liveReloadIconData []byte

var (
	Default    = Icon{Data: defaultIconData}
	Crash      = Icon{Data: crashIconData}
	Pause      = Icon{Data: pauseIconData}
	LiveReload = Icon{Data: liveReloadIconData}
)

//////////////////////////////////////////////

var statusIconsDir string = "status_icons"

// Apple's convention for template images: an icon file whose name ends with
// "Template" (before the extension) is tinted by macOS to match the menu bar.
const templateSuffix = "Template"

// IsTemplateFilename reports whether an icon path follows the macOS template
// image naming convention, e.g. "mouseTemplate.png".
// https://developer.apple.com/documentation/foundation/bundle/image(forresource:)
func IsTemplateFilename(path string) bool {
	return strings.HasSuffix(filenameWithoutExt(path), templateSuffix)
}

func filenameWithoutExt(path string) string {
	basename := filepath.Base(path)
	return strings.TrimSuffix(basename, filepath.Ext(basename))
}

func LoadCustomStatusIcons(configDir string) error {
	dir := filepath.Join(configDir, statusIconsDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("os.ReadDir: %v", err)
	}

	targets := []struct {
		prefix string
		icon   *Icon
	}{
		{"default", &Default},
		{"crash", &Crash},
		{"pause", &Pause},
		{"live-reload", &LiveReload},
	}

	for _, target := range targets {
		filename, isTemplate, ok := findStatusIcon(entries, target.prefix)
		if !ok {
			continue
		}
		path := filepath.Join(dir, filename)

		log.Infof("loading status icon: %s (template: %v)", path, isTemplate)
		fileContent, err := os.ReadFile(path)
		if err != nil {
			log.Errorf("LoadCustomStatusIcons: os.ReadFile: %v", err)
			continue
		}
		if len(fileContent) == 0 {
			log.Errorf("LoadCustomStatusIcons: status icon file is empty: %s", path)
			continue
		}

		*target.icon = Icon{Data: fileContent, IsTemplate: isTemplate}
	}

	return nil
}

// Finds a status icon file matching a prefix.
// A "<prefix>Template.*" file wins over a plain "<prefix>*" one.
// Only first match is returned, others are ignored.
func findStatusIcon(entries []os.DirEntry, prefix string) (filename string, isTemplate bool, found bool) {
	var plainMatch string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := filenameWithoutExt(entry.Name())
		if name == prefix+templateSuffix {
			return entry.Name(), true, true
		}
		if name == prefix && plainMatch == "" {
			plainMatch = entry.Name()
		}
	}
	return plainMatch, false, plainMatch != ""
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
		data := [][]byte{defaultIconData, crashIconData, pauseIconData, liveReloadIconData}
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
