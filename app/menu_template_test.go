package app

import (
	"os"
	"testing"
)

func TestExpandHomeDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory:", err)
	}
	cases := map[string]string{
		"":                         "",
		"~":                        home,
		"~/kanata.kbd":             home + "/kanata.kbd",
		"/etc/kanata.kbd":          "/etc/kanata.kbd",
		`C:\PROGRA~1\kanata.exe`:   `C:\PROGRA~1\kanata.exe`,
		"/home/u/~/kanata.kbd":     "/home/u/~/kanata.kbd",
		"~user/kanata.kbd":         "~user/kanata.kbd",
		"--cfg=/home/u/kanata.kbd": "--cfg=/home/u/kanata.kbd",
	}
	for in, want := range cases {
		got, err := expandHomeDir(in)
		if err != nil {
			t.Errorf("expandHomeDir(%q): %v", in, err)
		} else if got != want {
			t.Errorf("expandHomeDir(%q) = %q, want %q", in, got, want)
		}
	}
}
