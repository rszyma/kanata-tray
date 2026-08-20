package config

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/elliotchance/orderedmap/v2"
	"github.com/expr-lang/expr"
	"github.com/k0kubun/pp/v3"
	"github.com/kr/pretty"
	"github.com/labstack/gommon/log"
	"github.com/pelletier/go-toml/v2"
	tomlu "github.com/pelletier/go-toml/v2/unstable"

	_ "embed"
)

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
	ExtraEnv           map[string]string
	AutorestartOnCrash bool
}

func (m *Preset) GoString() string {
	pp.Default.SetColoringEnabled(false)
	return pp.Sprintf("%s", m)
}

type GeneralConfigOptions struct {
	AllowConcurrentPresets bool
	ControlServerEnable    bool
	ControlServerPort      int
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
	AutorunExpr        any               `toml:"autorun"`
	KanataExecutable   *string           `toml:"kanata_executable"`
	KanataConfig       *string           `toml:"kanata_config"`
	TcpPort            *int              `toml:"tcp_port"`
	LayerIcons         map[string]string `toml:"layer_icons"`
	Hooks              *hooks            `toml:"hooks"`
	ExtraArgs          extraArgs         `toml:"extra_args"`
	ExtraEnv           map[string]string `toml:"extra_env"`
	AutorestartOnCrash *bool             `toml:"autorestart_on_crash"`
}

