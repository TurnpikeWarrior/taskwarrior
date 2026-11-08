package app

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"taskwarrior/internal/storage"
	"taskwarrior/internal/tasks"
)

func press(m *Model, key string) tea.Cmd {
	k := tea.Key{Text: key, Code: []rune(key)[0]}
	switch key {
	case "enter":
		k = tea.Key{Code: tea.KeyEnter}
	case "esc":
		k = tea.Key{Code: tea.KeyEsc}
	case "down":
		k = tea.Key{Code: tea.KeyDown}
	case "up":
		k = tea.Key{Code: tea.KeyUp}
	case "home":
		k = tea.Key{Code: tea.KeyHome}
	case "end":
		k = tea.Key{Code: tea.KeyEnd}
	case "space":
		k = tea.Key{Text: " ", Code: tea.KeySpace}
	case "tab":
		k = tea.Key{Code: tea.KeyTab}
	case "shift+tab":
		k = tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}
	case "ctrl+u":
		k = tea.Key{Code: 'u', Mod: tea.ModCtrl}
	case "ctrl+c":
		k = tea.Key{Code: 'c', Mod: tea.ModCtrl}
	}
	_, cmd := m.Update(tea.KeyPressMsg(k))
	return cmd
}
func finish(m *Model, cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	_, next := m.Update(cmd())
	return next
}
func newTestModel(t *testing.T) *Model {
	t.Helper()
	m := New(tasks.Empty(), filepath.Join(t.TempDir(), "tasks.json"))
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 6})
	return m
}
func add(t *testing.T, m *Model, title string) string {
	t.Helper()
	press(m, "a")
	m.Update(tea.PasteMsg{Content: title})
	finish(m, press(m, "enter"))
	if m.LastError != nil {
		t.Fatal(m.LastError)
	}
	return m.Selected
}

