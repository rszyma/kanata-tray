package config_test

import (
	_ "embed"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/rszyma/kanata-tray/config"
	defaultconfig "github.com/rszyma/kanata-tray/config/default_config"
	"github.com/rszyma/kanata-tray/lib"
)

//go:embed testdata/config_sample_toml10.toml
var cfgSampleToml10 string

//go:embed testdata/config_sample_toml11.toml
var cfgSampleToml11 string

// Wraps a hook command in the default `cmd_template` of the current OS.
func defaultHookCmd(cmd string) []string {
	if runtime.GOOS == "windows" {
		return []string{cmd}
	}
	return []string{"/bin/sh", "-c", cmd}
}

func TestParseConfig(t *testing.T) {
	cmpTransformer := cmpopts.AcyclicTransformer("OrderedMapToEntries",
		func(m *lib.OrderedMap[string, *config.Preset]) []lib.Entry[string, *config.Preset] {
			return m.Entries()
		},
	)

	// Result of parsing an empty config file.
	wantDefault := &config.Config{
		PresetDefaults: config.Preset{TcpPort: 5829},
		General:        config.GeneralConfigOptions{ControlServerPort: 8100},
		Presets: lib.NewOrderedMapFromIter([]lib.Entry[string, *config.Preset]{
			{Key: "Default Preset", Value: &config.Preset{TcpPort: 5829}},
		}),
	}

	// Result of parsing each of the testdata/config_sample_*.toml files.
	wantSample := &config.Config{
		PresetDefaults: config.Preset{
			TcpPort:    5829,
			LayerIcons: map[string]string{"*": "other.ico", "mouse": "mouse.png"},
			ExtraArgs:  []string{"--nodelay"},
		},
		General: config.GeneralConfigOptions{
			ControlServerEnable: true,
			ControlServerPort:   8100,
		},
		Presets: lib.NewOrderedMapFromIter([]lib.Entry[string, *config.Preset]{
			{
				Key: "with hooks",
				Value: &config.Preset{
					Autorun:   true,
					TcpPort:   5829,
					Hooks:     config.Hooks{PreStart: [][]string{defaultHookCmd("notify-send hi")}},
					ExtraArgs: []string{"--nodelay"},
					ExtraEnv:  map[string]string{"FOO": "1"},
				},
			},
			{
				Key: "plain",
				Value: &config.Preset{
					KanataConfig: "test.kbd",
					TcpPort:      5829,
					ExtraArgs:    []string{"--nodelay"},
				},
			},
		}),
	}

	tests := []struct {
		name    string
		cfgText string
		want    *config.Config
	}{
		{"empty config", "", wantDefault},
		{"unknown config entry is ignored", "general.unknown_option_123 = true\n", wantDefault},
		{"sample config in TOML v1.0", cfgSampleToml10, wantSample},
		{"sample config in TOML v1.1", cfgSampleToml11, wantSample},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := config.ParseConfig(tt.cfgText, defaultconfig.DefaultConfigContent)
			if err != nil {
				t.Fatalf("config.ParseConfig: %v", err)
			}
			if diff := cmp.Diff(tt.want, got, cmpTransformer); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseAutorunBool(t *testing.T) {
	cfgText := `
	[defaults]
	autorun = false

	[presets."preset1"]
	autorun = true
	`
	cfg, err := config.ParseConfig(cfgText, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Presets.GetOrDefault("preset1", nil).Autorun != true {
		t.Errorf("expected preset1.autorun == true, but got false")
	}
}

func TestParseAutorunExprLiteralTrue(t *testing.T) {
	cfgText := `
	[defaults]
	autorun = false

	[presets."preset1"]
	autorun = "true"
	`
	cfg, err := config.ParseConfig(cfgText, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Presets.GetOrDefault("preset1", nil).Autorun != true {
		t.Errorf("expected preset1.autorun == true, but got false")
	}
}

func TestParseAutorunExprSystemConditional(t *testing.T) {
	cfgText := `
	[defaults]
	autorun = false

	[presets."preset1"]
	autorun = "(linux || darwin || windows)"
	`
	cfg, err := config.ParseConfig(cfgText, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Presets.GetOrDefault("preset1", nil).Autorun != true {
		t.Errorf("expected preset1.autorun == true, but got false")
	}
}

func TestParseAutorunExprEnvConditional(t *testing.T) {
	os.Setenv("KANATA_TRAY_TEST_ENV", "test")
	cfgText := `
	[defaults]
	autorun = false

	[presets."preset1"]
	autorun = 'env("KANATA_TRAY_TEST_ENV") == "test"'
	`
	cfg, err := config.ParseConfig(cfgText, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Presets.GetOrDefault("preset1", nil).Autorun != true {
		t.Errorf("expected preset1.autorun == true, but got false")
	}
}

func TestParseAutorunExprFailsOnUndefinedVariable(t *testing.T) {
	os.Setenv("KANATA_TRAY_TEST_ENV", "test")
	cfgText := `
	[defaults]
	autorun = false

	[presets."preset1"]
	autorun = 'undefined_var'
	`
	cfg, err := config.ParseConfig(cfgText, "")
	if err != nil {
		if !strings.Contains(err.Error(), "unknown name undefined_var") {
			t.Errorf("error message differs:\n%v", err)
		}
	} else {
		t.Errorf("should fail, but succeeded with: %v", cfg)
	}
}

func TestParsePresetsOrder(t *testing.T) {
	tests := []struct {
		name    string
		cfgText string
		want    []string
	}{
		{
			name: "tables and keys in presets table",
			cfgText: `
			[presets.a]
			[presets.b.hooks]
			[presets]
			c = { kanata_config = "x" }
			d.kanata_config = "x"
			[presets.a.hooks]
			[presets.e]
			`,
			want: []string{"a", "b", "c", "d", "e"},
		},
		{
			name: "dotted keys",
			cfgText: `
			presets.b.kanata_config = "x"
			presets.a = {}
			`,
			want: []string{"b", "a"},
		},
		{
			name:    "inline table",
			cfgText: `presets = { b = {}, a.kanata_config = "x" }`,
			want:    []string{"b", "a"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := config.ParseConfig(tt.cfgText, defaultconfig.DefaultConfigContent)
			if err != nil {
				t.Fatal(err)
			}
			got := []string{}
			for _, entry := range cfg.Presets.Entries() {
				got = append(got, entry.Key)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
