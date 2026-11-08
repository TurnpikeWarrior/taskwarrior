package tasks

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestInlineGroupParsing(t *testing.T) {
	for _, c := range []struct {
		input, title, group string
		invalid             bool
	}{
		{"agenda #MADA", "agenda", "MADA", false},
		{"earlier #tag stays #MADA", "earlier #tag stays", "MADA", false},
		{`agenda \#MADA`, "agenda #MADA", "", false},
		{`\#MADA`, "#MADA", "", false},
		{"#MADA", "", "", true},
		{"agenda#MADA", "agenda#MADA", "", false},
		{"agenda #bad!", "agenda #bad!", "", false},
		{"agenda #_bad", "agenda #_bad", "", false},
		{"agenda #" + strings.Repeat("x", 33), "agenda #" + strings.Repeat("x", 33), "", false},
		{"agenda\t#Team_2-x\n", "agenda", "Team_2-x", false},
		{strings.Repeat("界", 500) + " #MADA", strings.Repeat("界", 500), "MADA", false},
		{"agenda\u0085 #MADA", "", "", true},
	} {
		t.Run(c.input[:min(40, len(c.input))], func(t *testing.T) {
			s := NewSession(Empty())
			_, err := s.Add(c.input, "blue", "green")
			if c.invalid {
				if err == nil || len(s.Data.Groups) != 0 || s.Previous != nil {
					t.Fatal("invalid input mutated state", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			task := s.Data.Tasks[0]
			if task.Text != c.title {
				t.Fatalf("title %q", task.Text)
			}
			if c.group == "" {
				if task.GroupID != "" || len(s.Data.Groups) != 0 {
					t.Fatal("unexpected group")
				}
			} else if len(s.Data.Groups) != 1 || s.Data.Groups[0].Name != c.group || task.GroupID != strings.ToLower(c.group) {
				t.Fatal("inline group missing", s.Data)
			}
		})
	}
}

func TestGroupReuseSequenceAndCompositeUndo(t *testing.T) {
	s := NewSession(Empty())
	first, _ := s.Add("first #MADA", "blue", "green")
	s.Add("second #mada", "blue", "green")
	if len(s.Data.Groups) != 1 || s.Data.Groups[0].Name != "MADA" || s.Data.Groups[0].ColorKey != "blue" || s.Data.NextGroupSeq != 2 {
		t.Fatal(s.Data)
	}
	s.Add("third #Zulu", "blue", "green")
	s.Add("fourth #alpha", "blue", "green")
	if s.Data.Groups[1].ColorKey != "green" || s.Data.Groups[2].ColorKey != "blue" {
		t.Fatal("palette did not cycle")
	}
	sorted := s.Data.SortedGroups()
	if sorted[0].Name != "alpha" || sorted[1].Name != "MADA" || sorted[2].Name != "Zulu" {
		t.Fatal(sorted)
	}
	before := s.Data.Clone()
	s.AssignGroup(first, "New", "red")
	if err := s.Undo(); err != nil || !reflect.DeepEqual(before, s.Data) {
		t.Fatal("assignment/group creation not one undo", err)
	}
	s.Add("composite #BrandNew", "red")
	s.Undo()
	if !reflect.DeepEqual(before, s.Data) {
		t.Fatal("inline creation not one undo")
	}
	s.Delete(first)
	if len(s.Data.Groups) != 3 {
		t.Fatal("empty group was removed")
	}
	if err := s.Data.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestAssignmentOverrideAndColorUndo(t *testing.T) {
	s := NewSession(Empty())
	id, _ := s.Add("task")
	if err := s.SetTaskColor(id, "red"); err != nil {
		t.Fatal(err)
	}
	if err := s.AssignGroup(id, "MADA", "blue"); err != nil {
		t.Fatal(err)
	}
	if s.Data.Tasks[0].ColorKey != "red" {
		t.Fatal("group cleared override")
	}
	before := s.Data.Clone()
	s.AssignGroup(id, "")
	s.Undo()
	if !reflect.DeepEqual(before, s.Data) {
		t.Fatal("group undo")
	}
	s.SetGroupColor("mada", "")
	s.Undo()
	if s.Data.Groups[0].ColorKey != "blue" {
		t.Fatal("group color undo")
	}
	s.SetTaskColor(id, "")
	s.Undo()
	if s.Data.Tasks[0].ColorKey != "red" {
		t.Fatal("task override undo")
	}
	s.SetGroupColor("mada", "removed-key")
	if s.Data.Validate() != nil {
		t.Fatal("missing palette keys must remain valid")
	}
	prior := s.Clone()
	for _, name := range []string{"bad group", "#MADA", "_bad", strings.Repeat("x", 33), "bad\x1b"} {
		if s.AssignGroup(id, name) == nil {
			t.Fatal("invalid name", name)
		}
	}
	if !reflect.DeepEqual(prior, s) {
		t.Fatal("invalid group overwrote undo")
	}
	if !errors.Is(s.AssignGroup(id, "mada"), ErrNoChange) {
		t.Fatal("same group should not mutate")
	}
	if !reflect.DeepEqual(prior, s) {
		t.Fatal("no-op overwrote undo")
	}
	if s.SetGroupColor("missing", "red") == nil || s.SetTaskColor("missing", "red") == nil {
		t.Fatal("unknown ID accepted")
	}
}

func TestEmptyPaletteAndExistingAssignments(t *testing.T) {
	s := NewSession(Empty())
	s.Add("first #One")
	if s.Data.Groups[0].ColorKey != "" {
		t.Fatal("empty palette assigned color")
	}
	s.Add("next #Two", "orange")
	if s.Data.Groups[1].ColorKey != "orange" || s.Data.Groups[0].ColorKey != "" {
		t.Fatal("palette change recolored existing groups")
	}
}

func TestEmptyGroupRemainsAfterLastTaskIsDeleted(t *testing.T) {
	s := NewSession(Empty())
	id, _ := s.Add("only #MADA", "blue")
	s.Delete(id)
	if len(s.Data.Tasks) != 0 || len(s.Data.Groups) != 1 || s.Data.Groups[0].Name != "MADA" || s.Data.NextGroupSeq != 2 {
		t.Fatal("empty group removed", s.Data)
	}
	id, _ = s.Add("new task")
	s.AssignGroup(id, "mada", "red")
	if len(s.Data.Groups) != 1 || s.Data.Groups[0].ColorKey != "blue" {
		t.Fatal("empty group not reused")
	}
}
