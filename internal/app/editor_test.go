package app

import (
	tea "charm.land/bubbletea/v2"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"taskwarrior/internal/config"
	"taskwarrior/internal/storage"
	"taskwarrior/internal/style"
	"testing"
)

func openPaletteEdit(t *testing.T, m *Model) {
	t.Helper()
	press(m, "t")
	press(m, "down")
	press(m, "enter")
	press(m, "enter")
}
func editPaletteForm(m *Model, name, hex string) {
	press(m, "ctrl+u")
	m.Update(tea.PasteMsg{Content: name})
	press(m, "tab")
	press(m, "ctrl+u")
	m.Update(tea.PasteMsg{Content: hex})
	press(m, "tab")
	press(m, "enter")
}
func saveEditor(m *Model) tea.Cmd {
	press(m, "esc")
	press(m, "end")
	press(m, "up")
	return press(m, "enter")
}
func TestStyleEditorCancelDraftAndKeyboardSave(t *testing.T) {
	m := newTestModel(t)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	add(t, m, "agenda #MADA")
	before := m.Session.Clone()
	raw, _ := os.ReadFile(m.Path)
	openPaletteEdit(t, m)
	editPaletteForm(m, "Violet", "#aa55ff")
	if m.editor.Draft.Palette.Entries[0].Hex != "#AA55FF" || m.Renderer.Palette[0].Hex != "#268BFF" {
		t.Fatal("draft leaked or was not updated")
	}
	press(m, "esc")
	press(m, "esc")
	if m.Mode != Main || m.Renderer.Palette[0].Hex != "#268BFF" || !reflect.DeepEqual(m.Session, before) {
		t.Fatal("cancel changed saved state")
	}
	if _, err := os.Stat(m.ConfigPath); !os.IsNotExist(err) {
		t.Fatal("cancel wrote config")
	}
	openPaletteEdit(t, m)
	editPaletteForm(m, "Violet", "#aa55ff")
	cmd := saveEditor(m)
	if !m.Pending || m.Renderer.Palette[0].Hex != "#268BFF" {
		t.Fatal("config promoted before atomic save")
	}
	finish(m, cmd)
	if m.Mode != Main || m.Renderer.Palette[0].Key != "blue" || m.Renderer.Palette[0].Hex != "#AA55FF" || !reflect.DeepEqual(m.Session, before) {
		t.Fatal("palette save corrupted references or undo")
	}
	after, _ := os.ReadFile(m.Path)
	if string(raw) != string(after) {
		t.Fatal("palette save wrote JSON")
	}
	cfg, err := config.Load(m.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	data, err := storage.Load(m.Path)
	if err != nil {
		t.Fatal(err)
	}
	restarted := NewConfigured(data, m.Path, m.ConfigPath, cfg)
	if style.ResolveColor(restarted.rows()[0], data.Groups, restarted.Renderer.Palette) != "#AA55FF" {
		t.Fatal("saved HEX not applied after restart")
	}
}
func TestEditorRemoveReferencedColorAndRestore(t *testing.T) {
	m := newTestModel(t)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	add(t, m, "agenda #MADA")
	press(m, "t")
	press(m, "down")
	press(m, "enter")
	press(m, "end")
	press(m, "up")
	press(m, "up")
	press(m, "enter")
	if _, ok := m.editor.Draft.Palette.Entries.Find("blue"); ok {
		t.Fatal("remove action failed")
	}
	finish(m, saveEditor(m))
	if style.ResolveColor(m.rows()[0], m.Session.Data.Groups, m.Renderer.Palette) != "" || m.Session.Data.Groups[0].ColorKey != "blue" {
		t.Fatal("removal did not preserve missing reference")
	}
	press(m, "t")
	press(m, "down")
	press(m, "enter")
	press(m, "end")
	press(m, "up")
	press(m, "enter")
	finish(m, saveEditor(m))
	if style.ResolveColor(m.rows()[0], m.Session.Data.Groups, m.Renderer.Palette) != "#268BFF" {
		t.Fatal("restore did not restore reference")
	}
}
func TestEditorAppearancePreviewAndFailedSave(t *testing.T) {
	m := newTestModel(t)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	add(t, m, "task")
	press(m, "t")
	press(m, "enter")
	press(m, "enter")
	press(m, "tab")
	press(m, "enter")
	if m.editor.Draft.Appearance.ListMode != "numbered" || m.editor.Draft.Appearance.RowGap != 1 || m.Renderer.Appearance.ListMode != "plain" || !strings.Contains(m.View().Content, "1.") {
		t.Fatal("appearance preview not live")
	}
	good := m.ConfigPath
	blocker := filepath.Join(t.TempDir(), "blocker")
	os.WriteFile(blocker, []byte("x"), 0600)
	m.ConfigPath = filepath.Join(blocker, "config.toml")
	finish(m, saveEditor(m))
	if m.Mode != StyleEditor || m.editor.Draft.Appearance.RowGap != 1 || m.Renderer.Appearance.RowGap != 0 || m.LastError == nil {
		t.Fatal("failed save lost draft or promoted it")
	}
	m.ConfigPath = good
	press(m, "end")
	press(m, "up")
	finish(m, press(m, "enter"))
	if m.Mode != Main || m.Renderer.Appearance.RowGap != 1 {
		t.Fatal("retry did not promote draft")
	}
}
func TestEditorFormValidationResizeAndModalIsolation(t *testing.T) {
	m := newTestModel(t)
	add(t, m, "task")
	openPaletteEdit(t, m)
	editPaletteForm(m, "Green", "#fff")
	if m.editor.Page != editorForm || m.editor.Error == "" || m.editor.Draft.Palette.Entries[0].Name != "Blue" {
		t.Fatal("invalid palette edited draft")
	}
	m.Update(tea.WindowSizeMsg{Width: 10, Height: 2})
	m.Update(tea.MouseClickMsg{X: 3, Y: 1, Button: tea.MouseLeft})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	press(m, "esc")
	press(m, "d")
	press(m, "u")
	press(m, "q")
	if m.Mode != StyleEditor || len(m.rows()) != 1 {
		t.Fatal("editor leaked shortcuts")
	}
	press(m, "esc")
	press(m, "esc")
	if m.Mode != Main {
		t.Fatal("Esc did not cancel editor")
	}
}

func TestEditorMouseFieldFocusAndSaveAtTinyHeight(t *testing.T) {
	m := newTestModel(t)
	add(t, m, "task")
	openPaletteEdit(t, m)
	m.Update(tea.MouseClickMsg{X: 5, Y: 2, Button: tea.MouseLeft})
	press(m, "ctrl+u")
	m.Update(tea.PasteMsg{Content: "#ABCDEF"})
	press(m, "tab")
	press(m, "enter")
	if m.editor.Draft.Palette.Entries[0].Name != "Blue" || m.editor.Draft.Palette.Entries[0].Hex != "#ABCDEF" {
		t.Fatal("form click did not focus HEX")
	}
	press(m, "esc")
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 3})
	press(m, "home")
	press(m, "down")
	press(m, "down")
	cmd := press(m, "enter")
	if !strings.Contains(m.View().Content, "❯ Save") {
		t.Fatal("Save focus hidden while publishing", m.View().Content)
	}
	finish(m, cmd)
}
