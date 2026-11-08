// Package config loads and validates versioned TOML settings.
package config

import (
	"bytes"
	tea "charm.land/bubbletea/v2"
	"errors"
	"fmt"
	"github.com/pelletier/go-toml/v2"
	"os"
	"strings"
	"taskwarrior/internal/style"
	"unicode"
	"unicode/utf8"
)

type Palette struct {
	Entries style.Palette `toml:"entries"`
}
type Config struct {
	Version    int                 `toml:"version"`
	Appearance style.Appearance    `toml:"appearance"`
	Keys       map[string][]string `toml:"keys"`
	Palette    Palette             `toml:"palette"`
}

var Actions = []string{"up", "down", "first", "last", "add", "toggle", "delete", "undo", "group", "color", "style", "hide_completed", "help", "quit"}

func Defaults() Config {
	return Config{Version: 1, Appearance: style.Defaults(), Palette: Palette{style.DefaultPalette()}, Keys: map[string][]string{
		"up": {"up", "k"}, "down": {"down", "j"}, "first": {"home"}, "last": {"end"}, "add": {"a"}, "toggle": {"space"}, "delete": {"d"}, "undo": {"u"}, "group": {"g"}, "color": {"c"}, "style": {"t"}, "hide_completed": {"h"}, "help": {"?"}, "quit": {"q"},
	}}
}
func (c Config) Clone() Config {
	c.Palette.Entries = append(style.Palette{}, c.Palette.Entries...)
	keys := map[string][]string{}
	for k, v := range c.Keys {
		keys[k] = append([]string{}, v...)
	}
	c.Keys = keys
	return c
}

// Use the pinned event adapter's names rather than a separate partial vocabulary.
var namedKeys = func() map[string]bool {
	names := map[string]bool{"tab": true, "space": true, "backspace": true, "esc": true, "enter": true}
	for code := tea.KeyUp; code <= tea.KeyIsoLevel5Shift; code++ {
		names[(tea.Key{Code: code}).String()] = true
	}
	return names
}()

func NormalizeKey(value string) (string, error) {
	if value == " " {
		return "space", nil
	}
	if utf8.RuneCountInString(value) == 1 {
		r, _ := utf8.DecodeRuneInString(value)
		if unicode.IsPrint(r) {
			return value, nil
		}
	}
	aliases := map[string]string{"escape": "esc", "return": "enter", "ctrl+[": "esc", "ctrl+m": "enter", "ctrl+i": "tab", "ctrl+h": "backspace", "pageup": "pgup", "pagedown": "pgdown"}
	if alias, ok := aliases[value]; ok {
		value = alias
	}
	if strings.HasPrefix(value, "shift+") {
		shifted := strings.TrimPrefix(value, "shift+")
		if utf8.RuneCountInString(shifted) == 1 {
			r, _ := utf8.DecodeRuneInString(shifted)
			if unicode.IsLetter(r) {
				return string(unicode.ToUpper(r)), nil
			}
		}
	}
	base := value
	for _, prefix := range []string{"ctrl+", "alt+", "shift+", "meta+", "hyper+", "super+"} {
		base = strings.TrimPrefix(base, prefix)
	}
	if namedKeys[base] {
		return value, nil
	}
	if base != value && utf8.RuneCountInString(base) == 1 {
		r, _ := utf8.DecodeRuneInString(base)
		if unicode.IsPrint(r) {
			return value, nil
		}
	}
	return "", fmt.Errorf("unknown key %q", value)
}
func (c *Config) Normalize() error {
	if c.Version != 1 {
		return fmt.Errorf("unsupported config version %d", c.Version)
	}
	if err := c.Appearance.Normalize(); err != nil {
		return err
	}
	if err := c.Palette.Entries.Normalize(); err != nil {
		return err
	}
	defaults := Defaults().Keys
	seen := map[string]string{}
	for action, bindings := range c.Keys {
		if _, ok := defaults[action]; !ok {
			return fmt.Errorf("unknown action %q", action)
		}
		if len(bindings) == 0 {
			return fmt.Errorf("empty bindings for %s", action)
		}
		for i, binding := range bindings {
			key, err := NormalizeKey(binding)
			if err != nil {
				return err
			}
			if key == "esc" || key == "enter" || key == "ctrl+c" {
				return fmt.Errorf("reserved key %s", key)
			}
			if owner, ok := seen[key]; ok {
				return fmt.Errorf("key %s collides between %s and %s", key, owner, action)
			}
			seen[key] = action
			c.Keys[action][i] = key
		}
	}
	for _, action := range Actions {
		if len(c.Keys[action]) == 0 {
			return fmt.Errorf("missing bindings for %s", action)
		}
	}
	return nil
}
func (c Config) ActionFor(key string) string {
	key, err := NormalizeKey(key)
	if err != nil {
		return ""
	}
	for action, bindings := range c.Keys {
		for _, binding := range bindings {
			if key == binding {
				return action
			}
		}
	}
	return ""
}
func Load(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Defaults(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	c := Defaults()
	// A pointer distinguishes an absent palette from a present table with no entries.
	wire := struct {
		Version    int                 `toml:"version"`
		Appearance style.Appearance    `toml:"appearance"`
		Keys       map[string][]string `toml:"keys"`
		Palette    *Palette            `toml:"palette"`
	}{Version: c.Version, Appearance: c.Appearance, Keys: c.Keys}
	decoder := toml.NewDecoder(bytes.NewReader(raw)).DisallowUnknownFields()
	if err = decoder.Decode(&wire); err == nil {
		c.Version, c.Appearance, c.Keys = wire.Version, wire.Appearance, wire.Keys
		if wire.Palette != nil {
			if wire.Palette.Entries == nil {
				err = errors.New("palette.entries is required")
			} else {
				c.Palette = *wire.Palette
			}
		}
	}
	if err == nil {
		err = c.Normalize()
	}
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}
func Encode(c Config) ([]byte, error) {
	c = c.Clone()
	if err := c.Normalize(); err != nil {
		return nil, err
	}
	return toml.Marshal(c)
}
