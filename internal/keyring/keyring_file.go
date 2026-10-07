//go:build !windows

package keyring

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// Where no system store can be used (a Linux without the Secret Service,
// other systems): one file this user alone can read, beside R3V's
// settings.

var mu sync.Mutex

func path() string {
	d := Dir
	if d == "" {
		d, _ = os.UserConfigDir()
	}
	return filepath.Join(d, "keyring.json")
}

func load() (map[string]string, error) {
	// (made readable by others since, e.g. copied: this user's again)
	if fi, err := os.Stat(path()); err == nil && fi.Mode().Perm()&0o077 != 0 {
		os.Chmod(path(), 0o600)
	}
	data, err := os.ReadFile(path())
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	return m, json.Unmarshal(data, &m)
}

func save(m map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path()), 0o700); err != nil {
		return err
	}
	data, _ := json.Marshal(m)
	tmp := path() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path())
}

func fileSet(target, secret string) error {
	mu.Lock()
	defer mu.Unlock()
	m, err := load()
	if err != nil {
		return err
	}
	m[target] = secret
	return save(m)
}

func fileGet(target string) (string, error) {
	mu.Lock()
	defer mu.Unlock()
	m, err := load()
	if err != nil {
		return "", err
	}
	s, ok := m[target]
	if !ok {
		return "", ErrNotFound
	}
	return s, nil
}

func fileDel(target string) error {
	mu.Lock()
	defer mu.Unlock()
	m, err := load()
	if err != nil {
		return err
	}
	delete(m, target)
	return save(m)
}
