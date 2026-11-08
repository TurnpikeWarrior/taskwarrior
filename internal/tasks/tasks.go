// Package tasks owns task state without terminal or filesystem dependencies.
package tasks

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

type Task struct {
	ID            string `json:"id"`
	Text          string `json:"text"`
	Completed     bool   `json:"completed"`
	GroupID       string `json:"group_id,omitempty"`
	ColorKey      string `json:"color_key,omitempty"`
	CompletionSeq uint64 `json:"completion_seq,omitempty"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}
type Group struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ColorKey string `json:"color_key,omitempty"`
}
type Data struct {
	Version           int     `json:"version"`
	NextCompletionSeq uint64  `json:"next_completion_seq"`
	NextGroupSeq      uint64  `json:"next_group_seq"`
	Tasks             []Task  `json:"tasks"`
	Groups            []Group `json:"groups"`
}

func Empty() Data {
	return Data{Version: 1, NextCompletionSeq: 1, NextGroupSeq: 1, Tasks: []Task{}, Groups: []Group{}}
}
func (d Data) Clone() Data {
	d.Tasks = append([]Task{}, d.Tasks...)
	d.Groups = append([]Group{}, d.Groups...)
	return d
}
func (d Data) Visible(hide bool) []Task {
	active, done := []Task{}, []Task{}
	for _, t := range d.Tasks {
		if t.Completed {
			if !hide {
				done = append(done, t)
			}
		} else {
			active = append(active, t)
		}
	}
	slices.SortFunc(done, func(a, b Task) int {
		if a.CompletionSeq > b.CompletionSeq {
			return -1
		}
		if a.CompletionSeq < b.CompletionSeq {
			return 1
		}
		return 0
	})
	return append(active, done...)
}
func ValidateText(text string) error {
	if !utf8.ValidString(text) {
		return errors.New("text is not valid UTF-8")
	}
	if strings.TrimSpace(text) == "" {
		return errors.New("title is required")
	}
	if utf8.RuneCountInString(text) > 500 {
		return errors.New("title exceeds 500 characters")
	}
	for _, r := range text {
		if unicode.IsControl(r) {
			return errors.New("text contains terminal control characters")
		}
	}
	return nil
}

var groupName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,31}$`)

func (d Data) Validate() error {
	if d.Version != 1 {
		return fmt.Errorf("unsupported schema version %d", d.Version)
	}
	if d.NextCompletionSeq == 0 || d.NextGroupSeq == 0 {
		return errors.New("sequence counters must be positive")
	}
	if d.Tasks == nil || d.Groups == nil {
		return errors.New("tasks and groups arrays are required")
	}
	groups := map[string]bool{}
	for _, g := range d.Groups {
		if !groupName.MatchString(g.Name) || g.ID != strings.ToLower(g.Name) || groups[g.ID] {
			return fmt.Errorf("invalid or duplicate group %q", g.ID)
		}
		if g.ColorKey != "" {
			if err := ValidateText(g.ColorKey); err != nil {
				return err
			}
		}
		groups[g.ID] = true
	}
	if d.NextGroupSeq <= uint64(len(groups)) {
		return errors.New("next_group_seq is out of bounds")
	}
	ids := map[string]bool{}
	seqs := map[uint64]bool{}
	for _, t := range d.Tasks {
		parsed, err := uuid.Parse(t.ID)
		if err != nil || parsed.String() != t.ID || ids[t.ID] {
			return fmt.Errorf("invalid or duplicate task ID %q", t.ID)
		}
		ids[t.ID] = true
		if err := ValidateText(t.Text); err != nil {
			return fmt.Errorf("task %s: %w", t.ID, err)
		}
		for _, stamp := range []string{t.CreatedAt, t.UpdatedAt} {
			v, err := time.Parse(time.RFC3339, stamp)
			if err != nil || v.Location() != time.UTC {
				return fmt.Errorf("task %s: timestamp must be UTC RFC3339", t.ID)
			}
		}
		if t.ColorKey != "" {
			if err := ValidateText(t.ColorKey); err != nil {
				return err
			}
		}
		if t.GroupID != "" && !groups[t.GroupID] {
			return fmt.Errorf("task %s: unknown group", t.ID)
		}
		if t.Completed {
			if t.CompletionSeq == 0 || t.CompletionSeq >= d.NextCompletionSeq || seqs[t.CompletionSeq] {
				return fmt.Errorf("task %s: invalid completion sequence", t.ID)
			}
			seqs[t.CompletionSeq] = true
		} else if t.CompletionSeq != 0 {
			return fmt.Errorf("active task %s has completion sequence", t.ID)
		}
	}
	return nil
}

type Session struct {
	revision uint64
	Data     Data
	Previous *Data
}

func NewSession(d Data) Session { return Session{Data: d.Clone()} }
func (s Session) Clone() Session {
	s.Data = s.Data.Clone()
	if s.Previous != nil {
		d := s.Previous.Clone()
		s.Previous = &d
	}
	return s
}

// Revision changes after successful commands and is never persisted.
func (s Session) Revision() uint64 { return s.revision }
func (s *Session) remember()       { d := s.Data.Clone(); s.Previous = &d; s.revision++ }
func (s *Session) Add(text string, paletteKeys ...string) (string, error) {
	text = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ", "\t", " ").Replace(text)
	// Validate controls before trimming: trimming must not hide a C1 control.
	for _, r := range text {
		if unicode.IsControl(r) {
			return "", errors.New("text contains terminal control characters")
		}
	}
	text, name := parseGroupSuffix(strings.TrimSpace(text))
	if err := ValidateText(text); err != nil {
		return "", err
	}
	group, created, err := s.planGroup(name, paletteKeys)
	if err != nil {
		return "", err
	}
	id := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339)
	s.remember()
	s.publishGroup(group, created)
	s.Data.Tasks = append([]Task{{ID: id, Text: text, GroupID: group.ID, CreatedAt: now, UpdatedAt: now}}, s.Data.Tasks...)
	return id, nil
}
func (s *Session) index(id string) int {
	for i, t := range s.Data.Tasks {
		if t.ID == id {
			return i
		}
	}
	return -1
}
func (s *Session) Toggle(id string) error {
	i := s.index(id)
	if i < 0 {
		return errors.New("task not found")
	}
	if !s.Data.Tasks[i].Completed && s.Data.NextCompletionSeq == math.MaxUint64 {
		return errors.New("completion sequence exhausted")
	}
	s.remember()
	t := &s.Data.Tasks[i]
	t.Completed = !t.Completed
	t.CompletionSeq = 0
	if t.Completed {
		t.CompletionSeq = s.Data.NextCompletionSeq
		s.Data.NextCompletionSeq++
	}
	t.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}
func (s *Session) Delete(id string) error {
	i := s.index(id)
	if i < 0 {
		return errors.New("task not found")
	}
	s.remember()
	s.Data.Tasks = append(s.Data.Tasks[:i:i], s.Data.Tasks[i+1:]...)
	return nil
}
func (s *Session) Undo() error {
	if s.Previous == nil {
		return errors.New("nothing to undo")
	}
	s.Data = s.Previous.Clone()
	s.Previous = nil
	s.revision++
	return nil
}
