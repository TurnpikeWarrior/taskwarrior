package app

import (
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"taskwarrior/internal/storage"
	"taskwarrior/internal/tasks"
)

func selectedTask(t *testing.T, m *Model) tasks.Task {
	t.Helper()
	for _, row := range m.rows() {
		if row.ID == m.Selected {
			return row
		}
	}
	t.Fatal("selection missing")
	return tasks.Task{}
}
func TestMilestoneTwoAssignmentColorsAndRestart(t *testing.T) {
	m := newTestModel(t)
	add(t, m, "agenda #MADA")
	id := add(t, m, "second")
	if len(m.Session.Data.Groups) != 1 || m.Session.Data.Groups[0].ColorKey != "blue" {
		t.Fatal("inline color", m.Session.Data)
	}
	press(m, "g")
	if m.Mode != GroupPicker {
		t.Fatal("group picker not open")
	}
	press(m, "down")
	finish(m, press(m, "enter"))
	if selectedTask(t, m).GroupID != "mada" || m.Selected != id {
		t.Fatal("group assignment/focus")
	}
	press(m, "c")
	press(m, "end")
	finish(m, press(m, "enter"))
	if selectedTask(t, m).ColorKey != "red" {
		t.Fatal("task override")
	}
	press(m, "g")
	press(m, "home")
	finish(m, press(m, "enter"))
	task := selectedTask(t, m)
	if task.GroupID != "" || task.ColorKey != "red" {
		t.Fatal("clearing group lost override", task)
	}
	finish(m, press(m, "u"))
	press(m, "c")
	press(m, "home")
	finish(m, press(m, "enter"))
	task = selectedTask(t, m)
	if task.GroupID != "mada" || task.ColorKey != "" {
		t.Fatal("inherit", task)
	}
	finish(m, press(m, "u"))
	loaded, err := storage.Load(m.Path)
	if err != nil {
		t.Fatal(err)
	}
	restarted := New(loaded, m.Path)
	restarted.Update(tea.WindowSizeMsg{Width: 60, Height: 12})
	if !reflect.DeepEqual(loaded, m.Session.Data) || selectedTask(t, restarted).ColorKey != "red" || !strings.Contains(restarted.View().Content, "#MADA") {
		t.Fatal("restart lost groups/colors")
	}
}
func TestNewGroupNestedColorAndUndo(t *testing.T) {
	m := newTestModel(t)
	id := add(t, m, "task")
	press(m, "g")
	press(m, "end")
	press(m, "enter")
	if m.Mode != NewGroup {
		t.Fatal("new group form missing")
	}
	press(m, "enter")
	if m.PromptError == "" || m.Mode != NewGroup {
		t.Fatal("empty group accepted")
	}
	m.Update(tea.PasteMsg{Content: "Bad Name"})
	press(m, "enter")
	if m.Mode != NewGroup || len(m.Session.Data.Groups) != 0 {
		t.Fatal("bad group mutated")
	}
	m.Input.Reset()
	m.Update(tea.PasteMsg{Content: "MADA"})
	finish(m, press(m, "enter"))
	if selectedTask(t, m).GroupID != "mada" || m.Mode != Main {
		t.Fatal("group creation")
	}
	before := m.Session.Clone()
	press(m, "g")
	press(m, "tab")
	press(m, "enter")
	if m.Mode != GroupColorPicker {
		t.Fatal("group color focus/action")
	}
	press(m, "end")
	finish(m, press(m, "enter"))
	if m.Mode != GroupPicker || m.Session.Data.Groups[0].ColorKey != "red" {
		t.Fatal("group color save/return")
	}
	press(m, "esc")
	finish(m, press(m, "u"))
	if !reflect.DeepEqual(m.Session.Data, before.Data) || m.Selected != id {
		t.Fatal("group color undo")
	}
	press(m, "g")
	press(m, "shift+tab")
	press(m, "enter")
	press(m, "home")
	finish(m, press(m, "enter"))
	if m.Session.Data.Groups[0].ColorKey != "" {
		t.Fatal("default foreground not cleared")
	}
	press(m, "esc")
	finish(m, press(m, "u"))
	press(m, "g")
	press(m, "end")
	press(m, "enter")
	m.Update(tea.PasteMsg{Content: "mada"})
	finish(m, press(m, "enter"))
	if len(m.Session.Data.Groups) != 1 || m.Session.Data.Groups[0].Name != "MADA" || m.Pending {
		t.Fatal("reuse/no-op")
	}
	// The previous undo consumed the single slot; a no-op does not create another.
	finish(m, press(m, "u"))
	if selectedTask(t, m).GroupID != "mada" || len(m.Session.Data.Groups) != 1 {
		t.Fatal("no-op created undo history")
	}

}
func TestPickerMouseAndModalIsolation(t *testing.T) {
	m := newTestModel(t)
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 12})
	add(t, m, "first #MADA")
	add(t, m, "second")
	press(m, "g")
	press(m, "d")
	press(m, "q")
	press(m, "space")
	m.Update(tea.MouseClickMsg{X: 3, Y: 0, Button: tea.MouseLeft})
	if len(m.rows()) != 2 || m.Mode != GroupPicker || selectedTask(t, m).Completed {
		t.Fatal("modal isolation")
	}
	_, cmd := m.Update(tea.MouseClickMsg{X: 5, Y: 2, Button: tea.MouseLeft})
	finish(m, cmd)
	if selectedTask(t, m).GroupID != "mada" {
		t.Fatal("group mouse assignment")
	}
	press(m, "g")
	_, cmd = m.Update(tea.MouseClickMsg{X: 5, Y: 4, Button: tea.MouseLeft})
	finish(m, cmd)
	if m.Mode != GroupColorPicker {
		t.Fatal("group color mouse action")
	}
	_, cmd = m.Update(tea.MouseClickMsg{X: 5, Y: 6, Button: tea.MouseLeft})
	finish(m, cmd)
	if m.Session.Data.Groups[0].ColorKey != "red" {
		t.Fatal("color menu mouse selection")
	}
	press(m, "esc")
	press(m, "c")
	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	press(m, "end")
	finish(m, press(m, "enter"))
	if selectedTask(t, m).ColorKey != "red" {
		t.Fatal("keyboard after menu wheel")
	}
}
func TestGroupSaveFailureKeepsDraftUndoAndPicker(t *testing.T) {
	m := newTestModel(t)
	add(t, m, "saved")
	before := m.Session.Clone()
	good := m.Path
	blocker := filepath.Join(t.TempDir(), "blocker")
	os.WriteFile(blocker, []byte("x"), 0600)
	m.Path = filepath.Join(blocker, "tasks.json")
	press(m, "g")
	press(m, "end")
	press(m, "enter")
	m.Update(tea.PasteMsg{Content: "MADA"})
	cmd := press(m, "enter")
	if !m.Pending || len(m.Session.Data.Groups) != 1 {
		t.Fatal("no optimistic group")
	}
	finish(m, cmd)
	if m.Mode != NewGroup || m.Input.Value() != "MADA" || !reflect.DeepEqual(before, m.Session) || m.LastError == nil {
		t.Fatal("failure lost draft/state/undo")
	}
	m.Path = good
	finish(m, press(m, "enter"))
	if m.Mode != Main || len(m.Session.Data.Groups) != 1 {
		t.Fatal("group retry failed")
	}
	before = m.Session.Clone()
	m.Path = filepath.Join(blocker, "tasks.json")
	press(m, "c")
	press(m, "end")
	finish(m, press(m, "enter"))
	if m.Mode != TaskColorPicker || !reflect.DeepEqual(before, m.Session) {
		t.Fatal("color failure lost picker/undo")
	}
	m.Path = good
	finish(m, press(m, "enter"))
	if selectedTask(t, m).ColorKey != "red" {
		t.Fatal("color retry failed")
	}
}
func TestMissingColorPickerAndCancellation(t *testing.T) {
	m := newTestModel(t)
	id := add(t, m, "task #MADA")
	m.Session.SetTaskColor(id, "old-key")
	before := m.Session.Clone()
	press(m, "c")
	if !strings.Contains(m.View().Content, "Missing: old-key") {
		t.Fatal("missing key not shown", m.View().Content)
	}
	finish(m, press(m, "enter"))
	if m.Mode != Main || !reflect.DeepEqual(before, m.Session) {
		t.Fatal("opening missing assignment cleared reference or undo")
	}
	press(m, "g")
	press(m, "tab")
	press(m, "enter")
	press(m, "esc")
	if m.Mode != GroupPicker {
		t.Fatal("nested cancel")
	}
	press(m, "end")
	press(m, "enter")
	m.Update(tea.PasteMsg{Content: "Uncommitted"})
	m.Update(tea.WindowSizeMsg{Width: 10, Height: 2})
	m.Update(tea.MouseClickMsg{X: 3, Y: 1, Button: tea.MouseLeft})
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 6})
	if m.Input.Value() != "Uncommitted" {
		t.Fatal("resize lost draft")
	}
	press(m, "esc")
	press(m, "esc")
	if m.Mode != Main || !reflect.DeepEqual(before, m.Session) {
		t.Fatal("cancellation mutated state")
	}
}

