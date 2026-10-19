// Package keyring keeps secrets in the system's credential store: Windows
// Credential Manager, the macOS Keychain, the Secret Service on Linux;
// elsewhere (or without one) a file only this user can read.
// R3V-Cloud's session tokens live here, never in teams.json.
package keyring

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
)

// Dir is where the file goes on systems without a store wired up yet (R3V's
// settings folder; set by the caller).
var Dir = ""

// ErrNotFound: nothing is kept under that name.
var ErrNotFound = errors.New("keyring: not found")

// Get reads the secret kept under target.
func Get(target string) (string, error) { return get(scoped(target)) }

// Set keeps secret under target (replacing what was there); user is a
// label shown by the system's tools.
func Set(target, user, secret string) error {
	if secret == "" {
		return errors.New("keyring: empty secret")
	}
	return set(scoped(target), user, secret)
}

// Delete forgets target; absent is fine.
func Delete(target string) error { return del(scoped(target)) }

// scoped is the name target is kept under. The system's store is one per
// user, not per settings folder: with R3V_CONFIG_DIR set (tests, the demo,
// a second setup) the name carries that folder, so such a run never reads
// or replaces the user's own secrets. Unset, names are as they always were.
func scoped(target string) string {
	d := os.Getenv("R3V_CONFIG_DIR")
	if d == "" {
		return target
	}
	if abs, err := filepath.Abs(d); err == nil {
		d = abs
	}
	h := sha256.Sum256([]byte(filepath.Clean(d)))
	return target + "@" + hex.EncodeToString(h[:6])
}
