package keyring

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	b := make([]byte, 6)
	rand.Read(b)
	target := "R3V-test/" + hex.EncodeToString(b)
	t.Cleanup(func() { Delete(target) })

	if _, err := Get(target); !errors.Is(err, ErrNotFound) {
		t.Fatalf("before: %v, want ErrNotFound", err)
	}
	if err := Set(target, "tester", "first"); err != nil {
		t.Fatal(err)
	}
	if err := Set(target, "tester", "second"); err != nil { // replaces
		t.Fatal(err)
	}
	if got, err := Get(target); err != nil || got != "second" {
		t.Fatalf("Get = %q, %v", got, err)
	}
	if err := Delete(target); err != nil {
		t.Fatal(err)
	}
	if _, err := Get(target); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after Delete: %v, want ErrNotFound", err)
	}
	if err := Delete(target); err != nil {
		t.Errorf("deleting what's gone: %v", err)
	}
	if err := Set(target, "tester", ""); err == nil {
		t.Error("an empty secret was kept")
	}
}

// A run with its own settings folder (tests, the demo) keeps its secrets
// apart: another folder, or the user's own store, sees none of them.
func TestScopedByConfigDir(t *testing.T) {
	b := make([]byte, 6)
	rand.Read(b)
	target := "R3V-test/" + hex.EncodeToString(b)
	one, two := t.TempDir(), t.TempDir()

	t.Setenv("R3V_CONFIG_DIR", one)
	t.Cleanup(func() { os.Setenv("R3V_CONFIG_DIR", one); Delete(target) })
	if scoped(target) == target {
		t.Fatal("the name doesn't carry the settings folder")
	}
	if err := Set(target, "tester", "kept-in-one"); err != nil {
		t.Fatal(err)
	}

	t.Setenv("R3V_CONFIG_DIR", two)
	if _, err := Get(target); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another settings folder: %v, want ErrNotFound", err)
	}
	t.Setenv("R3V_CONFIG_DIR", "")
	if scoped(target) != target {
		t.Fatal("without R3V_CONFIG_DIR the name changed")
	}
	if _, err := Get(target); !errors.Is(err, ErrNotFound) {
		t.Fatalf("the user's own store: %v, want ErrNotFound", err)
	}

	t.Setenv("R3V_CONFIG_DIR", one)
	if got, err := Get(target); err != nil || got != "kept-in-one" {
		t.Fatalf("back in the first folder: err %v, found %v", err, got == "kept-in-one")
	}
}