func TestMilestoneKeyboardRestart(t *testing.T) {
	m := newTestModel(t)
	a := add(t, m, "one")
	b := add(t, m, "two")
	c := add(t, m, "three")
	if m.Selected != c {
		t.Fatal("new task not selected")
	}
	press(m, "down")
	if m.Selected != b {
		t.Fatal("navigation")
	}
	finish(m, press(m, "space"))
	if m.Selected != b || m.rows()[2].ID != b {
		t.Fatal("completion focus/order")
	}
	finish(m, press(m, "space"))
	if m.rows()[1].ID != b {
		t.Fatal("reopen creation order")
	}
	finish(m, press(m, "d"))
	if m.Selected != a {
		t.Fatal("delete fallback selection")
	}
	press(m, "home")
	finish(m, press(m, "u"))
	if m.Selected != b {
		t.Fatal("undo selection")
	}
	before := m.Session.Data.Clone()
	finish(m, press(m, "u"))
	if !reflect.DeepEqual(before, m.Session.Data) {
		t.Fatal("undo repeated")
	}
	loaded, err := storage.Load(m.Path)
	if err != nil {
		t.Fatal(err)
	}
	restarted := New(loaded, m.Path)
	if !reflect.DeepEqual(loaded, before) || restarted.Selected != c || restarted.Session.Previous != nil {
		t.Fatal("restart mismatch")
	}
}
func TestModalIsolationAndValidation(t *testing.T) {
	m := newTestModel(t)
	id := add(t, m, "keep")
	press(m, "a")
	for _, key := range []string{"q", "d", "u", "space", "a"} {
		press(m, key)
	}
	if m.Input.Value() != "qdu a" || len(m.rows()) != 1 {
		t.Fatal(m.Input.Value())
	}
	press(m, "esc")
	press(m, "?")
	press(m, "d")
	if len(m.rows()) != 1 || m.Selected != id {
		t.Fatal("help leaked key")
	}
	m.Update(tea.MouseClickMsg{X: 3, Y: 0, Button: tea.MouseLeft})
	if m.Session.Data.Tasks[0].Completed {
		t.Fatal("modal leaked mouse")
	}
	press(m, "esc")
	press(m, "a")
	press(m, "enter")
	if m.Mode != Adding || m.PromptError == "" {
		t.Fatal("empty title accepted")
	}
	m.Update(tea.PasteMsg{Content: "bad\x1btext"})
	if m.PromptError == "" || m.Input.Value() != "" {
		t.Fatal("paste control sanitized silently")
	}
	m.Update(tea.PasteMsg{Content: "hello\nworld\t!"})
	finish(m, press(m, "enter"))
	if m.rows()[0].Text != "hello world !" {
		t.Fatal(m.rows())
	}
}
func TestMouseParityScrollingResize(t *testing.T) {
	m := newTestModel(t)
	for _, title := range []string{"one", "two", "three", "four", "five", "six", "seven", "eight"} {
		add(t, m, title)
	}
	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if m.Offset == 0 {
		t.Fatal("wheel did not scroll")
	}
	target := m.rows()[m.Offset].ID
	_, cmd := m.Update(tea.MouseClickMsg{X: 3, Y: 0, Button: tea.MouseLeft})
	finish(m, cmd)
	if m.Selected != target {
		t.Fatal("wrong scrolled task")
	}
	completed := 0
	for _, row := range m.rows() {
		if row.Completed {
			completed++
		}
	}
	if completed != 1 {
		t.Fatal("checkbox did not toggle once")
	}
	m.Update(tea.MouseReleaseMsg{X: 3, Y: 0, Button: tea.MouseLeft})
	if !m.rows()[len(m.rows())-1].Completed {
		t.Fatal("release toggled")
	}
	press(m, "home")
	m.Update(tea.WindowSizeMsg{Width: 20, Height: 3})
	if m.Offset != 0 {
		t.Fatal("keyboard did not bring selection into view")
	}
	m.Update(tea.MouseClickMsg{X: 7, Y: 1, Button: tea.MouseLeft})
	if m.Selected != m.rows()[1].ID {
		t.Fatal("body click after resize")
	}
	m.Update(tea.WindowSizeMsg{Width: 10, Height: 2})
	before := m.Selected
	m.Update(tea.MouseClickMsg{X: 3, Y: 0, Button: tea.MouseLeft})
	if m.Selected != before {
		t.Fatal("tiny targets enabled")
	}
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 6})
	m.Renderer.Appearance.Mouse = false
	m.Update(tea.MouseClickMsg{X: 3, Y: 0, Button: tea.MouseLeft})
	if m.Session.Data.Tasks[0].Completed {
		t.Fatal("disabled mouse changed task")
	}
}
func TestFailureRollsBackDraftAndUndo(t *testing.T) {
	m := newTestModel(t)
	id := add(t, m, "saved")
	before := m.Session.Clone()
	raw, _ := os.ReadFile(m.Path)
	// A regular file where a directory is required causes a real pre-replacement failure.
	blocker := filepath.Join(t.TempDir(), "blocker")
	os.WriteFile(blocker, []byte("x"), 0600)
	goodPath := m.Path
	m.Path = filepath.Join(blocker, "tasks.json")
	press(m, "a")
	m.Update(tea.PasteMsg{Content: "retry me"})
	cmd := press(m, "enter")
	if !m.Pending || len(m.rows()) != 2 {
		t.Fatal("no optimistic state")
	}
	press(m, "d")
	press(m, "space")
	if len(m.rows()) != 2 {
		t.Fatal("mutation gate")
	}
	finish(m, cmd)
	if m.Pending || m.Selected != id || !reflect.DeepEqual(m.Session, before) || m.Input.Value() != "retry me" || m.Mode != Adding || m.LastError == nil {
		t.Fatal("rollback/draft/undo failure", m)
	}
	after, _ := os.ReadFile(goodPath)
	if string(raw) != string(after) {
		t.Fatal("previous file changed")
	}
	m.Path = goodPath
	finish(m, press(m, "enter"))
	if m.Mode != Main || len(m.rows()) != 2 {
		t.Fatal("retry failed")
	}
}
func TestQuitWaitsForSave(t *testing.T) {
	m := newTestModel(t)
	press(m, "a")
	m.Update(tea.PasteMsg{Content: "save before quit"})
	cmd := press(m, "enter")
	if press(m, "ctrl+c") != nil || !m.Quitting {
		t.Fatal("quit did not wait")
	}
	next := finish(m, cmd)
	if next == nil {
		t.Fatal("quit did not resume")
	}
	if _, ok := next().(tea.QuitMsg); !ok {
		t.Fatal("not quit")
	}
	d, err := storage.Load(m.Path)
	if err != nil || len(d.Tasks) != 1 {
		t.Fatal(d, err)
	}
}
func TestHelpAndNoPersistentChrome(t *testing.T) {
	m := newTestModel(t)
	add(t, m, "task")
	if strings.Contains(m.View().Content, "TaskWarrior") {
		t.Fatal("persistent header")
	}
	press(m, "?")
	if !strings.Contains(m.View().Content, "Esc") {
		t.Fatal("missing help escape")
	}
}