func TestNoOpGroupReusePreservesCompositeUndo(t *testing.T) {
	m := newTestModel(t)
	add(t, m, "task")
	press(m, "g")
	press(m, "end")
	press(m, "enter")
	m.Update(tea.PasteMsg{Content: "MADA"})
	finish(m, press(m, "enter"))
	before := m.Session.Clone()
	press(m, "g")
	press(m, "end")
	press(m, "enter")
	m.Update(tea.PasteMsg{Content: "mada"})
	finish(m, press(m, "enter"))
	if !reflect.DeepEqual(before, m.Session) {
		t.Fatal("reuse replaced the undo slot")
	}
	finish(m, press(m, "u"))
	if len(m.Session.Data.Groups) != 0 || selectedTask(t, m).GroupID != "" {
		t.Fatal("composite creation undo lost")
	}
}

func TestPickerKeepsFocusedChoiceVisibleWhileSaving(t *testing.T) {
	m := newTestModel(t)
	add(t, m, "task")
	press(m, "c")
	press(m, "end")
	cmd := press(m, "enter")
	if !m.Pending {
		t.Fatal("save did not start")
	}
	found := false
	for _, line := range strings.Split(ansi.Strip(m.View().Content), "\n") {
		if strings.Contains(line, "❯") && strings.Contains(line, "Red #FF4040") {
			found = true
		}
	}
	if !found {
		t.Fatal("focused choice disappeared during save", m.View().Content)
	}
	finish(m, cmd)
}

