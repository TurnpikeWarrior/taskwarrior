// Package style shares cell layout between the list renderer and mouse input.
package style

import (
	"fmt"
	"strings"
	"taskwarrior/internal/tasks"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Appearance supplies typed presentation settings to the renderer.
type Appearance struct {
	ListMode            string `toml:"list_mode"`
	RowGap              int    `toml:"row_gap"`
	LeftPadding         int    `toml:"left_padding"`
	HideCompleted       bool   `toml:"hide_completed"`
	Border              bool   `toml:"border"`
	Footer              bool   `toml:"footer"`
	Symbols             string `toml:"symbols"`
	Foreground          string `toml:"foreground"`
	Background          string `toml:"background"`
	CompletedForeground string `toml:"completed_foreground"`
	SelectionForeground string `toml:"selection_foreground"`
	Mouse               bool   `toml:"mouse"`
	MouseDelete         bool   `toml:"mouse_delete"`
}

func Defaults() Appearance {
	return Appearance{ListMode: "plain", LeftPadding: 1, Symbols: "unicode", Foreground: "terminal", Background: "terminal", CompletedForeground: "terminal", SelectionForeground: "terminal", Mouse: true}
}

type Renderer struct {
	Appearance            Appearance
	NoColor               bool
	Palette               Palette
	FooterText, EmptyHint string
	EmptyHintCompact      string
}
type Target struct {
	ID                                        string
	Y, Start, End, CheckboxStart, CheckboxEnd int
	DeleteStart, DeleteEnd                    int
}
type Layout struct {
	Content string
	Targets []Target
}

func (l Layout) Hit(x, y int) (string, bool) {
	id, action := l.HitAction(x, y)
	return id, action == "toggle"
}
func (l Layout) HitAction(x, y int) (string, string) {
	for _, t := range l.Targets {
		if y == t.Y && x >= t.Start && x < t.End {
			if x >= t.DeleteStart && x < t.DeleteEnd {
				return t.ID, "delete"
			}
			if x >= t.CheckboxStart && x < t.CheckboxEnd {
				return t.ID, "toggle"
			}
			return t.ID, "select"
		}
	}
	return "", ""
}
func Clip(text string, width int, ascii bool) string {
	if width <= 0 {
		return ""
	}
	tail := "…"
	if ascii {
		tail = "..."
		if width < 3 {
			tail = strings.Repeat(".", width)
		}
	}
	return ansi.Truncate(text, width, tail)
}

// StatusLines wraps diagnostics so long filesystem paths do not hide the cause.
func StatusLines(text string, width, limit int) []string {
	if text == "" || width < 2 || limit <= 0 {
		return nil
	}
	lines := strings.Split(ansi.Hardwrap(text, width-1, true), "\n")
	if len(lines) > limit {
		lines = lines[:limit]
		lines[limit-1] = ansi.Truncate(lines[limit-1], max(0, width-2), "") + "…"
	}
	for i := range lines {
		lines[i] = " " + lines[i]
	}
	return lines
}
func Capacity(width, height int, status string) int {
	return max(0, height-len(StatusLines(status, width, max(0, height-1))))
}

// ListCapacity returns the number of task rows the current cell layout can show.
func ListCapacity(a Appearance, width, height int, status string) int {
	innerWidth, innerHeight, _, footer := listFrame(a, width, height)
	if footer {
		innerHeight--
	}
	lines := Capacity(innerWidth, innerHeight, status)
	return max(1, (lines+a.RowGap)/(a.RowGap+1))
}
func listFrame(a Appearance, width, height int) (innerWidth, innerHeight, border int, footer bool) {
	if a.Border {
		border = 1
	}
	innerWidth = max(1, width-2*border)
	innerHeight = max(1, height-2*border)
	footer = a.Footer && innerHeight >= 2
	return
}
func (r Renderer) paint(text, fg string, faint, strike bool) string {
	if r.NoColor {
		return text
	}
	s := lipgloss.NewStyle().Strikethrough(strike).Faint(faint)
	if fg != "" && fg != "terminal" {
		s = s.Foreground(lipgloss.Color(fg))
	}
	return s.Render(text)
}
func (r Renderer) List(rows []tasks.Task, selected string, offset, width, height int, status string, groups ...tasks.Group) Layout {
	a := r.Appearance
	ascii := a.Symbols == "ascii"
	if width < 12 || height < 3 {
		return Layout{Content: Clip("Resize terminal", width, ascii)}
	}
	iw, ih, border, footer := listFrame(a, width, height)
	taskHeight := ih
	if footer {
		taskHeight--
	}
	capacity := ListCapacity(a, width, height, status)
	marker, empty, done, bullet, del := "❯", "☐", "☑", "•", " ×"
	if ascii {
		marker, empty, done, bullet, del = ">", "[ ]", "[x]", "-", " x"
	}
	numbering := len(fmt.Sprint(len(rows)))
	var lines []string
	layout := Layout{}
	for i := max(0, offset); i < len(rows) && len(layout.Targets) < capacity; i++ {
		row := rows[i]
		if len(lines) > 0 {
			for gap := 0; gap < a.RowGap; gap++ {
				lines = append(lines, "")
			}
		}
		mark := " "
		if row.ID == selected {
			mark = marker
		}
		box := empty
		if row.Completed {
			box = done
		}
		decoration := ""
		switch a.ListMode {
		case "numbered":
			decoration = fmt.Sprintf("%*d. ", numbering, i+1)
		case "bulleted":
			decoration = bullet + " "
		}
		// Compress optional columns at tiny widths while retaining focus and checkbox.
		pad := min(a.LeftPadding, max(0, iw-(2+ansi.StringWidth(decoration)+ansi.StringWidth(box)+2)))
		if pad+2+ansi.StringWidth(decoration)+ansi.StringWidth(box)+2 > iw {
			decoration = ""
			pad = 0
		}
		prefix := strings.Repeat(" ", pad) + r.paint(mark, a.SelectionForeground, false, false) + " " + decoration + box + " "
		available := iw - ansi.StringWidth(prefix)
		suffix := ""
		for _, g := range groups {
			if g.ID == row.GroupID {
				suffix = "  #" + g.Name
				break
			}
		}
		control := ""
		if a.MouseDelete && row.ID == selected {
			control = del
		}
		if ansi.StringWidth(row.Text+suffix+control) > available {
			control = ""
		}
		if ansi.StringWidth(row.Text+suffix) > available {
			suffix = ""
		}
		title := Clip(row.Text, available-ansi.StringWidth(suffix+control), ascii)
		fg, faint := a.Foreground, false
		if row.Completed {
			fg = a.CompletedForeground
			faint = fg == "terminal"
		} else if assigned := ResolveColor(row, groups, r.Palette); assigned != "" {
			fg = assigned
		}
		line := prefix + r.paint(title, fg, faint, row.Completed) + r.paint(suffix, fg, faint, false) + control
		boxX := border + pad + 2 + ansi.StringWidth(decoration)
		target := Target{ID: row.ID, Y: border + len(lines), Start: border + pad, End: border + ansi.StringWidth(line), CheckboxStart: boxX, CheckboxEnd: boxX + ansi.StringWidth(box), DeleteStart: -1, DeleteEnd: -1}
		if control != "" {
			target.DeleteStart = target.End - ansi.StringWidth(control) + 1
			target.DeleteEnd = target.End
		}
		layout.Targets = append(layout.Targets, target)
		lines = append(lines, line)
	}
	if len(rows) == 0 {
		hint := r.EmptyHint
		if hint == "" {
			hint = "No tasks. Press a to add."
		}
		if ansi.StringWidth(" "+hint) > iw {
			if r.EmptyHintCompact != "" {
				hint = r.EmptyHintCompact
			} else {
				hint = "a: Add"
			}
		}
		lines = append(lines, Clip(" "+hint, iw, ascii))
	}
	lines = append(lines, StatusLines(status, iw, max(0, taskHeight-len(lines)))...)
	if footer {
		text := r.FooterText
		if text == "" {
			text = "a add · t style · ? help · q quit"
		}
		lines = append(lines, Clip(" "+text, iw, ascii))
	}
	for i, line := range lines {
		if border > 0 || a.Background != "terminal" {
			line += strings.Repeat(" ", max(0, iw-ansi.StringWidth(line)))
		}
		if !r.NoColor {
			s := lipgloss.NewStyle()
			if a.Foreground != "terminal" {
				s = s.Foreground(lipgloss.Color(a.Foreground))
			}
			if a.Background != "terminal" {
				s = s.Background(lipgloss.Color(a.Background))
			}
			line = s.Render(line)
		}
		lines[i] = line
	}
	if border > 0 {
		tl, tr, bl, br, v, h := "┌", "┐", "└", "┘", "│", "─"
		if ascii {
			tl, tr, bl, br, v, h = "+", "+", "+", "+", "|", "-"
		}
		for i := range lines {
			lines[i] = v + lines[i] + v
		}
		lines = append([]string{tl + strings.Repeat(h, iw) + tr}, lines...)
		lines = append(lines, bl+strings.Repeat(h, iw)+br)
	}
	layout.Content = strings.Join(lines, "\n")
	return layout
}
