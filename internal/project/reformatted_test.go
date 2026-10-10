package project

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// A code formatter run over the project folder lays .r3v's version records
// out again: R3V puts them back as they were (their ids prove it), never
// reports them damaged and never shares them so; a record whose values
// changed stays damaged.
func TestReformattedRecordPutBack(t *testing.T) {
	root := newProject(t)
	r, _ := Init(root, "yi")
	write(t, root, "notes.txt", "first")
	mustSnapshot(t, r, "first")
	write(t, root, "notes.txt", "second")
	mustSnapshot(t, r, "second")
	id := r.Head()
	path := r.snapshotPath(id)
	made, _ := os.ReadFile(path)

	var compact bytes.Buffer
	json.Compact(&compact, made)
	os.WriteFile(path, compact.Bytes(), 0o644)
	rep, err := r.Verify(false)
	if err != nil || len(rep.Problems) != 0 {
		t.Fatalf("verify: %v %+v", err, rep.Problems)
	}
	if now, _ := os.ReadFile(path); !bytes.Equal(now, made) {
		t.Fatalf("not put back:\n%s", now)
	}

	os.WriteFile(path, bytes.ReplaceAll(made, []byte("\n"), []byte("\r\n")), 0o644)
	if data, err := r.sharedSnapshot(id); err != nil || !bytes.Equal(data, made) {
		t.Fatalf("shared: %v", err)
	}

	os.WriteFile(path, []byte(strings.Replace(string(made), `"second"`, `"other"`, 1)), 0o644)
	if _, err := r.sharedSnapshot(id); err == nil {
		t.Fatal("a changed record would be shared")
	}
	if rep, _ := r.Verify(false); len(rep.Problems) != 1 {
		t.Fatalf("a changed record: %+v", rep.Problems)
	}
}