func TestPromptFitsWithoutSpuriousEllipsis(t *testing.T) {
	m := newTestModel(t)
	press(m, "a")
	m.Update(tea.PasteMsg{Content: "short title"})
	if strings.Contains(m.View().Content, "…") {
		t.Fatal("prompt clips empty padding", m.View().Content)
	}
}

func TestSaveFailureShowsCauseAndPath(t *testing.T) {
	m := newTestModel(t)
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 12})
	blocker := filepath.Join(t.TempDir(), "blocker")
	os.WriteFile(blocker, []byte("x"), 0600)
	m.Path = filepath.Join(blocker, "tasks.json")
	press(m, "a")
	m.Update(tea.PasteMsg{Content: "draft"})
	finish(m, press(m, "enter"))
	view := m.View().Content
	if !strings.Contains(view, "Save failed:") || (runtime.GOOS != "windows" && !strings.Contains(view, "not a directory")) {
		t.Fatal("failure cause clipped", view)
	}
	if !strings.Contains(strings.ReplaceAll(view, "\n ", ""), m.Path) {
		t.Fatal("failure path clipped", view)
	}
}

func TestPublishedSnapshotSurvivesDurabilityWarning(t *testing.T) {
	m := newTestModel(t)
	press(m, "a")
	m.Update(tea.PasteMsg{Content: "published"})
	cmd := press(m, "enter")
	result := cmd().(saveResult)
	if result.Err != nil || !result.Published {
		t.Fatal(result)
	}
	// The storage layer reports this after replacement when directory sync fails.
	result.Err = errors.New("injected directory sync failure")
	m.Update(result)
	if len(m.rows()) != 1 || m.Mode != Main || m.LastError == nil || !strings.Contains(m.View().Content, "crash durability uncertain") {
		t.Fatal("published snapshot rolled back or warning missing")
	}
	saved, err := storage.Load(m.Path)
	if err != nil || !reflect.DeepEqual(saved, m.Session.Data) {
		t.Fatal("published state diverged", err)
	}
	finish(m, press(m, "u"))
	if len(m.rows()) != 0 || m.LastError != nil {
		t.Fatal("warning lost undo slot or successful recovery")
	}
}

func TestVisibleRowsTrackDomainChangesAndRestoredSnapshots(t *testing.T) {
	m := newTestModel(t)
	id := add(t, m, "first #MADA")
	m.rows()
	m.Session.SetTaskColor(id, "red")
	if m.rows()[0].ColorKey != "red" {
		t.Fatal("stale task color")
	}
	snapshot := m.Session.Clone()
	m.Session.AssignGroup(id, "")
	if m.rows()[0].GroupID != "" {
		t.Fatal("stale group")
	}
	m.Session = snapshot
	if m.rows()[0].GroupID != "mada" {
		t.Fatal("stale restored snapshot")
	}
	m.Session.Toggle(id)
	m.Renderer.Appearance.HideCompleted = true
	if len(m.rows()) != 0 {
		t.Fatal("stale hidden rows")
	}
	m.Session.Undo()
	if len(m.rows()) != 1 || m.rows()[0].Completed {
		t.Fatal("stale undo")
	}
	m.Session.Delete(id)
	if len(m.rows()) != 0 {
		t.Fatal("stale deletion")
	}
	m.Session.Add("second")
	if len(m.rows()) != 1 || m.rows()[0].Text != "second" {
		t.Fatal("stale insertion")
	}
}
