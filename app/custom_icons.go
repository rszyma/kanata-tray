package app

import (
	"os"
	"path/filepath"

	"github.com/labstack/gommon/log"

	"github.com/rszyma/kanata-tray/config"
)

type LayerIcons struct {
	presetIcons  map[string]*LayerIconsForPreset
	defaultIcons LayerIconsForPreset
}

type LayerIconsForPreset struct {
	layerIcons   map[string][]byte
	wildcardIcon []byte // can be nil
}

// Order of resolution:
// preset -> global -> preset_wildcard -> global_wildcard -> default
//
// Returns nil if resolution yields no icon. Caller should then use global default icon.
func (c LayerIcons) IconForLayerName(presetName string, layerName string) []byte {
	// preset
	preset, ok := c.presetIcons[presetName]
	if ok {
		if layerIcon, ok := preset.layerIcons[layerName]; ok {
			log.Infof("Setting icon: preset:%s, layer:%s", presetName, layerName)
			return layerIcon
		}
	}
	// global
	layerIcon, ok := c.defaultIcons.layerIcons[layerName]
	if ok {
		log.Infof("Setting icon: preset:*, layer:%s", layerName)
		return layerIcon
	}
	// preset_wildcard
	if preset != nil && preset.wildcardIcon != nil {
		log.Infof("Setting icon: preset:%s, layer:*", presetName)
		return preset.wildcardIcon
	}
	// global_wildcard
	if c.defaultIcons.wildcardIcon != nil {
		log.Infof("Setting icon: preset:*, layer:*")
		return c.defaultIcons.wildcardIcon
	}
	// default
	return nil
}

func (c LayerIcons) MappedLayers(presetName string) []string {
	var res []string
	for layerName := range c.defaultIcons.layerIcons {
		res = append(res, layerName)
	}
	presetIcons, ok := c.presetIcons[presetName]
	if !ok {
		// return only layers name in "defaults" section
		return res
	}
	for layerName := range presetIcons.layerIcons {
		res = append(res, layerName)
	}
	return res
}

func ResolveIcons(configFolder string, cfg *config.Config) LayerIcons {
	customIconsFolder := filepath.Join(configFolder, "icons")
	var icons = LayerIcons{
		presetIcons: make(map[string]*LayerIconsForPreset),
		defaultIcons: LayerIconsForPreset{
			layerIcons:   make(map[string][]byte),
			wildcardIcon: nil,
		},
	}
	for layerName, unvalidatedIconPath := range cfg.PresetDefaults.LayerIcons {
		data, err := readIconInFolder(unvalidatedIconPath, customIconsFolder)
		if err != nil {
			log.Warnf("defaults - custom icon file can't be read: %v", err)
		} else if layerName == "*" {
			icons.defaultIcons.wildcardIcon = data
		} else {
			icons.defaultIcons.layerIcons[layerName] = data
		}
	}

	for m := cfg.Presets.Front(); m != nil; m = m.Next() {
		presetName := m.Key
		preset := m.Value
		for layerName, unvalidatedIconPath := range preset.LayerIcons {
			data, err := readIconInFolder(unvalidatedIconPath, customIconsFolder)
			if err != nil {
				log.Warnf("Preset '%s' - custom icon file can't be read: %v", presetName, err)
			} else if layerName == "*" {
				icons.presetIcons[presetName].wildcardIcon = data
			} else {
				icons.presetIcons[presetName].layerIcons[layerName] = data
			}
		}
	}
	return icons
}

func readIconInFolder(filePath string, folder string) ([]byte, error) {
	var path string
	if filepath.IsAbs(filePath) {
		path = filePath
	} else {
		path = filepath.Join(folder, filePath)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return content, nil
}
