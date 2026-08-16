package status_icons

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsTemplateFilename(t *testing.T) {
	cases := map[string]bool{
		"mouseTemplate.png":           true,
		"mousetemplate.png":           true,
		"/abs/path/mouseTemplate.ico": true,
		"defaultTemplate":             true,
		"mouse.png":                   false,
		"template_mouse.png":          false,
		"mouseTemplate.png.disabled":  false,
		"live-reloadTemplate.jpg":     true,
	}
	for path, want := range cases {
		if got := IsTemplateFilename(path); got != want {
			t.Errorf("IsTemplateFilename(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestLoadCustomStatusIcons(t *testing.T) {
	configDir := t.TempDir()
	iconsDir := filepath.Join(configDir, statusIconsDir)
	if err := os.MkdirAll(iconsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"default.ico":         "plain default",
		"defaultTemplate.png": "template default",
		"pause.ico":           "plain pause",
		"crash.ico":           "",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(iconsDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	crashBefore := Crash
	if err := LoadCustomStatusIcons(configDir); err != nil {
		t.Fatalf("LoadCustomStatusIcons: %v", err)
	}

	// A `<status>Template.*` file wins over a plain `<status>.*` one.
	if got := string(Default.Data); got != "template default" {
		t.Errorf("Default.Data = %q, want the template file's content", got)
	}
	if !Default.IsTemplate {
		t.Error("Default.IsTemplate = false, want true")
	}

	if got := string(Pause.Data); got != "plain pause" {
		t.Errorf("Pause.Data = %q, want the plain file's content", got)
	}
	if Pause.IsTemplate {
		t.Error("Pause.IsTemplate = true, want false")
	}

	// An empty file is ignored instead of being passed to systray, where it
	// would panic on `&iconBytes[0]`.
	if string(Crash.Data) != string(crashBefore.Data) {
		t.Error("Crash was overwritten by an empty icon file")
	}

	// Not overridden, keeps the embedded icon.
	if len(LiveReload.Data) == 0 || LiveReload.IsTemplate {
		t.Errorf("LiveReload = %d bytes, template=%v; want embedded non-template icon",
			len(LiveReload.Data), LiveReload.IsTemplate)
	}
}

func TestLoadCustomStatusIconsMissingDir(t *testing.T) {
	if err := LoadCustomStatusIcons(t.TempDir()); err != nil {
		t.Errorf("LoadCustomStatusIcons on a config dir without status_icons: %v", err)
	}
}