func TestMenuScrollResizeAndMouseCoordinates(t *testing.T) {
	session := tasks.NewSession(tasks.Empty())
	for i := 0; i < 12; i++ {
		session.Add(fmt.Sprintf("task #Group%02d", i), "blue", "green")
	}
	m := New(session.Data, filepath.Join(t.TempDir(), "tasks.json"))
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 5})
	press(m, "g")
	press(m, "home")
	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	_, cmd := m.Update(tea.MouseClickMsg{X: 5, Y: 1, Button: tea.MouseLeft})
	finish(m, cmd)
	if selectedTask(t, m).GroupID != "group00" {
		t.Fatal("scrolled row coordinate mismatch")
	}
	press(m, "g")
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 3})
	press(m, "tab")
	press(m, "enter")
	press(m, "end")
	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	press(m, "down")
	_, cmd = m.Update(tea.MouseClickMsg{X: 5, Y: 2, Button: tea.MouseLeft})
	finish(m, cmd)
	group, _ := m.Session.Data.Group("group00")
	if group.ColorKey != "red" {
		t.Fatal("color hit target after resize/scroll", group)
	}
}

func TestMissingGroupColorAndEmptyPalette(t *testing.T) {
	m := newTestModel(t)
	add(t, m, "task #MADA")
	m.Session.SetGroupColor("mada", "gone")
	before := m.Session.Clone()
	press(m, "g")
	press(m, "tab")
	press(m, "enter")
	if !strings.Contains(m.View().Content, "Missing: gone") {
		t.Fatal("group missing assignment not disclosed")
	}
	finish(m, press(m, "enter"))
	press(m, "esc")
	if !reflect.DeepEqual(before, m.Session) {
		t.Fatal("missing group assignment was changed")
	}
	m.Renderer.Palette = nil
	press(m, "c")
	press(m, "end")
	finish(m, press(m, "enter"))
	if m.Mode != Main {
		t.Fatal("empty task palette unusable")
	}
	press(m, "g")
	press(m, "tab")
	press(m, "enter")
	press(m, "home")
	finish(m, press(m, "enter"))
	group, _ := m.Session.Data.Group("mada")
	if group.ColorKey != "" {
		t.Fatal("empty group palette cannot clear missing key")
	}
}
