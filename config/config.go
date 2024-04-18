package config

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/elliotchance/orderedmap/v2"
	"github.com/k0kubun/pp/v3"
	"github.com/kr/pretty"
	"github.com/pelletier/go-toml/v2"
	tomlu "github.com/pelletier/go-toml/v2/unstable"
)

type Config struct {
	PresetDefaults Preset
	General        GeneralConfigOptions
	Presets        *OrderedMap[string, *Preset]
}

type Preset struct {
	Autorun          bool
	KanataExecutable string
	KanataConfig     string
	TcpPort          int
	LayerIcons       map[string]string
	Hooks            Hooks
}

func (m *Preset) GoString() string {
	pp.Default.SetColoringEnabled(false)
	return pp.Sprintf("%s", m)
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
	Hooks            *Hooks            `toml:"hooks"`
}

type Hooks struct {
	PreStart       []string `toml:"pre-start"`
	PostStart      []string `toml:"post-start"`
	PostStartAsync []string `toml:"post-start-async"`
	PostStop       []string `toml:"post-stop"`
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
	//// Excluding layer icons is intended because they are handled specially.
	//
	// if p.LayerIcons == nil {
	// 	p.LayerIcons = defaults.LayerIcons
	// }
	if p.Hooks == nil {
		p.Hooks = defaults.Hooks
	}
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
	if p.Hooks != nil {
		result.Hooks = *p.Hooks
	}
	return result
}

type generalConfigOptions struct {
	AllowConcurrentPresets *bool `toml:"allow_concurrent_presets"`
}

func ReadConfigOrCreateIfNotExist(configFilePath string) (*Config, error) {
	var cfg *config = &config{}
	// Golang map don't keep track of insertion order, so we need to get the
	// order of declarations in toml separately.
	layersNames, err := layersOrder([]byte(defaultCfg))
	if err != nil {
		panic(fmt.Errorf("default config failed layersOrder: %v", err))
	}
	err = toml.Unmarshal([]byte(defaultCfg), &cfg)
	if err != nil {
		panic(fmt.Errorf("failed to parse default config: %v", err))
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
		content, err := os.ReadFile(configFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file '%s': %v", configFilePath, err)
		}
		err = toml.NewDecoder(bytes.NewReader(content)).Decode(&cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to parse config file '%s': %v", configFilePath, err)
		}
		lnames, err := layersOrder(content)
		if err != nil {
			panic("default config failed layersOrder")
		}
		if len(lnames) != 0 {
			layersNames = lnames
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
		Presets: NewOrderedMap[string, *Preset](),
	}

	for _, layerName := range layersNames {
		v, ok := cfg.Presets[layerName]
		if !ok {
			panic("layer names should match")
		}
		v.applyDefaults(defaults)
		exported := v.intoExported()
		cfg2.Presets.Set(layerName, exported)
	}

	pretty.Println("loaded config:", cfg2)
	return cfg2, nil
}

// Returns an array of layer names from config in order of declaration.
func layersOrder(cfgContent []byte) ([]string, error) {
	layerNamesInOrder := []string{}

	p := tomlu.Parser{}
	p.Reset([]byte(cfgContent))

	// iterate over all top level expressions
	for p.NextExpression() {
		e := p.Expression()

		if e.Kind != tomlu.Table {
			continue
		}

		// Let's look at the key. It's an iterator over the multiple dotted parts of the key.
		it := e.Key()
		parts := keyAsStrings(it)

		// we're only considering keys that look like `presets.XXX`
		if len(parts) != 2 {
			continue
		}
		if parts[0] != "presets" {
			continue
		}

		layerNamesInOrder = append(layerNamesInOrder, string(parts[1]))
	}

	return layerNamesInOrder, nil

}

// helper to transfor a key iterator to a slice of strings
func keyAsStrings(it tomlu.Iterator) []string {
	var parts []string
	for it.Next() {
		n := it.Node()
		parts = append(parts, string(n.Data))
	}
	return parts
}

// var _ tomlu.Unmarshaler = (*OrderedMap[string, preset])(nil)

// func (m *OrderedMap[string, preset]) UnmarshalTOML(node *tomlu.Node) error {
// 	fmt.Println(node)
// 	m = NewOrderedMap[string, preset]()
// 	// m.Set("asdf", preset{})
// 	for iter, ok := node.Key(), true; ok; ok = iter.Next() {
// 		n := iter.Node()
// 		fmt.Printf("n.Data: %v\n", n.Data)
// 		// m.Set(k, v)
// 	}

// 	return nil
// }

type OrderedMap[K string, V fmt.GoStringer] struct {
	*orderedmap.OrderedMap[K, V]
}

func NewOrderedMap[K string, V fmt.GoStringer]() *OrderedMap[K, V] {
	return &OrderedMap[K, V]{
		OrderedMap: orderedmap.NewOrderedMap[K, V](),
	}
}

// impl `fmt.GoStringer`
func (m *OrderedMap[K, V]) GoString() string {
	indent := "    "
	keys := []K{}
	values := []V{}
	for it := m.Front(); it != nil; it = it.Next() {
		keys = append(keys, it.Key)
		values = append(values, it.Value)
	}
	builder := strings.Builder{}
	builder.WriteString("{")
	for i := range keys {
		key := keys[i]
		value := values[i]
		valueLines := strings.Split(value.GoString(), "\n")
		for i, vl := range valueLines {
			if i == 0 {
				continue
			}
			valueLines[i] = fmt.Sprintf("%s%s", indent, vl)
		}
		indentedVal := strings.Join(valueLines, "\n")
		builder.WriteString(fmt.Sprintf("\n%s\"%s\": %s", indent, key, indentedVal))
	}
	builder.WriteString("\n}")
	return builder.String()
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
