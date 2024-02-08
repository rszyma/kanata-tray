package config

import (
	"fmt"
	"os"

	"github.com/k0kubun/pp/v3"
	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Configurations []string             `toml:"configurations"`
	Executables    []string             `toml:"executables"`
	General        GeneralConfigOptions `toml:"general"`
}

type GeneralConfigOptions struct {
	IncludeExecutablesFromSystemPath   bool `toml:"include_executables_from_system_path"`
	IncludeConfigsFromDefaultLocations bool `toml:"include_configs_from_default_locations"`
	LaunchOnStart                      bool `toml:"launch_on_start"`
}

func ReadConfigOrCreateIfNotExist(configFilePath string) (*Config, error) {
	var cfg *Config = &Config{}
	err := toml.Unmarshal([]byte(defaultCfg), &cfg)
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

	pp.Println("%v", cfg)
	return cfg, nil
}

var defaultCfg = `
# See https://github.com/rszyma/kanata-tray for help with configuration.

configurations = [
    
]

executables = [
    
]

[layer_icons]


[general]
include_executables_from_system_path = true
include_configs_from_default_locations = true
launch_on_start = true
`
