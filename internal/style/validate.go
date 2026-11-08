package style

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"regexp"
	"strings"
	"taskwarrior/internal/tasks"
)

var hexPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func NormalizeColor(value string, terminal bool) (string, error) {
	if terminal && value == "terminal" {
		return value, nil
	}
	if !hexPattern.MatchString(value) {
		return "", errors.New("color must be # followed by six HEX digits")
	}
	return strings.ToUpper(value), nil
}
func (a *Appearance) Normalize() error {
	if a.ListMode != "plain" && a.ListMode != "numbered" && a.ListMode != "bulleted" {
		return errors.New("invalid list_mode")
	}
	if a.RowGap < 0 || a.RowGap > 2 || a.LeftPadding < 0 || a.LeftPadding > 4 {
		return errors.New("row_gap must be 0–2 and left_padding 0–4")
	}
	if a.Symbols != "unicode" && a.Symbols != "ascii" {
		return errors.New("invalid symbols")
	}
	for _, field := range []*string{&a.Foreground, &a.Background, &a.CompletedForeground, &a.SelectionForeground} {
		value, err := NormalizeColor(*field, true)
		if err != nil {
			return err
		}
		*field = value
	}
	return nil
}
func (p Palette) Normalize() error {
	keys, names := map[string]bool{}, map[string]bool{}
	for i := range p {
		c := &p[i]
		if err := tasks.ValidateText(c.Key); err != nil {
			return fmt.Errorf("palette key: %w", err)
		}
		if err := tasks.ValidateText(c.Name); err != nil {
			return fmt.Errorf("palette name: %w", err)
		}
		if strings.TrimSpace(c.Name) != c.Name {
			return errors.New("palette names cannot have surrounding whitespace")
		}
		if keys[c.Key] || names[strings.ToLower(c.Name)] {
			return errors.New("palette keys and case-insensitive names must be unique")
		}
		keys[c.Key] = true
		names[strings.ToLower(c.Name)] = true
		value, err := NormalizeColor(c.Hex, false)
		if err != nil {
			return err
		}
		c.Hex = value
	}
	return nil
}
func (p Palette) Put(key, name, hex string) (Palette, error) {
	out := append(Palette{}, p...)
	if key == "" {
		out = append(out, Color{Key: "custom-" + uuid.NewString(), Name: name, Hex: hex})
	} else {
		found := false
		for i := range out {
			if out[i].Key == key {
				out[i].Name = name
				out[i].Hex = hex
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New("palette entry not found")
		}
	}
	if err := out.Normalize(); err != nil {
		return nil, err
	}
	return out, nil
}
func (p Palette) Remove(key string) Palette {
	out := Palette{}
	for _, c := range p {
		if c.Key != key {
			out = append(out, c)
		}
	}
	return out
}
