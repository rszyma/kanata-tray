package app

import (
	"fmt"
	"os"
	"path/filepath"
)

type LayerIcons struct {
	layerIcons   map[string][]byte
	fallbackIcon []byte
}

func (c LayerIcons) IconForLayerName(layerName string) []byte {
	if v, ok := c.layerIcons[layerName]; ok {
		fmt.Printf("Icon for layer '%s'\n", layerName)
		return v
	} else {
		fmt.Printf("Fallback icon for layer '%s'\n", layerName)
		return c.fallbackIcon
	}
}

func (c LayerIcons) MappedLayers() []string {
	var res []string
	for layerName := range c.layerIcons {
		res = append(res, layerName)
	}
	return res
}

func ResolveIcons(configFolder string, unvalidatedLayerIcons map[string]string, defaultFallbackIcon []byte) LayerIcons {
	customIconsFolder := filepath.Join(configFolder, "icons")
	var layerIcons = make(map[string][]byte)
	for layerName, unvalidatedIconPath := range unvalidatedLayerIcons {
		var path string
		if filepath.IsAbs(unvalidatedIconPath) {
			path = unvalidatedIconPath
		} else {
			path = filepath.Join(customIconsFolder, unvalidatedIconPath)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("Custom icon file '%s' can't be accessed: %v\n", path, err)
			continue
		}
		layerIcons[layerName] = content
	}

	var fallbackIcon []byte
	if v, ok := layerIcons["*"]; ok {
		fallbackIcon = v
		delete(layerIcons, "*")
	} else {
		fallbackIcon = defaultFallbackIcon
	}

	return LayerIcons{
		layerIcons:   layerIcons,
		fallbackIcon: fallbackIcon,
	}
}
