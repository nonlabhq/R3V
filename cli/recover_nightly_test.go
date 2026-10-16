//go:build nightly

package cli

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"testing"
)

// A goto parking uncommitted changes (Nightly), killed half-way: the next
// command puts the files back with the changes, as they were.
func TestKilledGotoWithChangesRecovered(t *testing.T) {
	dir, fake, one, two, before, _ := switchedProject(t)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("my edit"), 0o644)
	before["notes.txt"] = "my edit"
	if !runKilledCLI(t, dir, 2, "goto", two) {
		t.Fatal("it finished")
	}
	st, writes, rep := statusAfter(t, dir, fake)
	if st.Recovered == nil || st.Recovered.Action != "restored" || !st.Recovered.Changes || st.Recovered.Version != one {
		t.Fatalf("status: %s", rep.Result)
	}
	if got := folderFiles(t, dir); !maps.Equal(got, before) || len(st.Changes) != 1 || st.Changes[0].Path != "notes.txt" {
		t.Fatalf("after status: %d changes %+v", len(st.Changes), st.Changes)
	}
	if writes != 0 {
		t.Errorf("sent %d writes", writes)
	}
	// (the changes are in the folder again: not parked as well)
	exit, rep := run(t, dir, "parked")
	var p struct{ Parked []parkedJSON }
	json.Unmarshal(rep.Result, &p)
	if exit != 0 || len(p.Parked) != 0 {
		t.Fatalf("parked: %d %s", exit, rep.Result)
	}
}
