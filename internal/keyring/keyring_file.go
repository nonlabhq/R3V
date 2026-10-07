//go:build !windows

package keyring

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// Until the system stores (macOS Keychain, the Secret Service) are wired
// up: one file this user alone can read, beside R3V's settings.

var mu sync.Mutex

func path() string {
	d := Dir
	if d == "" {
		d, _ = os.UserConfigDir()
	}
	return filepath.Join(d, "keyring.json")
}

func load() (map[string]string, error) {
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

func set(target, _ string, secret string) error {
	mu.Lock()
	defer mu.Unlock()
	m, err := load()
	if err != nil {
		return err
	}
	m[target] = secret
	return save(m)
}

func get(target string) (string, error) {
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

func del(target string) error {
	mu.Lock()
	defer mu.Unlock()
	m, err := load()
	if err != nil {
		return err
	}
	delete(m, target)
	return save(m)
}
