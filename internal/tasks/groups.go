package tasks

import (
	"errors"
	"math"
	"slices"
	"strings"
	"time"
	"unicode"
)

// ErrNoChange leaves the undo slot intact when the selected assignment is unchanged.
var ErrNoChange = errors.New("assignment unchanged")

func parseGroupSuffix(text string) (title, name string) {
	split := strings.LastIndexFunc(text, unicode.IsSpace)
	token := text[split+1:]
	if strings.HasPrefix(token, `\#`) && groupName.MatchString(token[2:]) {
		return text[:split+1] + token[1:], ""
	}
	if strings.HasPrefix(token, "#") && groupName.MatchString(token[1:]) {
		return strings.TrimSpace(text[:split+1]), token[1:]
	}
	return text, ""
}
func ValidateGroupName(name string) error {
	if !groupName.MatchString(name) {
		return errors.New("group name: 1–32 letters, digits, _ or -; start with a letter or digit")
	}
	return nil
}
func (d Data) Group(id string) (Group, bool) {
	for _, g := range d.Groups {
		if g.ID == id {
			return g, true
		}
	}
	return Group{}, false
}
func (d Data) SortedGroups() []Group {
	groups := append([]Group{}, d.Groups...)
	slices.SortFunc(groups, func(a, b Group) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	return groups
}
func (s *Session) planGroup(name string, keys []string) (Group, bool, error) {
	if name == "" {
		return Group{}, false, nil
	}
	if err := ValidateGroupName(name); err != nil {
		return Group{}, false, err
	}
	id := strings.ToLower(name)
	if group, ok := s.Data.Group(id); ok {
		return group, false, nil
	}
	if s.Data.NextGroupSeq == 0 || s.Data.NextGroupSeq == math.MaxUint64 {
		return Group{}, false, errors.New("group sequence exhausted")
	}
	group := Group{ID: id, Name: name}
	if len(keys) > 0 {
		group.ColorKey = keys[(s.Data.NextGroupSeq-1)%uint64(len(keys))]
	}
	return group, true, nil
}
func (s *Session) publishGroup(group Group, created bool) {
	if created {
		s.Data.Groups = append(s.Data.Groups, group)
		s.Data.NextGroupSeq++
	}
}
func (s *Session) AssignGroup(taskID, name string, paletteKeys ...string) error {
	i := s.index(taskID)
	if i < 0 {
		return errors.New("task not found")
	}
	group, created, err := s.planGroup(name, paletteKeys)
	if err != nil {
		return err
	}
	if s.Data.Tasks[i].GroupID == group.ID {
		return ErrNoChange
	}
	s.remember()
	s.publishGroup(group, created)
	s.Data.Tasks[i].GroupID = group.ID
	s.Data.Tasks[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}
func (s *Session) SetTaskColor(taskID, key string) error {
	i := s.index(taskID)
	if i < 0 {
		return errors.New("task not found")
	}
	if key != "" {
		if err := ValidateText(key); err != nil {
			return err
		}
	}
	if s.Data.Tasks[i].ColorKey == key {
		return ErrNoChange
	}
	s.remember()
	s.Data.Tasks[i].ColorKey = key
	s.Data.Tasks[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}
func (s *Session) SetGroupColor(groupID, key string) error {
	if key != "" {
		if err := ValidateText(key); err != nil {
			return err
		}
	}
	for i, group := range s.Data.Groups {
		if group.ID != groupID {
			continue
		}
		if group.ColorKey == key {
			return ErrNoChange
		}
		s.remember()
		s.Data.Groups[i].ColorKey = key
		return nil
	}
	return errors.New("group not found")
}
