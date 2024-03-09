package config

import (
	"fmt"
	"os"

	"github.com/k0kubun/pp/v3"
	"github.com/pelletier/go-toml/v2"
)

type partialConfigJustDefaults struct {
	PresetDefaults Preset `toml:"defaults"`
}

type Config struct {
	partialConfigJustDefaults
	General GeneralConfigOptions `toml:"general"`
	Presets map[string]Preset    `toml:"presets"`
}

type GeneralConfigOptions struct {
	AllowConcurrentPresets bool `toml:"allow_concurrent_presets"`
}

type Preset struct {
	Autorun          bool              `toml:"autorun"`
	KanataExecutable string            `toml:"kanata_executable"`
	KanataConfig     string            `toml:"kanata_config"`
	TcpPort          int               `toml:"tcp_port"`
	LayerIcons       map[string]string `toml:"layer_icons"`
}

var defaults *partialConfigJustDefaults = nil

func (c *Preset) UnmarshalTOML(text []byte) error {
	if defaults != nil {
		c = &defaults.PresetDefaults
	}
	return toml.Unmarshal(text, c)
}

func ReadConfigOrCreateIfNotExist(configFilePath string) (*Config, error) {
	defaults = &partialConfigJustDefaults{}
	err := toml.Unmarshal([]byte(defaultCfg), defaults)
	if err != nil {
		return nil, fmt.Errorf("failed to parse default config: %v", err)
	}

	var cfg *Config = &Config{}
	err = toml.Unmarshal([]byte(defaultCfg), &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse default config: %v", err)
	}

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
		decoder := toml.NewDecoder(fh)
		err = decoder.Decode(&cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to parse config file '%s': %v", configFilePath, err)
		}
	}

	pp.Println("%v", defaults)
	pp.Println("%v", cfg)
	return cfg, nil
}

var defaultCfg = `
# See https://github.com/rszyma/kanata-tray for help with configuration.
"$schema" = "https://raw.githubusercontent.com/rszyma/kanata-tray/v0.1.0/doc/config_schema.json"

general.allow_concurrent_presets = false

[defaults.layer_icons]


[presets.'Default Preset']
kanata_executable = ''
kanata_config = ''
autorun = false
tcp_port = 5829

`
