package project

import (
	"errors"
	"os"
	"testing"
)

// A check's result is recorded beside a version (the latest per check),
// with the changes not committed counted; names and states R3V doesn't
// take are refused; results go with their version.
func TestResults(t *testing.T) {
	r, _ := Init(newProject(t), "yi")
	write(t, r.Root, "notes.txt", "one")
	first := mustSnapshot(t, r, "first")
	if _, _, err := r.SetResult("", "tests", "fail", "2 failed"); err != nil {
		t.Fatal(err)
	}
	write(t, r.Root, "notes.txt", "fixed, not committed")
	id, res, err := r.SetResult("", "tests", "pass", "all passed")
	if err != nil || id != first.ID || res.Changes != 1 {
		t.Fatalf("pass: %s %+v %v", id, res, err)
	}
	r.SetResult(first.ID[:10], "build", "running", "")
	all := r.Results(first.ID)
	if len(all) != 2 || all["tests"].State != "pass" || all["build"].State != "running" ||
		ResultNames(all)[0] != "build" {
		t.Fatalf("results: %+v", all)
	}
	for _, bad := range [][2]string{{"Tests", "pass"}, {"tests", "ok"}, {"", "pass"}, {"a b", "fail"}} {
		if _, _, err := r.SetResult("", bad[0], bad[1], ""); !errors.Is(err, ErrBadResult) {
			t.Errorf("%v taken: %v", bad, err)
		}
	}
	if len(r.Results("0123456789")) != 0 {
		t.Error("results for a version that has none")
	}
	// A version gone (undone, its record removed): its results go too.
	second := mustSnapshot(t, r, "second")
	r.SetResult("", "tests", "pass", "")
	r.setHead(first.ID)
	os.Remove(r.snapshotPath(second.ID))
	r.pruneResults()
	if len(r.Results(second.ID)) != 0 || len(r.Results(first.ID)) != 2 {
		t.Fatal("results kept for a version no longer here, or dropped for one that is")
	}
}

// What each version's checks said, summed up: a failed check fails it,
// every check passed passes it, one still running leaves it out.
func TestChecked(t *testing.T) {
	r, _ := Init(newProject(t), "yi")
	a := mustSnapshot(t, r, "a")
	r.SetResult(a.ID, "tests", "pass", "")
	r.SetResult(a.ID, "build", "fail", "")
	write(t, r.Root, "notes.txt", "b")
	b := mustSnapshot(t, r, "b")
	r.SetResult(b.ID, "tests", "pass", "")
	r.SetResult(b.ID, "build", "pass", "")
	write(t, r.Root, "notes.txt", "c")
	c := mustSnapshot(t, r, "c")
	r.SetResult(c.ID, "tests", "pass", "")
	r.SetResult(c.ID, "build", "running", "")
	write(t, r.Root, "notes.txt", "d")
	mustSnapshot(t, r, "d")
	got := r.Checked()
	if len(got) != 2 || got[a.ID] != "fail" || got[b.ID] != "pass" {
		t.Fatalf("checked: %v", got)
	}
}
