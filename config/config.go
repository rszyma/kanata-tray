package config

import (
	"fmt"
	"os"

	"github.com/elliotchance/orderedmap/v2"
	"github.com/k0kubun/pp/v3"
	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	PresetDefaults Preset
	General        GeneralConfigOptions
	Presets        orderedmap.OrderedMap[string, *Preset]
}

type Preset struct {
	Autorun          bool
	KanataExecutable string
	KanataConfig     string
	TcpPort          int
	LayerIcons       map[string]string
}

type GeneralConfigOptions struct {
	AllowConcurrentPresets bool
}

// =========
// All golang toml parsers suck :/

type config struct {
	PresetDefaults *preset               `toml:"defaults"`
	General        *generalConfigOptions `toml:"general"`
	Presets        map[string]preset     `toml:"presets"`
}

type preset struct {
	Autorun          *bool             `toml:"autorun"`
	KanataExecutable *string           `toml:"kanata_executable"`
	KanataConfig     *string           `toml:"kanata_config"`
	TcpPort          *int              `toml:"tcp_port"`
	LayerIcons       map[string]string `toml:"layer_icons"`
}

func (p *preset) applyDefaults(defaults *preset) {
	if p.Autorun == nil {
		p.Autorun = defaults.Autorun
	}
	if p.KanataExecutable == nil {
		p.KanataExecutable = defaults.KanataExecutable
	}
	if p.KanataConfig == nil {
		p.KanataConfig = defaults.KanataConfig
	}
	if p.TcpPort == nil {
		p.TcpPort = defaults.TcpPort
	}
	// This is intended because we layer icons are handled specially.
	//
	// if p.LayerIcons == nil {
	// 	p.LayerIcons = defaults.LayerIcons
	// }
}

func (p *preset) intoExported() *Preset {
	result := &Preset{}
	if p.Autorun != nil {
		result.Autorun = *p.Autorun
	}
	if p.KanataExecutable != nil {
		result.KanataExecutable = *p.KanataExecutable
	}
	if p.KanataConfig != nil {
		result.KanataConfig = *p.KanataConfig
	}
	if p.TcpPort != nil {
		result.TcpPort = *p.TcpPort
	}
	if p.LayerIcons != nil {
		result.LayerIcons = p.LayerIcons
	}
	return result
}

type generalConfigOptions struct {
	AllowConcurrentPresets *bool `toml:"allow_concurrent_presets"`
}

func ReadConfigOrCreateIfNotExist(configFilePath string) (*Config, error) {
	var cfg *config = &config{}
	err := toml.Unmarshal([]byte(defaultCfg), &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse default config: %v", err)
	}
	// temporarily remove default presets
	presetsFromDefaultConfig := cfg.Presets
	cfg.Presets = nil

	// Does the file not exist?
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		fmt.Printf("Config file doesn't exist. Creating default config. Path: '%s'\n", configFilePath)
		os.WriteFile(configFilePath, []byte(defaultCfg), os.FileMode(0600))
	} else {
		// Load the existing file.
		fh, err := os.Open(configFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to open file '%s': %v", configFilePath, err)
		}
		defer fh.Close()
		err = toml.NewDecoder(fh).Decode(&cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to parse config file '%s': %v", configFilePath, err)
		}
	}

	if cfg.Presets == nil {
		cfg.Presets = presetsFromDefaultConfig
	}

	defaults := cfg.PresetDefaults

	var cfg2 *Config = &Config{
		PresetDefaults: *defaults.intoExported(),
		General: GeneralConfigOptions{
			AllowConcurrentPresets: *cfg.General.AllowConcurrentPresets,
		},
		Presets: util.NewOrdereredMap[string, *Preset](),
	}

	// TODO: keep order of items
	for k, v := range cfg.Presets {
		v.applyDefaults(defaults)
		exported := v.intoExported()
		cfg2.Presets[k] = exported
	}

	pp.Printf("loaded config: %v\n", cfg2)
	return cfg2, nil
}

var defaultCfg = `
# See https://github.com/rszyma/kanata-tray for help with configuration.
"$schema" = "https://raw.githubusercontent.com/rszyma/kanata-tray/v0.1.0/doc/config_schema.json"

general.allow_concurrent_presets = false
defaults.tcp_port = 5829

[defaults.layer_icons]


[presets.'Default Preset']
kanata_executable = ''
kanata_config = ''
autorun = false

`
