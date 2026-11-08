package app

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"strings"
	"taskwarrior/internal/config"
	"taskwarrior/internal/storage"
	"taskwarrior/internal/style"
	"taskwarrior/internal/tasks"
)

type configTransaction struct {
	Candidate, Before config.Config
	Selected          string
	Offset, Index     int
	Editor            bool
}
type configResult struct{ storage.Result }

func NewConfigured(data tasks.Data, path, configPath string, c config.Config) *Model {
	m := New(data, path)
	m.ConfigPath = configPath
	m.applyConfig(c)
	m.reconcile(0)
	return m
}
func (m *Model) currentConfig() config.Config {
	c := m.Settings.Clone()
	c.Appearance = m.Renderer.Appearance
	c.Palette.Entries = append(style.Palette{}, m.Renderer.Palette...)
	return c
}
func (m *Model) applyConfig(c config.Config) {
	m.Settings = c.Clone()
	m.Renderer.Appearance = c.Appearance
	m.Renderer.Palette = append(style.Palette{}, c.Palette.Entries...)
	m.Input.Prompt = "❯ "
	if c.Appearance.Symbols == "ascii" {
		m.Input.Prompt = "> "
	}
}
func (m *Model) errorPath() string {
	if m.ErrorPath != "" {
		return m.ErrorPath
	}
	return m.Path
}
func (m *Model) binding(action string) string { return strings.Join(m.Settings.Keys[action], "/") }
func (m *Model) listRenderer() style.Renderer {
	r := m.Renderer
	r.EmptyHintCompact = m.Settings.Keys["add"][0] + ": Add"
	r.EmptyHint = fmt.Sprintf("No tasks. Press %s to add.", m.binding("add"))
	if len(m.Session.Data.Tasks) > 0 && len(m.rows()) == 0 {
		r.EmptyHintCompact = m.Settings.Keys["add"][0] + ":add " + m.Settings.Keys["hide_completed"][0] + ":show"
		r.EmptyHint = fmt.Sprintf("All tasks hidden. %s: Add · %s: Show completed", m.binding("add"), m.binding("hide_completed"))
	}
	r.FooterText = fmt.Sprintf("%s add · %s toggle · %s delete · %s undo · %s style · %s help · %s quit", m.binding("add"), m.binding("toggle"), m.binding("delete"), m.binding("undo"), m.binding("style"), m.binding("help"), m.binding("quit"))
	return r
}
func (m *Model) helpLines() []string {
	return []string{
		"TaskWarrior — Milestone 4",
		m.binding("up") + " / " + m.binding("down") + "  Move selection",
		m.binding("first") + " / " + m.binding("last") + "  First / last task",
		m.binding("add") + "  Add task",
		m.binding("toggle") + "  Complete / reopen",
		m.binding("delete") + "  Delete immediately",
		m.binding("undo") + "  Undo last mutation",
		m.binding("group") + "  Assign group / group color",
		m.binding("color") + "  Task color / inherit",
		m.binding("help") + "  Shortcut help",
		m.binding("style") + "  Style editor",
		m.binding("hide_completed") + "  Hide / show completed",
		m.binding("quit") + " / Esc  Quit list",
		"Ctrl+C  Quit (wait for save)", "Prompt: Enter saves, Esc cancels", "Click body: select; checkbox: toggle", "Wheel: scroll; keys reveal selection",
	}
}
func (m *Model) toggleVisibility() tea.Cmd {
	c := m.currentConfig()
	c.Appearance.HideCompleted = !c.Appearance.HideCompleted
	return m.saveSettings(c, false)
}
func (m *Model) saveSettings(candidate config.Config, fromEditor bool) tea.Cmd {
	if m.Pending {
		return nil
	}
	candidate = candidate.Clone()
	if err := candidate.Normalize(); err != nil {
		if m.editor != nil {
			m.editor.Error = err.Error()
		}
		return nil
	}
	m.configWrite = &configTransaction{Candidate: candidate, Before: m.currentConfig(), Selected: m.Selected, Offset: m.Offset, Index: m.index(), Editor: fromEditor}
	m.Pending = true
	if !fromEditor {
		m.applyConfig(candidate)
		m.reconcile(m.configWrite.Index)
	}
	m.followEditor()
	path := m.ConfigPath
	return func() tea.Msg { return configResult{storage.SaveConfig(path, candidate)} }
}
func (m *Model) finishConfig(result storage.Result) tea.Cmd {
	if m.configWrite == nil {
		return nil
	}
	write := m.configWrite
	m.configWrite = nil
	m.Pending = false
	m.LastError = result.Err
	m.ErrorPath = m.ConfigPath
	m.saveWarning = result.Published && result.Err != nil
	if result.Published {
		m.applyConfig(write.Candidate)
		m.reconcile(write.Index)
		if write.Editor {
			m.Mode = Main
			m.editor = nil
		}
	} else {
		m.applyConfig(write.Before)
		m.Selected = write.Selected
		m.Offset = write.Offset
		if write.Editor {
			m.Mode = StyleEditor
			m.followEditor()
		}
	}
	if m.Quitting {
		return tea.Quit
	}
	return nil
}
