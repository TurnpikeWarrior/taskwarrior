package style

import "taskwarrior/internal/tasks"

type Color struct {
	Key  string `toml:"key"`
	Name string `toml:"name"`
	Hex  string `toml:"hex"`
}
type Palette []Color

func DefaultPalette() Palette {
	return Palette{
		{"blue", "Blue", "#268BFF"},
		{"green", "Green", "#00D66B"},
		{"yellow", "Yellow", "#FFE135"},
		{"orange", "Orange", "#FF8C00"},
		{"red", "Red", "#FF4040"},
	}
}
func (p Palette) Keys() []string {
	keys := make([]string, len(p))
	for i, c := range p {
		keys[i] = c.Key
	}
	return keys
}
func (p Palette) Find(key string) (Color, bool) {
	for _, c := range p {
		if c.Key == key {
			return c, true
		}
	}
	return Color{}, false
}
func ResolveColor(task tasks.Task, groups []tasks.Group, palette Palette) string {
	if task.Completed {
		return ""
	}
	key := task.ColorKey
	if key == "" {
		for _, group := range groups {
			if group.ID == task.GroupID {
				key = group.ColorKey
				break
			}
		}
	}
	if color, ok := palette.Find(key); ok {
		return color.Hex
	}
	return ""
}
