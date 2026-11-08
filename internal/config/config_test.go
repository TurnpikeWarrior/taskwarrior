package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"taskwarrior/internal/style"
	"testing"
)

func TestDefaultsExampleAndRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	c, err := Load(path)
	if err != nil || !reflect.DeepEqual(c, Defaults()) {
		t.Fatal(c, err)
	}
	raw, _ := os.ReadFile("testdata/spec.toml")
	os.WriteFile(path, raw, 0600)
	c, err = Load(path)
	if err != nil || !reflect.DeepEqual(c, Defaults()) {
		t.Fatal("spec example", c, err)
	}
	c.Appearance.Border = true
	c.Appearance.RowGap = 2
	c.Palette.Entries = []style.Color{}
	c.Keys["add"] = []string{"n"}
	raw, err = Encode(c)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(path, raw, 0600)
	loaded, err := Load(path)
	if err != nil || !reflect.DeepEqual(c, loaded) {
		t.Fatal("round trip", loaded, err)
	}
}
func TestPartialConfigEmptyPaletteAndHexNormalization(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	os.WriteFile(path, []byte("[appearance]\nforeground='#aa55ff'\n[palette]\nentries=[]\n"), 0600)
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Appearance.Foreground != "#AA55FF" || c.Appearance.Mouse != true || c.Appearance.LeftPadding != 1 || len(c.Palette.Entries) != 0 || c.ActionFor("a") != "add" {
		t.Fatal(c)
	}
}
func TestInvalidConfigLeavesFileIntact(t *testing.T) {
	for _, raw := range []string{
		"version=9", "version=", "[appearance]\nrow_gap=3", "[appearance]\nleft_padding=-1", "[appearance]\nforeground='#fff'", "[appearance]\nsymbols='emoji'", "[appearance]\nlist_mode='other'",
		"[keys]\nadd=[]", "[keys]\nadd=['q']", "[keys]\nunknown=['x']", "[keys]\nadd=['esc']", "[keys]\nadd=['ctrl+c']", "[keys]\nadd=['enter']", "[keys]\nadd=['unknown-key']", "[keys]\nadd=['space',' ']",
		"[palette]\nentries=[{key='blue', name='Blue',hex='#123456'}, {key='other',name='blue',hex='#123456'}]", "[palette]\nentries=[{key='blue',name='X',hex='#12'}]", "[palette]", "[appearance]\nunknown=true",
	} {
		t.Run(raw, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			os.WriteFile(path, []byte(raw), 0600)
			if _, err := Load(path); err == nil || !strings.Contains(err.Error(), path) {
				t.Fatal("no path diagnostic", err)
			}
			after, _ := os.ReadFile(path)
			if string(after) != raw {
				t.Fatal("invalid config changed")
			}
		})
	}
}
func TestBindingAdapter(t *testing.T) {
	c := Defaults()
	c.Keys["add"] = []string{"n", "ctrl+n", "f2", "界"}
	if err := c.Normalize(); err != nil {
		t.Fatal(err)
	}
	for _, key := range c.Keys["add"] {
		if c.ActionFor(key) != "add" {
			t.Fatal(key)
		}
	}
	if c.ActionFor("a") != "" || c.ActionFor(" ") != "toggle" {
		t.Fatal("default replacement or space mapping failed")
	}
}

func TestShiftedLetterBindingsCanonicalizeToAdapterText(t *testing.T) {
	c := Defaults()
	c.Keys["add"] = []string{"shift+n"}
	if err := c.Normalize(); err != nil {
		t.Fatal(err)
	}
	if c.Keys["add"][0] != "N" || c.ActionFor("N") != "add" {
		t.Fatal("shifted binding cannot match adapter text", c.Keys["add"])
	}
	c = Defaults()
	c.Keys["add"] = []string{"shift+a"}
	c.Keys["color"] = []string{"A"}
	if c.Normalize() == nil {
		t.Fatal("alias collision accepted")
	}
}

func TestNamedAdapterKeysBeyondNavigation(t *testing.T) {
	c := Defaults()
	c.Keys["add"] = []string{"mul", "begin", "ctrl+f12"}
	if err := c.Normalize(); err != nil {
		t.Fatal("recognized adapter key rejected", err)
	}
	for _, key := range c.Keys["add"] {
		if c.ActionFor(key) != "add" {
			t.Fatal(key)
		}
	}
}