// TODO: move this to toml parser?
func (p *preset) applyDefaults(defaults *preset) {
	if p.AutorunExpr == nil {
		p.AutorunExpr = defaults.AutorunExpr
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
	if p.ExtraEnv == nil {
		p.ExtraEnv = defaults.ExtraEnv
	}
	if p.AutorestartOnCrash == nil {
		p.AutorestartOnCrash = defaults.AutorestartOnCrash
	}
}

func (p *preset) IntoExported(name string) (*Preset, error) {
	result := &Preset{}
	if p.AutorunExpr != nil {
		switch autorunExpr := p.AutorunExpr.(type) {
		case bool:
			result.Autorun = autorunExpr
		case string:
			env := map[string]any{
				"linux":   runtime.GOOS == "linux",
				"windows": runtime.GOOS == "windows",
				"darwin":  runtime.GOOS == "darwin",
				"env": func(key string) string {
					return os.Getenv(key)
				},
			}
			program, err := expr.Compile(autorunExpr, expr.AsBool(), expr.Env(env))
			if err != nil {
				return nil, fmt.Errorf("while parsing autorun expression: %s", err)
			}
			output, err := expr.Run(program, env)
			if err != nil {
				return nil, fmt.Errorf("while evaluating autorun expression: %s", err)
			}
			outputBool := output.(bool)
			result.Autorun = outputBool
			log.Infof("autorun for preset '%s' evaluated %s (expr: '%s')", name, strconv.FormatBool(outputBool), autorunExpr)
		default:
			return nil, fmt.Errorf("autorun has invalid type. Supported types: boolean, string")
		}
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
	if p.ExtraEnv != nil {
		result.ExtraEnv = p.ExtraEnv
	}
	if p.AutorestartOnCrash != nil {
		result.AutorestartOnCrash = *p.AutorestartOnCrash
	}
	return result, nil
}

type generalConfigOptions struct {
	AllowConcurrentPresets *bool `toml:"allow_concurrent_presets"`
	ControlServerEnable    *bool `toml:"control_server_enable"`
	ControlServerPort      *int  `toml:"control_server_port"`
}

func (g *generalConfigOptions) IntoExported() GeneralConfigOptions {
	if g == nil {
		return GeneralConfigOptions{}
	}
	var G GeneralConfigOptions
	if g.AllowConcurrentPresets != nil {
		G.AllowConcurrentPresets = *g.AllowConcurrentPresets
	}
	if g.ControlServerEnable != nil {
		G.ControlServerEnable = *g.ControlServerEnable
	}
	if g.ControlServerPort != nil {
		G.ControlServerPort = *g.ControlServerPort
	}
	return G
}

type hooks struct {
	CmdTemplate    []string `toml:"cmd_template"`
	PreStart       []string `toml:"pre-start"` // TODO: rename to snake case.
	PostStart      []string `toml:"post-start"`
	PostStartAsync []string `toml:"post-start-async"`
	PostStop       []string `toml:"post-stop"`
}

type cmdTempl struct {
	inner [](func(s string) string)
}

func newCmdTemplFromRaw(cmdTemplate []string) (*cmdTempl, error) {
	var r = new(cmdTempl)
	var fmtSeqCount int

	for _, arg := range cmdTemplate {
		fmtSeqCount += strings.Count(arg, "{}")
		arg := arg
		r.inner = append(r.inner, func(s string) string {
			return strings.ReplaceAll(arg, "{}", s)
		})
	}

	if fmtSeqCount != 1 {
		return nil, fmt.Errorf(
			"expected exactly one occurence of {}, found %d",
			fmtSeqCount,
		)
	}

	return r, nil
}

func (t *cmdTempl) apply(s string) (finalArgv []string) {
	for _, fn := range t.inner {
		finalArgv = append(finalArgv, fn(s))
	}
	return finalArgv
}

func (t *cmdTempl) applyMany(xs []string) [][]string {
	var results [][]string
	for _, x := range xs {
		results = append(results, t.apply(x))
	}
	return results
}

func (p *hooks) intoExported() (*Hooks, error) {
	cmdTemplate := p.CmdTemplate
	if cmdTemplate == nil {
		if runtime.GOOS == "windows" {
			cmdTemplate = []string{"{}"} // TODO: better default? maybe powershell?
		} else {
			cmdTemplate = []string{"/bin/sh", "-c", "{}"}
		}
	}
	templ, err := newCmdTemplFromRaw(cmdTemplate)
	if err != nil {
		return nil, fmt.Errorf("while parsing cmd_template: %w", err)
	}
	return &Hooks{
		PreStart:       templ.applyMany(p.PreStart),
		PostStart:      templ.applyMany(p.PostStart),
		PostStartAsync: templ.applyMany(p.PostStartAsync),
		PostStop:       templ.applyMany(p.PostStop),
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

func ReadOrCreateConfigFile(configPath string, cfgDefaultText string) (string, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Infof("Config file doesn't exist. Creating default config. Path: '%s'", configPath)
		err = os.WriteFile(configPath, []byte(cfgDefaultText), os.FileMode(0600))
		if err != nil {
			return "", fmt.Errorf("while writing default config file to '%s': %v", configPath, err)
		}
		return cfgDefaultText, nil
	}
	content, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("while reading config file from '%s': %v", configPath, err)
	}
	return string(content), nil
}

func ParseConfig(cfgText string, cfgDefaultText string) (*Config, error) {
	var cfg *config = &config{}
	var err error

	err = toml.Unmarshal([]byte(cfgDefaultText), &cfg)
	if err != nil {
		panic(fmt.Errorf("failed to parse default config: %v", err))
	}
	// temporarily remove default presets
	presetsFromDefaultConfig := cfg.Presets
	cfg.Presets = nil

	err = toml.NewDecoder(strings.NewReader(cfgText)).Decode(&cfg)
	if err != nil {
		return nil, fmt.Errorf("while decoding toml: %v", err)
	}

	// Golang don't keep track of map insertion order,
	// neither the toml lib we use provide us this info,
	// so we need to hack something up to get the original declaration order.
	presetNames, err := presetsOrder(cfgText)
	if err != nil {
		panic("default config failed layersOrder")
	}

	// If parsed config has no presets, populate it with preset(s) from default config.
	if cfg.Presets == nil {
		cfg.Presets = presetsFromDefaultConfig
		defaultPresetNames, err := presetsOrder(cfgDefaultText)
		if err != nil {
			panic(fmt.Errorf("failed presetsOrder for default config: %v", err))
		}
		presetNames = defaultPresetNames
	}

	defaultsExported, err := cfg.PresetDefaults.IntoExported("<default>")
	if err != nil {
		return nil, err
	}
	var cfg2 *Config = &Config{
		PresetDefaults: *defaultsExported,
		General:        cfg.General.IntoExported(),
		Presets:        NewOrderedMap[string, *Preset](),
	}

	for _, presetName := range presetNames {
		p, ok := cfg.Presets[presetName]
		if !ok {
			panic("preset names should match")
		}
		p.applyDefaults(cfg.PresetDefaults)
		exported, err := p.IntoExported(presetName)
		if err != nil {
			return nil, fmt.Errorf("preset '%s': %s", presetName, err)
		}
		cfg2.Presets.Set(presetName, exported)
	}

	log.Debugf("loaded config: %s", pretty.Sprint(cfg2))
	return cfg2, nil
}

// Returns an array of preset names from config in order of declaration.
func presetsOrder(cfgText string) ([]string, error) {
	layerNamesInOrder := []string{}

	p := tomlu.Parser{}
	p.Reset([]byte(cfgText))

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
