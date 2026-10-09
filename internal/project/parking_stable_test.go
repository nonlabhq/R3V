//go:build !nightly

package project

import (
	"errors"
	"testing"
)

// Stable doesn't park changes: a switch with changes still asks.
func TestStableParksNothing(t *testing.T) {
	if Parking {
		t.Fatal("parking is on in Stable")
	}
	r, v1, _ := twoLocal(t)
	changesOn(t, r)
	if _, _, err := r.GoTo(v1.ID, false); !errors.Is(err, ErrDirty) {
		t.Fatalf("going with changes: %v", err)
	}
	if len(r.ParkedSets()) != 0 {
		t.Fatal("parked")
	}
}
