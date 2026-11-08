// Package storage validates snapshots and publishes them through atomic replacement.
package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"taskwarrior/internal/config"
	"taskwarrior/internal/tasks"

	"github.com/gofrs/flock"
)

type Result struct {
	Published bool
	Err       error
}

func Load(path string) (tasks.Data, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return tasks.Empty(), nil
	}
	if err != nil {
		return tasks.Data{}, fmt.Errorf("%s: %w", path, err)
	}
	var d tasks.Data
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err = dec.Decode(&d); err == nil {
		var extra any
		if tail := dec.Decode(&extra); tail != io.EOF {
			err = errors.New("trailing JSON data")
		}
	}
	if err == nil {
		err = validateFields(raw)
	}
	if err == nil {
		err = d.Validate()
	}
	if err != nil {
		return tasks.Data{}, fmt.Errorf("%s: %w", path, err)
	}
	return d, nil
}
func Save(path string, d tasks.Data) Result {
	if err := d.Validate(); err != nil {
		return Result{Err: fmt.Errorf("%s: %w", path, err)}
	}
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return Result{Err: err}
	}
	return writeAtomic(path, append(raw, '\n'), replaceFile, syncDirectory)
}
func writeAtomic(path string, raw []byte, replace func(string, string) error, syncDir func(string) error) (result Result) {
	defer func() {
		if result.Err != nil {
			result.Err = fmt.Errorf("%s: %w", path, result.Err)
		}
	}()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return Result{Err: err}
	}
	f, err := os.CreateTemp(dir, ".taskwarrior-*")
	if err != nil {
		return Result{Err: err}
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return Result{Err: err}
	}
	if err = replace(name, path); err != nil {
		return Result{Err: err}
	}
	if err = syncDir(dir); err != nil {
		return Result{Published: true, Err: fmt.Errorf("saved; crash durability uncertain: %w", err)}
	}
	return Result{Published: true}
}

// ResolvePath resolves existing symlink ancestors, including when the leaf does not exist.
func ResolvePath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	current := filepath.Clean(absolute)
	var tail []string
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return resolved, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		tail = append(tail, filepath.Base(current))
		current = parent
	}
}
func LockPaths(paths ...string) (func(), error) {
	resolved := make([]string, 0, len(paths))
	infos := []os.FileInfo{}
	for _, p := range paths {
		p, err := ResolvePath(p)
		if err != nil {
			return nil, err
		}
		info, statErr := os.Stat(p)
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return nil, statErr
		}
		for i, other := range resolved {
			if samePath(p, other) || (info != nil && infos[i] != nil && os.SameFile(info, infos[i])) {
				return nil, fmt.Errorf("data and config resolve to the same file: %s", p)
			}
		}
		resolved = append(resolved, p)
		infos = append(infos, info)
	}
	sort.Strings(resolved)
	var held []*flock.Flock
	release := func() {
		for i := len(held) - 1; i >= 0; i-- {
			_ = held[i].Close()
		}
		held = nil
	}
	for _, p := range resolved {
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			release()
			return nil, err
		}
		lock := flock.New(p+".lock", flock.SetPermissions(0600))
		ok, err := lock.TryLock()
		if err != nil || !ok {
			_ = lock.Close()
			release()
			if err != nil {
				return nil, fmt.Errorf("lock %s: %w", p, err)
			}
			return nil, fmt.Errorf("another instance is using %s", p)
		}
		held = append(held, lock)
	}
	return release, nil
}

func validateFields(raw []byte) error {
	var document map[string]json.RawMessage
	if err := json.Unmarshal(raw, &document); err != nil {
		return err
	}
	require := func(object map[string]json.RawMessage, fields ...string) error {
		for _, field := range fields {
			value, ok := object[field]
			if !ok || string(value) == "null" {
				return fmt.Errorf("required field %q is missing or null", field)
			}
		}
		return nil
	}
	if err := require(document, "version", "next_completion_seq", "next_group_seq", "tasks", "groups"); err != nil {
		return err
	}
	for _, collection := range []string{"tasks", "groups"} {
		var rows []map[string]json.RawMessage
		if err := json.Unmarshal(document[collection], &rows); err != nil {
			return err
		}
		for _, row := range rows {
			if collection == "groups" {
				if err := require(row, "id", "name"); err != nil {
					return err
				}
				continue
			}
			if err := require(row, "id", "text", "completed", "created_at", "updated_at"); err != nil {
				return err
			}
			var completed bool
			if err := json.Unmarshal(row["completed"], &completed); err != nil {
				return err
			}
			if _, present := row["completion_seq"]; !completed && present {
				return errors.New("active tasks must omit completion_seq")
			}
		}
	}
	return nil
}

func SaveConfig(path string, c config.Config) Result {
	raw, err := config.Encode(c)
	if err != nil {
		return Result{Err: fmt.Errorf("%s: %w", path, err)}
	}
	return writeAtomic(path, raw, replaceFile, syncDirectory)
}
