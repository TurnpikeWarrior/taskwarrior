package style

import (
	"github.com/charmbracelet/x/ansi"
	"strings"
	"taskwarrior/internal/tasks"
	"testing"
)

func TestCompactLayoutAndHitTargets(t *testing.T) {
	rows := []tasks.Task{{ID: "a", Text: "active"}, {ID: "b", Text: "done", Completed: true}}
	r := Renderer{Appearance: Defaults()}
	l := r.List(rows, "a", 0, 40, 6, "")
	lines := strings.Split(ansi.Strip(l.Content), "\n")
	if len(lines) != 2 || lines[0] != " ❯ ☐ active" || lines[1] != "   ☑ done" {
		t.Fatal(lines)
	}
	if id, box := l.Hit(3, 0); id != "a" || !box {
		t.Fatal(id, box)
	}
	if id, box := l.Hit(6, 1); id != "b" || box {
		t.Fatal(id, box)
	}
	if id, _ := l.Hit(0, 0); id != "" {
		t.Fatal("padding is clickable")
	}
	if id, _ := l.Hit(30, 0); id != "" {
		t.Fatal("background is clickable")
	}
}
func TestClippingAndResize(t *testing.T) {
	r := Renderer{Appearance: Defaults()}
	rows := []tasks.Task{{ID: "a", Text: "界e\u0301👩‍💻abcdefghijk"}, {ID: "b", Text: "next"}}
	l := r.List(rows, "a", 0, 12, 3, "")
	for _, line := range strings.Split(l.Content, "\n") {
		if ansi.StringWidth(line) > 12 {
			t.Fatal(line)
		}
	}
	if !strings.Contains(l.Content, "…") {
		t.Fatal(l.Content)
	}
	if strings.Contains(l.Content, "\ufffd") {
		t.Fatal("broken grapheme")
	}
	l = r.List(rows, "a", 0, 11, 3, "")
	if id, _ := l.Hit(3, 0); id != "" {
		t.Fatal("tiny view has hit targets")
	}
	if ansi.StringWidth(l.Content) > 11 {
		t.Fatal(l.Content)
	}
	l = r.List(rows, "b", 1, 30, 3, "")
	if id, _ := l.Hit(3, 0); id != "b" {
		t.Fatal("viewport hit target", id)
	}
}
func TestMonochromeASCIIAndEmpty(t *testing.T) {
	a := Defaults()
	a.Symbols = "ascii"
	r := Renderer{Appearance: a, NoColor: true}
	l := r.List([]tasks.Task{{ID: "a", Text: "done", Completed: true}}, "a", 0, 30, 3, "")
	if l.Content != " > [x] done" {
		t.Fatalf("%q", l.Content)
	}
	l = r.List(nil, "", 0, 40, 3, "")
	if !strings.Contains(l.Content, "a") {
		t.Fatal("empty view omits add hint")
	}
}
