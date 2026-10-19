package cli

import (
	"encoding/json"
	"testing"
)

// r3v branch, branch archived and branch log give --json (and branch's
// changes say what they did).
func TestBranchJSON(t *testing.T) {
	a := teamProject(t)
	exit, rep := run(t, a, "branch", "new", "vfx")
	var made struct {
		Op, Key, Name, Head string
	}
	json.Unmarshal(rep.Result, &made)
	if exit != 0 || made.Op != "new" || made.Key != "vfx" || made.Name != "vfx" || made.Head == "" {
		t.Fatalf("branch new: exit %d %s %+v", exit, rep.Result, rep.Error)
	}
	if exit, rep := run(t, a, "switch", "main"); exit != 0 {
		t.Fatalf("switch: %+v", rep.Error)
	}
	exit, rep = run(t, a, "branch")
	var list struct{ Branches []branchItem }
	json.Unmarshal(rep.Result, &list)
	if exit != 0 || len(list.Branches) != 2 {
		t.Fatalf("branch: exit %d %s", exit, rep.Result)
	}
	byKey := map[string]branchItem{}
	for _, b := range list.Branches {
		byKey[b.Key] = b
	}
	if m := byKey["main"]; !m.Current || m.Name != "main" || m.Head == "" || m.Latest == nil || m.Latest.Message != "first" {
		t.Fatalf("main: %+v", m)
	}
	if v := byKey["vfx"]; v.Current || v.Head != made.Head {
		t.Fatalf("vfx: %+v", v)
	}
	if exit, rep := run(t, a, "branch", "archive", "vfx"); exit != 0 {
		t.Fatalf("archive: %+v", rep.Error)
	}
	exit, rep = run(t, a, "branch", "archived")
	var gone struct{ Archived []archivedItem }
	json.Unmarshal(rep.Result, &gone)
	if exit != 0 || len(gone.Archived) != 1 || gone.Archived[0].Key != "vfx" || gone.Archived[0].Head != made.Head || gone.Archived[0].Time.IsZero() {
		t.Fatalf("archived: exit %d %s", exit, rep.Result)
	}
	exit, rep = run(t, a, "branch", "log", "vfx")
	var log struct{ Moves []branchMoveItem }
	json.Unmarshal(rep.Result, &log)
	if exit != 0 || log.Moves == nil {
		t.Fatalf("branch log: exit %d %s", exit, rep.Result)
	}
	if exit, rep := run(t, a, "branch", "nope"); exit != exitUsage || rep.Error.Code != "usage" {
		t.Fatalf("branch nope: exit %d %+v", exit, rep.Error)
	}
}
