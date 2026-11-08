package app

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"fmt"
	"taskwarrior/internal/style"
)

type choiceState struct {
	Index, Offset int
	Action        bool
}
type choice struct{ Label, Value string }

func (m *Model) isPrompt() bool { return m.Mode == Adding || m.Mode == NewGroup }
func (m *Model) isPicker() bool {
	return m.Mode == GroupPicker || m.Mode == TaskColorPicker || m.Mode == GroupColorPicker
}
func (m *Model) taskGroupAndColor() (group, color string) {
	for _, row := range m.Session.Data.Tasks {
		if row.ID == m.Selected {
			return row.GroupID, row.ColorKey
		}
	}
	return "", ""
}
func (m *Model) groupItems() []choice {
	items := []choice{{Label: "Ungrouped"}}
	for _, g := range m.Session.Data.SortedGroups() {
		items = append(items, choice{Label: "#" + g.Name, Value: g.Name})
	}
	return append(items, choice{Label: "New group", Value: "!new"})
}
func (m *Model) colorItems() []choice {
	_, current := m.taskGroupAndColor()
	first := "Inherit"
	if m.Mode == GroupColorPicker {
		first = "Default foreground"
		g, _ := m.Session.Data.Group(m.groupColorID)
		current = g.ColorKey
	}
	items := []choice{{Label: first}}
	for _, color := range m.Renderer.Palette {
		label := fmt.Sprintf("%s %s", color.Name, color.Hex)
		if !m.Renderer.NoColor {
			label = lipgloss.NewStyle().Foreground(lipgloss.Color(color.Hex)).Render("■") + " " + label
		}
		items = append(items, choice{Label: label, Value: color.Key})
	}
	if _, ok := m.Renderer.Palette.Find(current); current != "" && !ok {
		items = append(items, choice{Label: "Missing: " + current + " (default foreground)", Value: current})
	}
	return items
}
func (m *Model) openPrompt(mode Mode) tea.Cmd {
	m.Mode = mode
	m.PromptError = ""
	m.Input.Reset()
	m.Input.Placeholder = "Task title"
	if mode == NewGroup {
		m.Input.Placeholder = "Group name"
	}
	return m.Input.Focus()
}
func (m *Model) openGroups() {
	m.Mode = GroupPicker
	m.PromptError = ""
	m.groupMenu = choiceState{}
	groupID, _ := m.taskGroupAndColor()
	for i, item := range m.groupItems() {
		if group, ok := m.Session.Data.Group(groupID); ok && item.Value == group.Name {
			m.groupMenu.Index = i
		}
	}
	m.followPicker()
}
func (m *Model) openTaskColors() {
	m.Mode = TaskColorPicker
	m.PromptError = ""
	_, key := m.taskGroupAndColor()
	m.selectColor(key)
}
func (m *Model) openGroupColors() {
	items := m.groupItems()
	i := m.groupMenu.Index
	if i <= 0 || i >= len(items)-1 {
		return
	}
	for _, group := range m.Session.Data.Groups {
		if group.Name == items[i].Value {
			m.groupColorID = group.ID
			m.Mode = GroupColorPicker
			m.PromptError = ""
			m.selectColor(group.ColorKey)
			return
		}
	}
}
func (m *Model) selectColor(key string) {
	m.colorMenu = choiceState{}
	for i, item := range m.colorItems() {
		if item.Value == key {
			m.colorMenu.Index = i
			break
		}
	}
	m.followPicker()
}
func (m *Model) completeDialog(operation string) {
	switch operation {
	case "add", "group-new":
		m.Mode = Main
		m.Input.Reset()
		m.Input.Blur()
		m.PromptError = ""
	case "group", "task-color":
		m.Mode = Main
		m.PromptError = ""
	case "group-color":
		m.Mode = GroupPicker
		m.PromptError = ""
		m.followPicker()
	}
}
func (m *Model) pickerItems() ([]choice, *choiceState, string, string) {
	if m.Mode == GroupPicker {
		items := m.groupItems()
		action := ""
		if m.groupMenu.Index > 0 && m.groupMenu.Index < len(items)-1 {
			action = "Group color: " + items[m.groupMenu.Index].Label + " (Tab)"
		}
		return items, &m.groupMenu, "Groups · Enter: assign · Esc: back", action
	}
	title := "Task color · Enter: apply · Esc: back"
	if m.Mode == GroupColorPicker {
		if group, ok := m.Session.Data.Group(m.groupColorID); ok {
			title = "Group color: #" + group.Name + " · Esc: back"
		}
	}
	return m.colorItems(), &m.colorMenu, title, ""
}
func (m *Model) dialogStatus() string {
	if m.PromptError != "" {
		return m.PromptError
	}
	return m.statusText()
}
func (m *Model) followPicker() {
	if !m.isPicker() {
		return
	}
	items, state, _, action := m.pickerItems()
	state.Index = max(0, min(state.Index, len(items)-1))
	if action == "" {
		state.Action = false
	}
	capacity := style.MenuCapacity(m.Width, m.Height, action, m.dialogStatus())
	state.Offset = max(0, min(state.Offset, max(0, len(items)-capacity)))
	if state.Index < state.Offset {
		state.Offset = state.Index
	}
	if state.Index >= state.Offset+capacity {
		state.Offset = state.Index - capacity + 1
	}
}
func (m *Model) menuLayout() style.MenuLayout {
	items, state, title, action := m.pickerItems()
	labels := make([]string, len(items))
	for i, item := range items {
		labels[i] = item.Label
	}
	return m.Renderer.Menu(title, labels, state.Index, state.Offset, m.Width, m.Height, action, state.Action, m.dialogStatus())
}
func (m *Model) scrollPicker(delta int) {
	items, state, _, action := m.pickerItems()
	capacity := style.MenuCapacity(m.Width, m.Height, action, m.dialogStatus())
	state.Offset = max(0, min(state.Offset+delta, max(0, len(items)-capacity)))
}
func (m *Model) pickerKey(key string) tea.Cmd {
	if key == "esc" {
		if m.Mode == GroupColorPicker {
			m.Mode = GroupPicker
		} else {
			m.Mode = Main
		}
		m.PromptError = ""
		m.followPicker()
		return nil
	}
	if m.Pending {
		return nil
	}
	items, state, _, action := m.pickerItems()
	switch key {
	case "tab", "shift+tab":
		if action != "" {
			state.Action = !state.Action
		}
	case "up", "down", "home", "end":
		state.Action = false
		switch key {
		case "up":
			state.Index--
		case "down":
			state.Index++
		case "home":
			state.Index = 0
		case "end":
			state.Index = len(items) - 1
		}
		m.followPicker()
	case "enter":
		return m.activatePicker()
	}
	return nil
}
func (m *Model) activatePicker() tea.Cmd {
	if m.Pending {
		return nil
	}
	switch m.Mode {
	case GroupPicker:
		if m.groupMenu.Action {
			m.openGroupColors()
			return nil
		}
		items := m.groupItems()
		if items[m.groupMenu.Index].Value == "!new" {
			return m.openPrompt(NewGroup)
		}
		return m.mutate("group")
	case TaskColorPicker:
		return m.mutate("task-color")
	case GroupColorPicker:
		return m.mutate("group-color")
	}
	return nil
}
func (m *Model) pickerClick(x, y int) tea.Cmd {
	if m.Pending {
		return nil
	}
	index, action, ok := m.menuLayout().Hit(x, y)
	if !ok {
		return nil
	}
	_, state, _, _ := m.pickerItems()
	state.Action = action
	if !action {
		state.Index = index
	}
	return m.activatePicker()
}
