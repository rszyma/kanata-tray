package config

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/elliotchance/orderedmap/v2"
	"github.com/k0kubun/pp/v3"
	"github.com/kr/pretty"
	"github.com/labstack/gommon/log"
	"github.com/pelletier/go-toml/v2"
	tomlu "github.com/pelletier/go-toml/v2/unstable"
)

var defaultCfg = `
# For help with configuration see https://github.com/rszyma/kanata-tray/blob/main/README.md
"$schema" = "https://raw.githubusercontent.com/rszyma/kanata-tray/main/doc/config_schema.json"

general.allow_concurrent_presets = false
defaults.tcp_port = 5829

[defaults.hooks]
# Hooks allow running custom commands on specific events (e.g. when starting preset).
# Documentation: https://github.com/rszyma/kanata-tray/blob/main/doc/hooks.md

[defaults.layer_icons]


[presets.'Default Preset']
kanata_executable = ''
kanata_config = ''
autorun = false

`

type Config struct {
	PresetDefaults Preset
	General        GeneralConfigOptions
	Presets        *OrderedMap[string, *Preset]
}

type Preset struct {
	Autorun            bool
	KanataExecutable   string
	KanataConfig       string
	TcpPort            int
	LayerIcons         map[string]string
	Hooks              Hooks
	ExtraArgs          []string
	AutorestartOnCrash bool
}

func (m *Preset) GoString() string {
	pp.Default.SetColoringEnabled(false)
	return pp.Sprintf("%s", m)
}

type GeneralConfigOptions struct {
	AllowConcurrentPresets bool
}

// Parsed hooks that contain list of args.
type Hooks struct {
	PreStart       [][]string
	PostStart      [][]string
	PostStartAsync [][]string
	PostStop       [][]string
}

// =========
// All golang toml parsers suck :/

type config struct {
	PresetDefaults *preset               `toml:"defaults"`
	General        *generalConfigOptions `toml:"general"`
	Presets        map[string]preset     `toml:"presets"`
}

type preset struct {
	Autorun            *bool             `toml:"autorun"`
	KanataExecutable   *string           `toml:"kanata_executable"`
	KanataConfig       *string           `toml:"kanata_config"`
	TcpPort            *int              `toml:"tcp_port"`
	LayerIcons         map[string]string `toml:"layer_icons"`
	Hooks              *hooks            `toml:"hooks"`
	ExtraArgs          extraArgs         `toml:"extra_args"`
	AutorestartOnCrash *bool             `toml:"autorestart_on_crash"`
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
	if p.ExtraArgs == nil {
		p.ExtraArgs = defaults.ExtraArgs
	}
	if p.AutorestartOnCrash == nil {
		p.AutorestartOnCrash = defaults.AutorestartOnCrash
	}
}

func (p *preset) intoExported() (*Preset, error) {
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
		x, err := p.Hooks.intoExported()
		if err != nil {
			return nil, err
		}
		result.Hooks = *x
	}
	if p.ExtraArgs != nil {
		x, err := p.ExtraArgs.intoExported()
		if err != nil {
			return nil, err
		}
		result.ExtraArgs = x
	}
	if p.AutorestartOnCrash != nil {
		result.AutorestartOnCrash = *p.AutorestartOnCrash
	}
	return result, nil
}

type generalConfigOptions struct {
	AllowConcurrentPresets *bool `toml:"allow_concurrent_presets"`
}

type hooks struct {
	PreStart       []string `toml:"pre-start"`
	PostStart      []string `toml:"post-start"`
	PostStartAsync []string `toml:"post-start-async"`
	PostStop       []string `toml:"post-stop"`
}

func (p *hooks) intoExported() (*Hooks, error) {
	parseResults := make([][][]string, 0, 4)
	hookLists := [][]string{p.PreStart, p.PostStart, p.PostStartAsync, p.PostStop}
	for _, hooks := range hookLists {
		parsedHooksOfOneKind := [][]string{}
		for _, hook := range hooks {
			args, err := parseCmd(hook)
			if err != nil {
				return nil, fmt.Errorf("failed to parse hook command '%s': %s", hook, err)
			}
			parsedHooksOfOneKind = append(parsedHooksOfOneKind, args)
		}
		parseResults = append(parseResults, parsedHooksOfOneKind)
	}
	return &Hooks{
		PreStart:       parseResults[0],
		PostStart:      parseResults[1],
		PostStartAsync: parseResults[2],
		PostStop:       parseResults[3],
	}, nil
}

type extraArgs []string

func (e extraArgs) intoExported() ([]string, error) {
	for _, s := range e {
		if strings.HasPrefix(s, "--port") || strings.HasPrefix(s, "-p") {
			return nil, fmt.Errorf("port argument is not allowed in extra_args, use tcp_port instead")
		}
	}
	return e, nil
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
		log.Infof("Config file doesn't exist. Creating default config. Path: '%s'", configFilePath)
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

	defaultsExported, err := defaults.intoExported()
	if err != nil {
		return nil, err
	}
	var cfg2 *Config = &Config{
		PresetDefaults: *defaultsExported,
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
		exported, err := v.intoExported()
		if err != nil {
			return nil, err
		}
		cfg2.Presets.Set(layerName, exported)
	}

	log.Debugf("loaded config: %s", pretty.Sprint(cfg2))
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

func parseCmd(cmdWithArgs string) ([]string, error) {
	result := []string{}
	level2_start_chars := []rune{'\'', '"'}
	builder := strings.Builder{}
	var level2_start rune = rune(0)
	for _, char := range cmdWithArgs {
		if level2_start != rune(0) {
			if char == level2_start {
				result = append(result, builder.String())
				builder.Reset()
				level2_start = rune(0)
				continue
			}
			builder.WriteRune(char)
			continue
		}
		// is on level 1

		if char == ' ' {
			result = append(result, builder.String())
			builder.Reset()
			continue
		}

		for _, c := range level2_start_chars {
			if char == c {
				result = append(result, builder.String())
				builder.Reset()
				level2_start = char
				break
			}
		}
		if level2_start != rune(0) {
			continue
		}

		builder.WriteRune(char)
	}

	switch level2_start {
	case rune(0):
		// all good
	case '\'':
		return nil, fmt.Errorf("unclosed single-quote character")
	case '"':
		return nil, fmt.Errorf("unclosed quote character")
	default:
		panic("unreachable")
	}

	leftover := builder.String()
	if len(leftover) != 0 {
		result = append(result, leftover)
	}

	return result, nil
}
