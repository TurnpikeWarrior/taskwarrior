package app

import (
	tea "charm.land/bubbletea/v2"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"taskwarrior/internal/config"
	"testing"
)

func TestBindingsHideVisibilityAndSeparateFiles(t *testing.T) {
	m := newTestModel(t)
	id := add(t, m, "task")
	finish(m, press(m, "space"))
	prior := m.Session.Clone()
	raw, _ := os.ReadFile(m.Path)
	c := config.Defaults()
	c.Keys["add"] = []string{"n"}
	c.Keys["hide_completed"] = []string{"v"}
	c.Appearance.Footer = true
	m = NewConfigured(m.Session.Data, m.Path, filepath.Join(t.TempDir(), "config.toml"), c)
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 12})
	m.Session = prior.Clone()
	press(m, "a")
	if m.Mode != Main {
		t.Fatal("replaced binding still active")
	}
	finish(m, press(m, "v"))
	if len(m.rows()) != 0 || m.Selected != "" || !strings.Contains(m.View().Content, "v: Show completed") || !strings.Contains(m.View().Content, "n") {
		t.Fatal("hidden state hints", m.View().Content)
	}
	after, _ := os.ReadFile(m.Path)
	if string(after) != string(raw) || !reflect.DeepEqual(m.Session, prior) {
		t.Fatal("visibility wrote tasks or consumed undo")
	}
	loaded, err := config.Load(m.ConfigPath)
	if err != nil || !loaded.Appearance.HideCompleted || loaded.Keys["add"][0] != "n" {
		t.Fatal(loaded, err)
	}
	finish(m, press(m, "v"))
	if m.Selected != id {
		t.Fatal("show selection")
	}
	press(m, "?")
	if !strings.Contains(m.View().Content, "n") {
		t.Fatal("help ignores bindings")
	}
	press(m, "esc")
	press(m, "n")
	if m.Mode != Adding {
		t.Fatal("new binding inert")
	}
}
func TestHideFailedWriteRollsBackPreferenceSelectionAndUndo(t *testing.T) {
	m := newTestModel(t)
	add(t, m, "task")
	finish(m, press(m, "space"))
	before := m.Session.Clone()
	selected := m.Selected
	blocker := filepath.Join(t.TempDir(), "blocker")
	os.WriteFile(blocker, []byte("x"), 0600)
	m.ConfigPath = filepath.Join(blocker, "config.toml")
	cmd := press(m, "h")
	if !m.Pending || !m.Renderer.Appearance.HideCompleted {
		t.Fatal("no optimistic preference")
	}
	finish(m, cmd)
	if m.Renderer.Appearance.HideCompleted || m.Selected != selected || !reflect.DeepEqual(before, m.Session) || m.LastError == nil {
		t.Fatal("preference rollback")
	}
	if !strings.Contains(m.statusText(), m.ConfigPath) {
		t.Fatal("wrong error path")
	}
}
func TestStyledMouseDeleteAndGapHitTargets(t *testing.T) {
	m := newTestModel(t)
	add(t, m, "one")
	add(t, m, "two")
	m.Renderer.Appearance.Border = true
	m.Renderer.Appearance.RowGap = 1
	m.Renderer.Appearance.ListMode = "numbered"
	m.Renderer.Appearance.MouseDelete = true
	m.Update(tea.WindowSizeMsg{Width: 50, Height: 12})
	layout := m.listRenderer().List(m.rows(), m.Selected, m.Offset, m.Width, m.Height, "", m.Session.Data.Groups...)
	m.Update(tea.MouseClickMsg{X: 5, Y: 2, Button: tea.MouseLeft})
	if len(m.rows()) != 2 {
		t.Fatal("gap mutated")
	}
	target := layout.Targets[0]
	_, cmd := m.Update(tea.MouseClickMsg{X: target.DeleteStart, Y: target.Y, Button: tea.MouseLeft})
	finish(m, cmd)
	if len(m.rows()) != 1 || m.rows()[0].Text != "one" {
		t.Fatal("row delete coordinate")
	}
}

func TestNarrowEmptyStateStillShowsEffectiveAddKey(t *testing.T) {
	m := newTestModel(t)
	m.Settings.Keys["add"] = []string{"n"}
	m.Update(tea.WindowSizeMsg{Width: 12, Height: 3})
	if !strings.Contains(m.View().Content, "n") {
		t.Fatal("add shortcut clipped out of empty state", m.View().Content)
	}
}
