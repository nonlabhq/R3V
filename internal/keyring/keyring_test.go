package keyring

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
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
