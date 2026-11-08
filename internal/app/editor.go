package app

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"fmt"
	"github.com/atotto/clipboard"
	"strings"
	"taskwarrior/internal/config"
	"taskwarrior/internal/style"
	"taskwarrior/internal/tasks"
	"unicode"
)

type editorPage int

const (
	editorMain editorPage = iota
	editorAppearance
	editorPalette
	editorForm
)

type editorState struct {
	Draft                                config.Config
	Page                                 editorPage
	Main, Appearance, Palette, FormState choiceState
	PaletteIndex                         int
	Form                                 *editorFieldForm
	Error                                string
}
type editorFieldForm struct {
	Parent          editorPage
	AppearanceField int
	Key             string
	Labels          []string
	Inputs          []textinput.Model
}

func (m *Model) openEditor() { m.editor = &editorState{Draft: m.currentConfig()}; m.Mode = StyleEditor }
func (m *Model) editorStateAndLabels() (*choiceState, []string, string) {
	e := m.editor
	a := e.Draft.Appearance
	switch e.Page {
	case editorMain:
		return &e.Main, []string{"Appearance", "Palette", "Save", "Cancel"}, "Style editor · Tab: focus · Enter · Esc: cancel"
	case editorAppearance:
		density := []string{"Compact", "Comfortable", "Spacious"}[a.RowGap]
		return &e.Appearance, []string{
			"List mode: " + a.ListMode, "Density: " + density, fmt.Sprintf("Left padding: %d", a.LeftPadding), "Foreground: " + a.Foreground, "Background: " + a.Background, "Completed foreground: " + a.CompletedForeground, "Selection marker: " + a.SelectionForeground, "Symbols: " + a.Symbols,
			fmt.Sprintf("Border: %t", a.Border), fmt.Sprintf("Footer: %t", a.Footer), fmt.Sprintf("Mouse: %t", a.Mouse), fmt.Sprintf("Mouse delete: %t", a.MouseDelete), fmt.Sprintf("Hide completed: %t", a.HideCompleted), "Back",
		}, "Appearance · Enter: change · Esc: back"
	case editorPalette:
		entries := e.Draft.Palette.Entries
		labels := []string{}
		for _, c := range entries {
			labels = append(labels, c.Name+" "+c.Hex)
		}
		name := "(none)"
		if len(entries) > 0 {
			e.PaletteIndex = max(0, min(e.PaletteIndex, len(entries)-1))
			name = entries[e.PaletteIndex].Name
		}
		labels = append(labels, "Add color", "Edit: "+name, "Remove: "+name, "Restore Defaults", "Back")
		return &e.Palette, labels, "Palette · Tab: focus · Enter · Esc: back"
	case editorForm:
		labels := []string{}
		for i, input := range e.Form.Inputs {
			labels = append(labels, e.Form.Labels[i]+": "+input.View())
		}
		labels = append(labels, "Apply", "Cancel")
		return &e.FormState, labels, "Color · Tab: next · Enter: apply · Esc: back"
	}
	return nil, nil, ""
}
func (m *Model) preview() []string {
	if m.Height < 8 || m.editor.Page == editorForm {
		return nil
	}
	r := m.listRenderer()
	r.Appearance = m.editor.Draft.Appearance
	r.Palette = m.editor.Draft.Palette.Entries
	r.EmptyHint = ""
	group := tasks.Group{ID: "preview", Name: "Group", ColorKey: "blue"}
	override := ""
	if len(m.Session.Data.Groups) > 0 {
		group = m.Session.Data.Groups[0]
	}
	if len(m.Session.Data.Tasks) > 0 {
		override = m.Session.Data.Tasks[0].ColorKey
	}
	rows := []tasks.Task{{ID: "selected", Text: "Selected task", GroupID: group.ID, ColorKey: override}, {ID: "active", Text: "Active task", GroupID: group.ID}, {ID: "completed", Text: "Completed task", Completed: true, GroupID: group.ID}}
	content := r.List(rows, "selected", 0, m.Width, min(12, max(3, m.Height/2)), "", group).Content
	return append([]string{" Preview"}, strings.Split(content, "\n")...)
}
func (m *Model) followEditor() {
	if m.editor == nil || m.Mode != StyleEditor {
		return
	}
	e := m.editor
	if e.Page == editorForm {
		for i := range e.Form.Inputs {
			e.Form.Inputs[i].SetWidth(max(1, m.Width-len(e.Form.Labels[i])-6))
		}
	}
	state, labels, _ := m.editorStateAndLabels()
	state.Index = max(0, min(state.Index, len(labels)-1))
	if e.Page == editorPalette && state.Index < len(e.Draft.Palette.Entries) {
		e.PaletteIndex = state.Index
	}
	status := m.editorStatus()
	height := m.Height - len(m.preview())
	capacity := style.MenuCapacity(m.Width, height, "", status)
	state.Offset = max(0, min(state.Offset, max(0, len(labels)-capacity)))
	if state.Index < state.Offset {
		state.Offset = state.Index
	}
	if state.Index >= state.Offset+capacity {
		state.Offset = state.Index - capacity + 1
	}
}
func (m *Model) editorStatus() string {
	if m.editor.Error != "" {
		return m.editor.Error
	}
	return m.statusText()
}
func (m *Model) editorView() style.MenuLayout {
	prefix := m.preview()
	state, labels, title := m.editorStateAndLabels()
	layout := m.Renderer.Menu(title, labels, state.Index, state.Offset, m.Width, m.Height-len(prefix), "", false, m.editorStatus())
	for i := range layout.Targets {
		layout.Targets[i].Y += len(prefix)
	}
	if len(prefix) > 0 {
		layout.Content = strings.Join(prefix, "\n") + "\n" + layout.Content
	}
	return layout
}
func (m *Model) focusForm(index int) {
	e := m.editor
	e.FormState.Index = index
	for i := range e.Form.Inputs {
		if i == index {
			e.Form.Inputs[i].Focus()
		} else {
			e.Form.Inputs[i].Blur()
		}
	}
	m.followEditor()
}
func (m *Model) openEditorForm(parent editorPage, field int, key string, labels, values []string) {
	inputs := make([]textinput.Model, len(values))
	for i, value := range values {
		input := textinput.New()
		input.Prompt = ""
		input.SetVirtualCursor(true)
		input.SetStyles(textinput.Styles{})
		input.SetValue(value)
		input.CursorEnd()
		inputs[i] = input
	}
	m.editor.Form = &editorFieldForm{Parent: parent, AppearanceField: field, Key: key, Labels: labels, Inputs: inputs}
	m.editor.Page = editorForm
	m.editor.FormState = choiceState{}
	m.editor.Error = ""
	m.focusForm(0)
}
func (m *Model) appearanceColor(field int) *string {
	a := &m.editor.Draft.Appearance
	switch field {
	case 3:
		return &a.Foreground
	case 4:
		return &a.Background
	case 5:
		return &a.CompletedForeground
	default:
		return &a.SelectionForeground
	}
}
func (m *Model) activateEditor() tea.Cmd {
	e := m.editor
	state, _, _ := m.editorStateAndLabels()
	e.Error = ""
	switch e.Page {
	case editorMain:
		switch state.Index {
		case 0:
			e.Page = editorAppearance
		case 1:
			e.Page = editorPalette
		case 2:
			return m.saveSettings(e.Draft, true)
		case 3:
			m.Mode = Main
			m.editor = nil
			return nil
		}
	case editorAppearance:
		a := &e.Draft.Appearance
		switch state.Index {
		case 0:
			switch a.ListMode {
			case "plain":
				a.ListMode = "numbered"
			case "numbered":
				a.ListMode = "bulleted"
			default:
				a.ListMode = "plain"
			}
		case 1:
			a.RowGap = (a.RowGap + 1) % 3
		case 2:
			a.LeftPadding = (a.LeftPadding + 1) % 5
		case 3, 4, 5, 6:
			m.openEditorForm(editorAppearance, state.Index, "", []string{"Color"}, []string{*m.appearanceColor(state.Index)})
			return nil
		case 7:
			if a.Symbols == "unicode" {
				a.Symbols = "ascii"
			} else {
				a.Symbols = "unicode"
			}
		case 8:
			a.Border = !a.Border
		case 9:
			a.Footer = !a.Footer
		case 10:
			a.Mouse = !a.Mouse
		case 11:
			a.MouseDelete = !a.MouseDelete
		case 12:
			a.HideCompleted = !a.HideCompleted
		case 13:
			e.Page = editorMain
		}
	case editorPalette:
		palette := e.Draft.Palette.Entries
		n := len(palette)
		index := state.Index
		switch {
		case index < n:
			m.openPaletteForm(palette[index].Key, palette[index].Name, palette[index].Hex)
			return nil
		case index == n:
			m.openPaletteForm("", "", "#AA55FF")
			return nil
		case index == n+1:
			if n > 0 {
				c := palette[e.PaletteIndex]
				m.openPaletteForm(c.Key, c.Name, c.Hex)
				return nil
			}
		case index == n+2:
			if n > 0 {
				e.Draft.Palette.Entries = palette.Remove(palette[e.PaletteIndex].Key)
				e.PaletteIndex = max(0, min(e.PaletteIndex, n-2))
				e.Palette.Index = len(e.Draft.Palette.Entries) + 2
			}
		case index == n+3:
			e.Draft.Palette.Entries = style.DefaultPalette()
			e.PaletteIndex = 0
			e.Palette.Index = len(e.Draft.Palette.Entries) + 3
		case index == n+4:
			e.Page = editorMain
		}
	case editorForm:
		fields := len(e.Form.Inputs)
		if state.Index < fields {
			m.focusForm(state.Index + 1)
			return nil
		}
		if state.Index == fields+1 {
			e.Page = e.Form.Parent
			e.Form = nil
			break
		}
		if err := m.applyEditorForm(); err != nil {
			e.Error = err.Error()
		} else {
			e.Page = e.Form.Parent
			e.Form = nil
		}
	}
	m.followEditor()
	return nil
}
func (m *Model) openPaletteForm(key, name, hex string) {
	m.openEditorForm(editorPalette, -1, key, []string{"Name", "HEX"}, []string{name, hex})
}
func (m *Model) applyEditorForm() error {
	e := m.editor
	form := e.Form
	if form.Parent == editorAppearance {
		color, err := style.NormalizeColor(form.Inputs[0].Value(), true)
		if err != nil {
			return err
		}
		*m.appearanceColor(form.AppearanceField) = color
		return nil
	}
	palette, err := e.Draft.Palette.Entries.Put(form.Key, form.Inputs[0].Value(), form.Inputs[1].Value())
	if err != nil {
		return err
	}
	e.Draft.Palette.Entries = palette
	if form.Key == "" {
		e.PaletteIndex = len(palette) - 1
	}
	e.Palette.Index = e.PaletteIndex
	return nil
}
func (m *Model) editorBack() {
	e := m.editor
	e.Error = ""
	switch e.Page {
	case editorMain:
		m.editor = nil
		m.Mode = Main
		return
	case editorAppearance, editorPalette:
		e.Page = editorMain
	case editorForm:
		e.Page = e.Form.Parent
		e.Form = nil
	}
	m.followEditor()
}
func (m *Model) editorUpdate(msg tea.Msg) tea.Cmd {
	e := m.editor
	switch event := msg.(type) {
	case tea.KeyPressMsg:
		key := event.String()
		if key == "esc" {
			if m.Pending {
				m.Mode = Main
			} else {
				m.editorBack()
			}
			return nil
		}
		if m.Pending {
			return nil
		}
		state, labels, _ := m.editorStateAndLabels()
		if key == "tab" || key == "shift+tab" {
			delta := 1
			if key == "shift+tab" {
				delta = -1
			}
			index := (state.Index + delta + len(labels)) % len(labels)
			if e.Page == editorForm {
				m.focusForm(index)
			} else {
				state.Index = index
				m.followEditor()
			}
			return nil
		}
		if key == "enter" {
			return m.activateEditor()
		}
		if e.Page == editorForm {
			if state.Index < len(e.Form.Inputs) {
				if key == "ctrl+v" {
					return func() tea.Msg {
						value, err := clipboard.ReadAll()
						if err != nil {
							return clipboardError{err}
						}
						return tea.PasteMsg{Content: value}
					}
				}
				input, cmd := e.Form.Inputs[state.Index].Update(msg)
				e.Form.Inputs[state.Index] = input
				e.Error = ""
				return cmd
			}
			return nil
		}
		switch key {
		case "up":
			state.Index--
		case "down":
			state.Index++
		case "home":
			state.Index = 0
		case "end":
			state.Index = len(labels) - 1
		}
		m.followEditor()
	case tea.PasteMsg:
		if m.Pending || e.Page != editorForm || e.FormState.Index >= len(e.Form.Inputs) {
			return nil
		}
		text := strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ", "\t", " ").Replace(event.Content)
		for _, r := range text {
			if unicode.IsControl(r) {
				e.Error = "Text contains terminal control characters"
				return nil
			}
		}
		input, cmd := e.Form.Inputs[e.FormState.Index].Update(tea.PasteMsg{Content: text})
		e.Form.Inputs[e.FormState.Index] = input
		e.Error = ""
		return cmd
	case clipboardError:
		e.Error = event.err.Error()
	case tea.MouseClickMsg:
		if m.Pending || !m.Renderer.Appearance.Mouse || m.Width < 12 || m.Height < 3 || event.Button != tea.MouseLeft {
			return nil
		}
		index, _, ok := m.editorView().Hit(event.X, event.Y)
		if !ok {
			return nil
		}
		state, _, _ := m.editorStateAndLabels()
		state.Index = index
		if e.Page == editorForm && index < len(e.Form.Inputs) {
			m.focusForm(index)
			return nil
		}
		return m.activateEditor()
	case tea.MouseWheelMsg:
		if !m.Renderer.Appearance.Mouse || m.Width < 12 || m.Height < 3 {
			return nil
		}
		delta := 0
		switch event.Button {
		case tea.MouseWheelDown:
			delta = 1
		case tea.MouseWheelUp:
			delta = -1
		}
		state, labels, _ := m.editorStateAndLabels()
		capacity := style.MenuCapacity(m.Width, m.Height-len(m.preview()), "", m.editorStatus())
		state.Offset = max(0, min(state.Offset+delta, max(0, len(labels)-capacity)))
	}
	return nil
}
