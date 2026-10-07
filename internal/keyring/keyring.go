// Package keyring keeps secrets in the system's credential store: Windows
// Credential Manager; elsewhere, for now, a file only this user can read.
// R3V-Cloud's session tokens live here, never in teams.json.
package keyring

import "errors"

// Dir is where the file goes on systems without a store wired up yet (R3V's
// settings folder; set by the caller).
var Dir = ""

// ErrNotFound: nothing is kept under that name.
var ErrNotFound = errors.New("keyring: not found")

// Get reads the secret kept under target.
func Get(target string) (string, error) { return get(target) }

// Set keeps secret under target (replacing what was there); user is a
// label shown by the system's tools.
func Set(target, user, secret string) error {
	if secret == "" {
		return errors.New("keyring: empty secret")
	}
	return set(target, user, secret)
}

// Delete forgets target; absent is fine.
func Delete(target string) error { return del(target) }
