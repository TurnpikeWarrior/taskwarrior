package style

import (
	"github.com/charmbracelet/x/ansi"
	"strings"
	"taskwarrior/internal/tasks"
	"testing"
)

func TestPaletteDefaultsAndResolution(t *testing.T) {
	p := DefaultPalette()
	want := []Color{{"blue", "Blue", "#268BFF"}, {"green", "Green", "#00D66B"}, {"yellow", "Yellow", "#FFE135"}, {"orange", "Orange", "#FF8C00"}, {"red", "Red", "#FF4040"}}
	for i, c := range want {
		if p[i] != c {
			t.Fatal(p)
		}
	}
	groups := []tasks.Group{{ID: "mada", Name: "MADA", ColorKey: "blue"}}
	for _, c := range []struct {
		task tasks.Task
		want string
	}{
		{tasks.Task{}, ""},
		{tasks.Task{GroupID: "mada"}, "#268BFF"},
		{tasks.Task{GroupID: "mada", ColorKey: "red"}, "#FF4040"},
		{tasks.Task{GroupID: "mada", ColorKey: "missing"}, ""},
		{tasks.Task{GroupID: "mada", ColorKey: "red", Completed: true}, ""},
		{tasks.Task{GroupID: "missing"}, ""},
	} {
		if got := ResolveColor(c.task, groups, p); got != c.want {
			t.Fatal(c, got)
		}
	}
	if got := ResolveColor(tasks.Task{GroupID: "mada"}, groups, nil); got != "" {
		t.Fatal("empty palette fallback", got)
	}
	missing := tasks.Task{ColorKey: "custom-old", GroupID: "mada"}
	p = append(p, Color{"custom-old", "Restored", "#AA55FF"})
	if ResolveColor(missing, groups, p) != "#AA55FF" {
		t.Fatal("restored key not resolved")
	}
}
func TestColoredRowsSuffixClippingAndCompletion(t *testing.T) {
	r := Renderer{Appearance: Defaults(), Palette: DefaultPalette()}
	groups := []tasks.Group{{ID: "mada", Name: "MADA", ColorKey: "blue"}}
	rows := []tasks.Task{{ID: "a", Text: "agenda", GroupID: "mada", ColorKey: "red"}}
	colored := r.List(rows, "a", 0, 40, 6, "", groups...).Content
	if ansi.Strip(colored) != " ❯ ☐ agenda  #MADA" || !strings.Contains(colored, "38;2;255;64;64") {
		t.Fatalf("override not colored: %q", colored)
	}
	r.NoColor = true
	plain := r.List(rows, "a", 0, 40, 6, "", groups...).Content
	if strings.Contains(plain, "\x1b") || !strings.Contains(plain, "#MADA") {
		t.Fatal(plain)
	}
	clipped := r.List(rows, "a", 0, 12, 3, "", groups...).Content
	if strings.Contains(clipped, "#MADA") || !strings.Contains(clipped, "agenda") || ansi.StringWidth(clipped) > 12 {
		t.Fatal("suffix did not yield title room", clipped)
	}
	r.NoColor = false
	rows[0].Completed = true
	completed := r.List(rows, "a", 0, 40, 6, "", groups...).Content
	if strings.Contains(completed, "38;2") || !strings.Contains(completed, "\x1b[2") || !strings.Contains(completed, "9") {
		t.Fatalf("completed treatment: %q", completed)
	}
}
func TestMenuLayoutAndViewportHitTargets(t *testing.T) {
	r := Renderer{Appearance: Defaults(), NoColor: true}
	l := r.Menu("Groups", []string{"Ungrouped", "Alpha", "MADA", "New group"}, 2, 1, 30, 4, "Group color: MADA", true, "")
	if !strings.Contains(l.Content, "Alpha") || !strings.Contains(l.Content, "Group color: MADA") {
		t.Fatal(l.Content)
	}
	if index, action, ok := l.Hit(5, 1); !ok || action || index != 1 {
		t.Fatal(index, action, ok)
	}
	if _, action, ok := l.Hit(5, 3); !ok || !action {
		t.Fatal("action target", action, ok)
	}
	if _, _, ok := l.Hit(0, 1); ok {
		t.Fatal("padding target")
	}
	if _, _, ok := l.Hit(29, 1); ok {
		t.Fatal("background target")
	}
	l = r.Menu("Groups", []string{"MADA"}, 0, 0, 10, 2, "", false, "")
	if _, _, ok := l.Hit(3, 1); ok {
		t.Fatal("tiny menu target")
	}
}
