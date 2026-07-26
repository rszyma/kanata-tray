package config_test

import (
	_ "embed"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/rszyma/kanata-tray/config"
)

//go:embed testdata/config_sample_toml10.toml
var cfgSampleToml10 []byte

//go:embed testdata/config_sample_toml11.toml
var cfgSampleToml11 []byte

func TestParseConfig(t *testing.T) {
	cmpTransformer := cmpopts.AcyclicTransformer("OrderedMapToEntries",
		func(m *config.OrderedMap[string, *config.Preset]) []config.Entry[string, *config.Preset] {
			return m.Entries()
		},
	)

	t.Run("default config", func(t *testing.T) {
		got, err := config.ParseConfig(nil)
		if err != nil {
			t.Errorf("config.ParseConfig: %v", err)
		}
		want := &config.Config{
			PresetDefaults: config.Preset{TcpPort: 5829},
			General:        config.GeneralConfigOptions{ControlServerPort: 8100},
			Presets: config.NewOrderedMapFromIter([]config.Entry[string, *config.Preset]{
				{
					Key: "Default Preset",
					Value: &config.Preset{
						TcpPort: 5829,
					},
				},
			}),
		}
		if diff := cmp.Diff(want, got, cmpTransformer); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("cfgSampleToml10", func(t *testing.T) {
		got, err := config.ParseConfig(cfgSampleToml10)
		if err != nil {
			t.Errorf("config.ParseConfig: %v", err)
		}
		want := &config.Config{
			PresetDefaults: config.Preset{
				TcpPort:            5829,
				LayerIcons:         map[string]string{"*": "other_layers.ico", "mouse": "mouse.png"},
				ExtraArgs:          []string{"--nodelay"},
				AutorestartOnCrash: true,
			},
			General: config.GeneralConfigOptions{
				ControlServerEnable: true,
				ControlServerPort:   8100,
			},
			Presets: config.NewOrderedMapFromIter([]config.Entry[string, *config.Preset]{
				{
					Key: "main - local - cmd-switch",
					Value: &config.Preset{
						Autorun:          true,
						KanataExecutable: "~/gh/kanata/target/release/kanata_cmd_switch",
						KanataConfig:     "~/.config/kanata/kanata.kbd",
						TcpPort:          5829,
						Hooks: config.Hooks{
							PreStart: [][]string{{"/bin/sh", "-c", "notify-send 'running kanata!!'"}},
						},
						ExtraArgs:          []string{"--nodelay"},
						ExtraEnv:           map[string]string{"KANATA_CMDSWITCH_ENABLE": "1"},
						AutorestartOnCrash: true,
					},
				},
				{
					Key: "main - local - release",
					Value: &config.Preset{
						KanataExecutable:   "~/gh/kanata/target/release/kanata",
						KanataConfig:       "~/.config/kanata/kanata.kbd",
						TcpPort:            5829,
						ExtraArgs:          []string{"--nodelay"},
						AutorestartOnCrash: true,
					},
				},
				{
					Key: "main - nixpkgs ver",
					Value: &config.Preset{
						KanataExecutable:   "/run/current-system/sw/bin/kanata",
						TcpPort:            5829,
						ExtraArgs:          []string{"--nodelay"},
						AutorestartOnCrash: true,
					},
				},
				{
					Key: "test",
					Value: &config.Preset{
						KanataConfig:       "~/.config/kanata/test.kbd",
						TcpPort:            5829,
						ExtraArgs:          []string{"--nodelay"},
						AutorestartOnCrash: true,
					},
				},
			}),
		}

		if diff := cmp.Diff(want, got, cmpTransformer); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("cfgSampleToml11", func(t *testing.T) {
		got, err := config.ParseConfig(cfgSampleToml11)
		if err != nil {
			t.Errorf("config.ParseConfig: %v", err)
		}
		want := &config.Config{
			PresetDefaults: config.Preset{TcpPort: 5829},
			General:        config.GeneralConfigOptions{ControlServerPort: 8100},
			Presets:        config.NewOrderedMapFromIter([]config.Entry[string, *config.Preset]{
				// FIXME
			}),
		}
		if diff := cmp.Diff(want, got, cmpTransformer); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("unknown config entry is ignored", func(t *testing.T) {
		got, err := config.ParseConfig([]byte("general.unknown_option_123 = true\n"))
		if err != nil {
			t.Errorf("config.ParseConfig: %v", err)
		}
		want := &config.Config{
			PresetDefaults: config.Preset{TcpPort: 5829},
			General:        config.GeneralConfigOptions{ControlServerPort: 8100},
			Presets: config.NewOrderedMapFromIter([]config.Entry[string, *config.Preset]{
				{
					Key: "Default Preset",
					Value: &config.Preset{
						TcpPort: 5829,
					},
				},
			}),
		}
		if diff := cmp.Diff(want, got, cmpTransformer); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}

	})
}
