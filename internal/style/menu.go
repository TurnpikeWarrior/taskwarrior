package style

import (
	"github.com/charmbracelet/x/ansi"
	"strings"
)

type MenuTarget struct {
	Index, Y, Start, End int
	Action               bool
}
type MenuLayout struct {
	Content string
	Targets []MenuTarget
}

func (l MenuLayout) Hit(x, y int) (index int, action, ok bool) {
	for _, target := range l.Targets {
		if target.Y == y && x >= target.Start && x < target.End {
			return target.Index, target.Action, true
		}
	}
	return 0, false, false
}
func MenuCapacity(width, height int, action, status string) int {
	reserved := 1
	if action != "" {
		reserved++
	}
	reserved += len(StatusLines(status, width, max(0, height-reserved-1)))
	return max(1, height-reserved)
}
func (r Renderer) Menu(title string, options []string, index, offset, width, height int, action string, focusAction bool, status string) MenuLayout {
	ascii := r.Appearance.Symbols == "ascii"
	if width < 12 || height < 3 {
		return MenuLayout{Content: Clip("Resize terminal", width, ascii)}
	}
	capacity := MenuCapacity(width, height, action, status)
	statusLimit := max(0, height-1-capacity)
	if action != "" {
		statusLimit--
	}
	if status != "" && statusLimit == 0 {
		title = strings.SplitN(status, "\n", 2)[0]
	}
	lines := []string{Clip(" "+title, width, ascii)}
	layout := MenuLayout{}
	marker := "❯"
	if ascii {
		marker = ">"
	}
	for i := max(0, offset); i < len(options) && len(lines) <= capacity; i++ {
		mark := " "
		if i == index && !focusAction {
			mark = marker
		}
		line := " " + mark + " " + Clip(options[i], width-3, ascii)
		layout.Targets = append(layout.Targets, MenuTarget{Index: i, Y: len(lines), Start: 1, End: ansi.StringWidth(line)})
		lines = append(lines, line)
	}
	if action != "" {
		mark := " "
		if focusAction {
			mark = marker
		}
		line := " " + mark + " " + Clip(action, width-3, ascii)
		layout.Targets = append(layout.Targets, MenuTarget{Action: true, Y: len(lines), Start: 1, End: ansi.StringWidth(line)})
		lines = append(lines, line)
	}
	lines = append(lines, StatusLines(status, width, min(statusLimit, height-len(lines)))...)
	layout.Content = strings.Join(lines, "\n")
	return layout
}
