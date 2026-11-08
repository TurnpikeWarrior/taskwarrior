package tasks

import (
	"reflect"
	"strings"
	"testing"
)

func TestOrderCompletionReopenAndUndo(t *testing.T) {
	s := NewSession(Empty())
	a, _ := s.Add("first")
	b, _ := s.Add("second")
	c, _ := s.Add("third")
	ids := func() []string {
		var out []string
		for _, v := range s.Data.Visible(false) {
			out = append(out, v.ID)
		}
		return out
	}
	if !reflect.DeepEqual(ids(), []string{c, b, a}) {
		t.Fatal("insertion", ids())
	}
	if err := s.Toggle(b); err != nil {
		t.Fatal(err)
	}
	if err := s.Toggle(a); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ids(), []string{c, a, b}) {
		t.Fatal("completion order", ids())
	}
	if len(s.Data.Visible(true)) != 1 {
		t.Fatal("hidden completed")
	}
	s.Toggle(b)
	if !reflect.DeepEqual(ids(), []string{c, b, a}) {
		t.Fatal("reopen order", ids())
	}
	before := s.Data.Clone()
	s.Delete(b)
	if err := s.Undo(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.Data, before) {
		t.Fatal("undo did not restore entire snapshot")
	}
	if s.Undo() == nil {
		t.Fatal("undo was not consumed")
	}
}

func TestUndoAndFailedMutation(t *testing.T) {
	s := NewSession(Empty())
	id, _ := s.Add("duplicate")
	s.Add("duplicate")
	if _, err := s.Add(" \t\n "); err == nil {
		t.Fatal("empty accepted")
	}
	if s.Delete("missing") == nil {
		t.Fatal("unknown accepted")
	}
	if err := s.Undo(); err != nil {
		t.Fatal(err)
	}
	if len(s.Data.Tasks) != 1 || s.Data.Tasks[0].ID != id {
		t.Fatal("failed command destroyed undo")
	}
	s.Toggle(id)
	s.Undo()
	if s.Data.Tasks[0].Completed || s.Data.Tasks[0].CompletionSeq != 0 {
		t.Fatal("toggle undo")
	}
	s.Undo()
	if len(s.Data.Tasks) != 1 {
		t.Fatal("second undo changed data")
	}
}

func TestTitleValidation(t *testing.T) {
	s := NewSession(Empty())
	id, err := s.Add("  hello\nworld\t!  ")
	if err != nil {
		t.Fatal(err)
	}
	if s.Data.Tasks[0].Text != "hello world !" {
		t.Fatal(s.Data.Tasks[0].Text)
	}
	for _, text := range []string{"\x1b[31m", "a\x00b", "a\u0085b", strings.Repeat("界", 501)} {
		if _, err := s.Add(text); err == nil {
			t.Fatalf("accepted %q", text)
		}
	}
	if id == "" {
		t.Fatal("missing UUID")
	}
	if _, err := s.Add(strings.Repeat("界", 500)); err != nil {
		t.Fatal(err)
	}
}

func TestValidate(t *testing.T) {
	s := NewSession(Empty())
	id, _ := s.Add("task")
	s.Toggle(id)
	if err := s.Data.Validate(); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		edit func(*Data)
	}{
		{"version", func(d *Data) { d.Version = 99 }},
		{"duplicate", func(d *Data) { d.Tasks = append(d.Tasks, d.Tasks[0]) }},
		{"sequence", func(d *Data) { d.NextCompletionSeq = 1 }},
		{"missing completion", func(d *Data) { d.Tasks[0].CompletionSeq = 0 }},
		{"active sequence", func(d *Data) { d.Tasks[0].Completed = false }},
		{"group reference", func(d *Data) { d.Tasks[0].GroupID = "missing" }},
		{"control", func(d *Data) { d.Tasks[0].Text = "bad\x1b" }},
		{"timestamp", func(d *Data) { d.Tasks[0].CreatedAt = "yesterday" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := s.Data.Clone()
			c.edit(&d)
			if d.Validate() == nil {
				t.Fatal("invalid data accepted")
			}
		})
	}
}
