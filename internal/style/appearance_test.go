package style

import (
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"strings"
	"taskwarrior/internal/tasks"
	"testing"
)

func TestNumberingGapsBorderAndMouseDelete(t *testing.T) {
	a := Defaults()
	a.ListMode = "numbered"
	a.RowGap = 1
	a.Border = true
	a.MouseDelete = true
	r := Renderer{Appearance: a, NoColor: true}
	rows := []tasks.Task{{ID: "a", Text: "first"}, {ID: "b", Text: "done", Completed: true}}
	l := r.List(rows, "b", 0, 30, 8, "")
	lines := strings.Split(l.Content, "\n")
	if !strings.Contains(lines[1], "1. ☐ first") || !strings.Contains(lines[3], "2. ☑ done") {
		t.Fatal(lines)
	}
	if id, _ := l.Hit(4, 2); id != "" {
		t.Fatal("gap is clickable")
	}
	if id, action := l.HitAction(7, 3); id != "b" || action != "toggle" {
		t.Fatal(id, action, l.Content)
	}
	target := l.Targets[1]
	if id, action := l.HitAction(target.DeleteStart, 3); id != "b" || action != "delete" {
		t.Fatal(id, action)
	}
	if id, _ := l.Hit(0, 1); id != "" {
		t.Fatal("border clickable")
	}
	for _, line := range lines {
		if ansi.StringWidth(line) > 30 {
			t.Fatal("overflow", line)
		}
	}
}
func TestThemeCompletedAndSelectionColors(t *testing.T) {
	a := Defaults()
	a.Foreground = "#AA55FF"
	a.CompletedForeground = "#123456"
	a.SelectionForeground = "#ABCDEF"
	a.Background = "#010203"
	r := Renderer{Appearance: a, Palette: DefaultPalette()}
	l := r.List([]tasks.Task{{ID: "a", Text: "active", ColorKey: "gone"}, {ID: "b", Text: "done", Completed: true, ColorKey: "red"}}, "a", 0, 40, 5, "")
	if !strings.Contains(l.Content, "38;2;170;85;255") || !strings.Contains(l.Content, "38;2;18;52;86") || !strings.Contains(l.Content, "38;2;171;205;239") || !strings.Contains(l.Content, "48;2;1;2;3") {
		t.Fatalf("missing theme colors %q", l.Content)
	}
	if strings.Contains(l.Content, "\x1b[2;") {
		t.Fatal("configured completed color still faint")
	}
}
func TestASCIIBulletsNarrowAndFooter(t *testing.T) {
	a := Defaults()
	a.Symbols = "ascii"
	a.ListMode = "bulleted"
	a.Border = true
	a.Footer = true
	a.LeftPadding = 4
	a.MouseDelete = true
	r := Renderer{Appearance: a, NoColor: true, FooterText: "n add · q quit"}
	l := r.List([]tasks.Task{{ID: "a", Text: "very long title", GroupID: "g"}}, "a", 0, 12, 3, "", tasks.Group{ID: "g", Name: "Group"})
	for _, line := range strings.Split(l.Content, "\n") {
		if ansi.StringWidth(line) > 12 {
			t.Fatal(line)
		}
	}
	if !strings.Contains(l.Content, "[ ]") || !strings.Contains(l.Content, ">") || strings.Contains(l.Content, "#Group") {
		t.Fatal("indicators lost", l.Content)
	}
	l = r.List(nil, "", 0, 40, 7, "")
	if !strings.Contains(l.Content, "n add") || strings.Contains(l.Content, "┌") {
		t.Fatal(l.Content)
	}
}
func TestPaletteEditRemoveRestore(t *testing.T) {
	p := DefaultPalette()
	p, err := p.Put("blue", "Violet", "#aa55ff")
	if err != nil {
		t.Fatal(err)
	}
	if p[0].Key != "blue" || p[0].Hex != "#AA55FF" {
		t.Fatal(p)
	}
	if _, err = p.Put("", "violet", "#123456"); err == nil {
		t.Fatal("duplicate label accepted")
	}
	p, err = p.Put("", "Custom", "#abcdef")
	if err != nil || !strings.HasPrefix(p[len(p)-1].Key, "custom-") {
		t.Fatal(p, err)
	}
	p = p.Remove("blue")
	if _, ok := p.Find("blue"); ok {
		t.Fatal("entry not removed")
	}
	p = DefaultPalette()
	if p[0].Key != "blue" || p[0].Hex != "#268BFF" {
		t.Fatal("defaults not restored")
	}
}

func TestAllListModesAndDensitiesShareCheckboxColumns(t *testing.T) {
	rows := []tasks.Task{}
	for i := 0; i < 12; i++ {
		rows = append(rows, tasks.Task{ID: fmt.Sprint(i), Text: "task", Completed: i > 9})
	}
	for _, mode := range []string{"plain", "numbered", "bulleted"} {
		for gap := 0; gap <= 2; gap++ {
			a := Defaults()
			a.ListMode = mode
			a.RowGap = gap
			r := Renderer{Appearance: a, NoColor: true}
			l := r.List(rows, "0", 0, 40, 40, "")
			if len(l.Targets) != 12 {
				t.Fatal("rows clipped unexpectedly", mode, gap)
			}
			for i, target := range l.Targets {
				if target.Y != i*(gap+1) || target.CheckboxStart != l.Targets[0].CheckboxStart {
					t.Fatal("gap/column mismatch", mode, gap, target)
				}
			}
			if mode == "numbered" && !strings.Contains(l.Content, "12. ☑") {
				t.Fatal("numbering restarted in completed section")
			}
		}
	}
}
