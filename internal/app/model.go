// Package app owns the Bubble Tea event loop and transactional save state.
package app

import (
	"charm.land/lipgloss/v2"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"taskwarrior/internal/config"
	"time"
	"unicode"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
	"taskwarrior/internal/storage"
	"taskwarrior/internal/style"
	"taskwarrior/internal/tasks"
)

type Mode int

const (
	Main Mode = iota
	Adding
	Help
	GroupPicker
	NewGroup
	TaskColorPicker
	GroupColorPicker
	StyleEditor
)

type transaction struct {
	session                 tasks.Session
	selected, undoSelection string
	offset                  int
	operation               string
	mode                    Mode
}
type saveResult struct{ storage.Result }
type clearStatus uint64
type clipboardError struct{ err error }

type Model struct {
	visibleRows           []tasks.Task
	visibleRevision       uint64
	visibleSource         *tasks.Task
	visibleCount          int
	visibleHide           bool
	Settings              config.Config
	ConfigPath, ErrorPath string
	configWrite           *configTransaction
	editor                *editorState
	groupMenu, colorMenu  choiceState
	groupColorID          string
	saveWarning           bool
	Session               tasks.Session
	Path                  string
	Selected              string
	Offset, Width, Height int
	Mode                  Mode
	Input                 textinput.Model
	Renderer              style.Renderer
	Pending, Quitting     bool
	LastError             error
	PromptError           string
	status                string
	statusSerial          uint64
	helpOffset            int
	undoSelection         string
	before                *transaction
}

