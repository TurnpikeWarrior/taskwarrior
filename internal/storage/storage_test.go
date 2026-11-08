package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"taskwarrior/internal/config"
	"taskwarrior/internal/tasks"
	"testing"
)

func TestRoundTripAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "tasks.json")
	d, err := Load(path)
	if err != nil || len(d.Tasks) != 0 {
		t.Fatal(d, err)
	}
	s := tasks.NewSession(d)
	id, _ := s.Add("first")
	s.Add("second")
	s.Toggle(id)
	result := Save(path, s.Data)
	if result.Err != nil || !result.Published {
		t.Fatal(result)
	}
	got, err := Load(path)
	if err != nil || !reflect.DeepEqual(got, s.Data) {
		t.Fatal(got, err)
	}
	if tasks.NewSession(got).Previous != nil {
		t.Fatal("undo persisted")
	}
	info, _ := os.Stat(path)
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		t.Fatal("nonprivate data mode", info.Mode())
	}
}
func TestInvalidDataLeavesFileIntact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	for _, raw := range []string{`{`, `{"version":2,"tasks":[],"groups":[]}`, `{"version":1,"next_completion_seq":1,"next_group_seq":1,"tasks":null,"groups":[]}`, `{"version":1,"next_completion_seq":1,"next_group_seq":1,"tasks":[],"groups":[]} {}`} {
		os.WriteFile(path, []byte(raw), 0600)
		if _, err := Load(path); err == nil || !strings.Contains(err.Error(), path) {
			t.Fatal("missing path diagnostic", err)
		}
		got, _ := os.ReadFile(path)
		if string(got) != raw {
			t.Fatal("bad file modified")
		}
	}
}
func TestAtomicFailureAndCleanup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.json")
	old := []byte("previous")
	os.WriteFile(path, old, 0600)
	result := writeAtomic(path, []byte("candidate"), func(string, string) error { return errors.New("injected replacement failure") }, syncDirectory)
	if result.Published || result.Err == nil {
		t.Fatal(result)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "previous" {
		t.Fatal(string(got))
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatal("temporary file leaked", entries)
	}
	result = writeAtomic(path, []byte("published"), replaceFile, func(string) error { return errors.New("sync failure") })
	if !result.Published || result.Err == nil {
		t.Fatal("durability uncertainty lost", result)
	}
	got, _ = os.ReadFile(path)
	if string(got) != "published" {
		t.Fatal(string(got))
	}
}
func TestLocksAndAliases(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	release, err := LockPaths(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if r, err := LockPaths(b); err == nil {
		r()
		t.Fatal("second instance acquired lock")
	}
	release()
	r, err := LockPaths(a, b)
	if err != nil {
		t.Fatal(err)
	}
	r()

	if _, err := LockPaths(a, a); err == nil {
		t.Fatal("same path accepted")
	}
	t.Run("hardlink", func(t *testing.T) {
		if err := os.WriteFile(a, []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Link(a, b); err != nil {
			if runtime.GOOS == "windows" {
				t.Skipf("hard links unavailable: %v", err)
			}
			t.Fatal(err)
		}
		if _, err := LockPaths(a, b); err == nil {
			t.Fatal("same inode accepted")
		}
	})
	t.Run("symlink", func(t *testing.T) {
		alias := filepath.Join(dir, "alias")
		if err := os.Symlink(dir, alias); err != nil {
			if runtime.GOOS == "windows" {
				t.Skipf("symlinks require suitable privileges: %v", err)
			}
			t.Fatal(err)
		}
		canonical, err := filepath.EvalSymlinks(dir)
		if err != nil {
			t.Fatal(err)
		}
		resolved, err := ResolvePath(filepath.Join(alias, "missing", "tasks.json"))
		if err != nil || resolved != filepath.Join(canonical, "missing", "tasks.json") {
			t.Fatal(resolved, err)
		}
	})
}

func TestRequiredTaskFieldsAndActiveSequenceOmission(t *testing.T) {
	s := tasks.NewSession(tasks.Empty())
	s.Add("task")
	raw, err := json.Marshal(s.Data)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	json.Unmarshal(raw, &doc)
	row := doc["tasks"].([]any)[0].(map[string]any)
	path := filepath.Join(t.TempDir(), "tasks.json")
	delete(row, "completed")
	raw, _ = json.Marshal(doc)
	os.WriteFile(path, raw, 0600)
	if _, err := Load(path); err == nil {
		t.Fatal("missing required completed field accepted")
	}
	row["completed"] = false
	row["completion_seq"] = 0
	raw, _ = json.Marshal(doc)
	os.WriteFile(path, raw, 0600)
	if _, err := Load(path); err == nil {
		t.Fatal("active completion_seq accepted")
	}
}

func TestConfigAtomicSaveAndJSONIsolation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	jsonPath := filepath.Join(dir, "tasks.json")
	Save(jsonPath, tasks.Empty())
	before, _ := os.ReadFile(jsonPath)
	c := config.Defaults()
	c.Appearance.RowGap = 2
	c.Keys["add"] = []string{"n"}
	result := SaveConfig(path, c)
	if result.Err != nil || !result.Published {
		t.Fatal(result)
	}
	loaded, err := config.Load(path)
	if err != nil || !reflect.DeepEqual(loaded, c) {
		t.Fatal(loaded, err)
	}
	old, _ := os.ReadFile(path)
	c.Appearance.RowGap = 9
	if result = SaveConfig(path, c); result.Published || result.Err == nil {
		t.Fatal("invalid config published", result)
	}
	after, _ := os.ReadFile(path)
	jsonAfter, _ := os.ReadFile(jsonPath)
	if string(old) != string(after) || string(before) != string(jsonAfter) {
		t.Fatal("config save altered another snapshot")
	}
}
