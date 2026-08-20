package config_test

import (
	"os"
	"strings"
	"testing"

	"github.com/rszyma/kanata-tray/config"
)

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
