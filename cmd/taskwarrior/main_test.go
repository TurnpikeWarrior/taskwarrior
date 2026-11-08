package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIInformationalFlagsAndTerminalRequirement(t *testing.T) {
	in, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	for _, flag := range []string{"--help", "--version"} {
		if err := run([]string{flag}, in, out); err != nil {
			t.Fatal(err)
		}
	}
	out.Seek(0, 0)
	var buf bytes.Buffer
	buf.ReadFrom(out)
	if !strings.Contains(buf.String(), "--data") || !strings.Contains(buf.String(), "milestone4") {
		t.Fatal(buf.String())
	}
	if err := run(nil, in, out); err == nil || !strings.Contains(err.Error(), "interactive terminal") {
		t.Fatal(err)
	}
	if err := run([]string{"unexpected"}, in, out); err == nil {
		t.Fatal("positional argument accepted")
	}
}
func TestDefaultPaths(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	dataRoot, configRoot := filepath.Join(root, "xdg-data"), filepath.Join(root, "xdg-config")
	localRoot, roamingRoot := filepath.Join(root, "local"), filepath.Join(root, "roaming")
	t.Setenv("XDG_DATA_HOME", dataRoot)
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("LOCALAPPDATA", localRoot)
	t.Setenv("APPDATA", roamingRoot)
	for _, c := range []struct{ system, dataRoot, configRoot string }{
		{"linux", dataRoot, configRoot},
		{"darwin", filepath.Join(home, "Library", "Application Support"), filepath.Join(home, "Library", "Application Support")},
		{"windows", localRoot, roamingRoot},
	} {
		t.Run(c.system, func(t *testing.T) {
			data, config := defaultPaths(c.system, home)
			if data != filepath.Join(c.dataRoot, "taskwarrior-tui", "tasks.json") || config != filepath.Join(c.configRoot, "taskwarrior-tui", "config.toml") {
				t.Fatal(data, config)
			}
		})
	}
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	data, config := defaultPaths("linux", home)
	if data != filepath.Join(home, ".local", "share", "taskwarrior-tui", "tasks.json") || config != filepath.Join(home, ".config", "taskwarrior-tui", "config.toml") {
		t.Fatal(data, config)
	}
}