func New(data tasks.Data, path string) *Model {
	input := textinput.New()
	input.Prompt = "❯ "
	input.Placeholder = "Task title"
	input.SetVirtualCursor(true)
	input.SetStyles(textinput.Styles{})
	_, noColor := os.LookupEnv("NO_COLOR")
	m := &Model{Session: tasks.NewSession(data), Path: path, Input: input, Renderer: style.Renderer{Appearance: style.Defaults(), NoColor: noColor, Palette: style.DefaultPalette()}}
	m.Settings = config.Defaults()
	m.ConfigPath = filepath.Join(filepath.Dir(path), "config.toml")
	m.reconcile(0)
	return m
}
func (m *Model) Init() tea.Cmd { return nil }
func (m *Model) rows() []tasks.Task {
	data := m.Session.Data.Tasks
	var source *tasks.Task
	if len(data) > 0 {
		source = &data[0]
	}
	hide := m.Renderer.Appearance.HideCompleted
	revision := m.Session.Revision()
	if m.visibleRows == nil || m.visibleSource != source || m.visibleCount != len(data) || m.visibleRevision != revision || m.visibleHide != hide {
		m.visibleRows = m.Session.Data.Visible(hide)
		m.visibleSource, m.visibleCount, m.visibleRevision, m.visibleHide = source, len(data), revision, hide
	}
	return m.visibleRows
}
func (m *Model) index() int {
	for i, t := range m.rows() {
		if t.ID == m.Selected {
			return i
		}
	}
	return -1
}
func (m *Model) statusText() string {
	if m.Pending {
		return "Saving…"
	}
	if m.LastError != nil {
		cause := m.LastError
		for errors.Unwrap(cause) != nil {
			cause = errors.Unwrap(cause)
		}
		label := "Save failed"
		if m.saveWarning {
			label = "Saved; crash durability uncertain"
		}
		return fmt.Sprintf("%s: %v\n%s", label, cause, m.errorPath())
	}
	return m.status
}
func (m *Model) capacity() int {
	return max(1, style.ListCapacity(m.Renderer.Appearance, m.Width, m.Height, m.statusText()))
}
func (m *Model) clampOffset() { m.Offset = max(0, min(m.Offset, max(0, len(m.rows())-m.capacity()))) }
func (m *Model) follow() {
	m.clampOffset()
	i := m.index()
	if i < 0 {
		return
	}
	if i < m.Offset {
		m.Offset = i
	}
	if i >= m.Offset+m.capacity() {
		m.Offset = i - m.capacity() + 1
	}
}
func (m *Model) reconcile(previousIndex int) {
	rows := m.rows()
	if len(rows) == 0 {
		m.Selected = ""
		m.Offset = 0
		return
	}
	if m.index() < 0 {
		m.Selected = rows[max(0, min(previousIndex, len(rows)-1))].ID
	}
	m.follow()
}
func (m *Model) navigate(index int) {
	rows := m.rows()
	if len(rows) == 0 {
		return
	}
	m.Selected = rows[max(0, min(index, len(rows)-1))].ID
	m.follow()
}
func (m *Model) quit() tea.Cmd {
	if m.Pending {
		m.Quitting = true
		return nil
	}
	return tea.Quit
}
func (m *Model) mutate(operation string) tea.Cmd {
	if m.Pending {
		return nil
	}
	if (operation == "toggle" || operation == "delete") && m.Selected == "" {
		return nil
	}
	before := &transaction{session: m.Session.Clone(), selected: m.Selected, undoSelection: m.undoSelection, offset: m.Offset, operation: operation, mode: m.Mode}
	index := m.index()
	var err error
	switch operation {
	case "add":
		m.Selected, err = m.Session.Add(m.Input.Value(), m.Renderer.Palette.Keys()...)
	case "group-new":
		if err = tasks.ValidateGroupName(m.Input.Value()); err == nil {
			err = m.Session.AssignGroup(m.Selected, m.Input.Value(), m.Renderer.Palette.Keys()...)
		}
	case "group":
		item := m.groupItems()[m.groupMenu.Index]
		err = m.Session.AssignGroup(m.Selected, item.Value, m.Renderer.Palette.Keys()...)
	case "task-color":
		err = m.Session.SetTaskColor(m.Selected, m.colorItems()[m.colorMenu.Index].Value)
	case "group-color":
		err = m.Session.SetGroupColor(m.groupColorID, m.colorItems()[m.colorMenu.Index].Value)
	case "toggle":
		err = m.Session.Toggle(m.Selected)
	case "delete":
		err = m.Session.Delete(m.Selected)
	case "undo":
		err = m.Session.Undo()
		if err == nil {
			m.Selected = m.undoSelection
		}
	}
	if errors.Is(err, tasks.ErrNoChange) {
		m.completeDialog(operation)
		return nil
	}
	if err != nil {
		m.Selected = before.selected
		m.PromptError = err.Error()
		return nil
	}
	if operation != "undo" {
		m.undoSelection = before.selected
	} else {
		m.undoSelection = ""
	}
	m.PromptError = ""
	m.before = before
	m.Pending = true
	m.reconcile(index)
	m.followPicker()
	snapshot, path := m.Session.Data.Clone(), m.Path
	return func() tea.Msg { return saveResult{storage.Save(path, snapshot)} }
}
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.Mode == StyleEditor {
		switch event := msg.(type) {
		case tea.KeyPressMsg:
			if event.String() == "ctrl+c" {
				return m, m.quit()
			}
			return m, m.editorUpdate(msg)
		case tea.PasteMsg, tea.MouseClickMsg, tea.MouseWheelMsg, clipboardError:
			return m, m.editorUpdate(msg)
		}
	}
	switch msg := msg.(type) {
	case configResult:
		return m, m.finishConfig(msg.Result)

	case clipboardError:
		if m.isPrompt() && !m.Pending {
			m.PromptError = msg.err.Error()
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.Width, m.Height = msg.Width, msg.Height
		m.Input.SetWidth(max(1, msg.Width-4))
		m.follow()
		m.followPicker()
		m.followEditor()
		return m, nil
	case saveResult:
		if !m.Pending {
			return m, nil
		}
		before := m.before
		m.Pending = false
		m.before = nil
		m.status = ""
		m.statusSerial++
		m.LastError = msg.Err
		m.ErrorPath = m.Path
		m.saveWarning = msg.Published && msg.Err != nil
		if !msg.Published {
			m.Session = before.session
			m.Selected = before.selected
			m.undoSelection = before.undoSelection
			m.Offset = before.offset
			m.Mode = before.mode
			if m.isPrompt() {
				m.Input.Focus()
			}
			m.followPicker()
		} else {
			m.completeDialog(before.operation)
			if before.operation == "delete" && msg.Err == nil {
				m.status = "Deleted. Press u to undo."
			}
		}
		m.follow()
		if m.Quitting {
			return m, tea.Quit
		}
		if m.status != "" {
			serial := m.statusSerial
			return m, tea.Tick(4*time.Second, func(time.Time) tea.Msg { return clearStatus(serial) })
		}
		return m, nil
	case clearStatus:
		if uint64(msg) == m.statusSerial {
			m.status = ""
			m.follow()
		}
		return m, nil
	case tea.KeyPressMsg:
		key := msg.String()
		if key == "ctrl+c" {
			return m, m.quit()
		}
		if m.Mode == Help {
			switch key {
			case "esc", "?", "q":
				m.Mode = Main
			case "up", "k":
				m.helpOffset = max(0, m.helpOffset-1)
			case "down", "j":
				m.helpOffset = min(max(0, len(m.helpLines())-max(1, m.Height-1)), m.helpOffset+1)
			case "home":
				m.helpOffset = 0
			case "end":
				m.helpOffset = max(0, len(m.helpLines())-max(1, m.Height-1))
			}
			return m, nil
		}
		if m.isPicker() {
			return m, m.pickerKey(key)
		}
		if m.isPrompt() {
			if key == "esc" {
				if m.Mode == NewGroup {
					m.Mode = GroupPicker
				} else {
					m.Mode = Main
				}
				m.followPicker()
				m.Input.Blur()
				if !m.Pending {
					m.Input.Reset()
					m.PromptError = ""
				}
				return m, nil
			}
			if m.Pending {
				return m, nil
			}
			if key == "enter" {
				operation := "add"
				if m.Mode == NewGroup {
					operation = "group-new"
				}
				return m, m.mutate(operation)
			}
			// Route clipboard content through the same validation as terminal paste.
			if key == "ctrl+v" {
				return m, func() tea.Msg {
					value, err := clipboard.ReadAll()
					if err != nil {
						return clipboardError{err}
					}
					return tea.PasteMsg{Content: value}
				}
			}
			m.PromptError = ""
			var cmd tea.Cmd
			m.Input, cmd = m.Input.Update(msg)
			return m, cmd
		}
		if key == "esc" {
			return m, m.quit()
		}
		switch m.Settings.ActionFor(key) {
		case "quit":
			return m, m.quit()
		case "up":
			m.navigate(m.index() - 1)
		case "down":
			m.navigate(m.index() + 1)
		case "first":
			m.navigate(0)
		case "last":
			m.navigate(len(m.rows()) - 1)
		case "help":
			m.Mode = Help
			m.helpOffset = 0
		case "add":
			if !m.Pending {
				return m, m.openPrompt(Adding)
			}
		case "group":
			if !m.Pending && m.Selected != "" {
				m.openGroups()
			}
		case "color":
			if !m.Pending && m.Selected != "" {
				m.openTaskColors()
			}
		case "style":
			if !m.Pending {
				m.openEditor()
			}
		case "hide_completed":
			if !m.Pending {
				return m, m.toggleVisibility()
			}
		case "toggle":
			return m, m.mutate("toggle")
		case "delete":
			return m, m.mutate("delete")
		case "undo":
			return m, m.mutate("undo")
		}
		return m, nil
	case tea.PasteMsg:
		if !m.isPrompt() || m.Pending {
			return m, nil
		}
		text := strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ", "\t", " ").Replace(msg.Content)
		for _, r := range text {
			if unicode.IsControl(r) {
				m.PromptError = "Text contains terminal control characters"
				return m, nil
			}
		}
		// Sanitize before Bubbles can discard controls, so invalid input is reported.
		msg.Content = text
		m.PromptError = ""
		var cmd tea.Cmd
		m.Input, cmd = m.Input.Update(msg)
		return m, cmd
	case tea.MouseClickMsg:
		if !m.Renderer.Appearance.Mouse {
			return m, nil
		}
		if m.isPicker() && msg.Button == tea.MouseLeft {
			return m, m.pickerClick(msg.X, msg.Y)
		}
		if m.Mode != Main {
			return m, nil
		}
		if msg.Button != tea.MouseLeft {
			return m, nil
		}
		layout := m.listRenderer().List(m.rows(), m.Selected, m.Offset, m.Width, m.Height, m.statusText(), m.Session.Data.Groups...)
		id, action := layout.HitAction(msg.X, msg.Y)
		if id == "" {
			return m, nil
		}
		m.Selected = id
		if action == "delete" {
			return m, m.mutate("delete")
		}
		if action == "toggle" {
			return m, m.mutate("toggle")
		}
		return m, nil
	case tea.MouseWheelMsg:
		if !m.Renderer.Appearance.Mouse || m.Width < 12 || m.Height < 3 {
			return m, nil
		}
		delta := 0
		switch msg.Button {
		case tea.MouseWheelDown:
			delta = 1
		case tea.MouseWheelUp:
			delta = -1
		}
		if m.isPicker() {
			m.scrollPicker(delta)
			return m, nil
		}
		if m.Mode == Help {
			m.helpOffset = max(0, min(max(0, len(m.helpLines())-max(1, m.Height-1)), m.helpOffset+delta))
		} else if m.Mode == Main {
			m.Offset += delta
			m.clampOffset()
		}
		return m, nil
	}
	if m.isPrompt() {
		var cmd tea.Cmd
		m.Input, cmd = m.Input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *Model) View() tea.View {
	var content string
	ascii := m.Renderer.Appearance.Symbols == "ascii"
	helpLines := m.helpLines()
	switch {
	case m.Width < 12 || m.Height < 3:
		content = style.Clip("Resize terminal", m.Width, ascii)
	case m.Mode == StyleEditor:
		content = m.editorView().Content
	case m.Mode == Help:
		lines := []string{style.Clip(" Esc: return · ↑/↓: scroll", m.Width, ascii)}
		start := min(m.helpOffset, max(0, len(m.helpLines())-max(1, m.Height-1)))
		for i := start; i < len(helpLines) && len(lines) < m.Height; i++ {
			lines = append(lines, style.Clip(" "+helpLines[i], m.Width, ascii))
		}
		content = strings.Join(lines, "\n")
	case m.isPicker():
		content = m.menuLayout().Content
	case m.isPrompt():
		title := " Add task · Enter: save · Esc: cancel"
		if m.Mode == NewGroup {
			title = " New group · Enter: assign · Esc: back"
		}
		lines := []string{style.Clip(title, m.Width, ascii), style.Clip(" "+m.Input.View(), m.Width, ascii)}
		if m.PromptError != "" {
			lines = append(lines, style.Clip(" "+m.PromptError, m.Width, ascii))
		}
		if status := m.statusText(); status != "" && len(lines) < m.Height {
			lines = append(lines, style.StatusLines(status, m.Width, m.Height-len(lines))...)
		}
		content = strings.Join(lines, "\n")
	default:
		content = m.listRenderer().List(m.rows(), m.Selected, m.Offset, m.Width, m.Height, m.statusText(), m.Session.Data.Groups...).Content
	}
	view := tea.NewView(content)
	view.AltScreen = true
	if !m.Renderer.NoColor {
		if fg := m.Renderer.Appearance.Foreground; fg != "terminal" {
			view.ForegroundColor = lipgloss.Color(fg)
		}
		if bg := m.Renderer.Appearance.Background; bg != "terminal" {
			view.BackgroundColor = lipgloss.Color(bg)
		}
	}
	if m.Renderer.Appearance.Mouse {
		view.MouseMode = tea.MouseModeCellMotion
	}
	return view
}
