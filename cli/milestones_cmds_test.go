package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// Milestones from the command line: a version named for the team (by
// default the one you're on), listed, renamed, given a note, removed; tag
// and tags are the same commands.
func TestMilestoneCommands(t *testing.T) {
	a := teamProject(t)
	list := func() []milestoneItem {
		t.Helper()
		exit, rep := run(t, a, "milestones")
		if exit != 0 {
			t.Fatalf("milestones: %+v", rep.Error)
		}
		var got struct{ Milestones []milestoneItem }
		json.Unmarshal(rep.Result, &got)
		return got.Milestones
	}
	if got := list(); len(got) != 0 {
		t.Fatalf("none yet: %+v", got)
	}
	exit, rep := run(t, a, "milestone", "Sent to the label", "-m", "v1 mix")
	if exit != 0 {
		t.Fatalf("milestone: %+v", rep.Error)
	}
	var added struct{ ID, Name, Version string }
	json.Unmarshal(rep.Result, &added)
	if added.Name != "Sent to the label" || added.Version == "" || len(added.ID) != 32 {
		t.Fatalf("added: %+v", added)
	}
	if got := list(); len(got) != 1 || got[0].Name != "Sent to the label" || got[0].Note != "v1 mix" || got[0].Version != added.Version {
		t.Fatalf("listed: %+v", got)
	}
	if exit, rep := run(t, a, "tag", "rename", "sent to the label", "Label v1"); exit != 0 {
		t.Fatalf("rename (by its name, any case): %+v", rep.Error)
	}
	if exit, rep := run(t, a, "milestone", "note", "Label v1", ""); exit != 0 {
		t.Fatalf("note: %+v", rep.Error)
	}
	if got := list(); len(got) != 1 || got[0].Name != "Label v1" || got[0].Note != "" {
		t.Fatalf("after rename and note: %+v", got)
	}
	if exit, _ := run(t, a, "milestone", "remove", "Nope"); exit == 0 {
		t.Error("removed a milestone that isn't there")
	}
	if exit, _ := run(t, a, "milestone", "Next", "no-such-version"); exit == 0 {
		t.Error("named a version that isn't there")
	}
	if exit, rep := run(t, a, "milestone", "remove", added.ID[:8]); exit != 0 {
		t.Fatalf("remove by id: %+v", rep.Error)
	}
	if exit, rep := run(t, a, "tags"); exit != 0 || len(list()) != 0 {
		t.Fatalf("tags after removing: %s %+v", rep.Result, rep.Error)
	}
}

// The words of r3v milestone never name a milestone by accident: milestone
// (alone) and milestone list list them, milestone add names any word.
func TestMilestoneWords(t *testing.T) {
	a := teamProject(t)
	count := func(args ...string) int {
		t.Helper()
		exit, rep := run(t, a, args...)
		if exit != 0 {
			t.Fatalf("%v: %+v", args, rep.Error)
		}
		var got struct{ Milestones []milestoneItem }
		if err := json.Unmarshal(rep.Result, &got); err != nil || got.Milestones == nil {
			t.Fatalf("%v didn't list: %s", args, rep.Result)
		}
		return len(got.Milestones)
	}
	for _, args := range [][]string{{"milestone"}, {"milestone", "list"}, {"tag", "LIST"}, {"milestone", "ls"}} {
		if n := count(args...); n != 0 {
			t.Fatalf("%v: %d milestones", args, n)
		}
	}
	for _, w := range []string{"show", "delete", "rm", "Show"} {
		exit, rep := run(t, a, "milestone", w)
		if exit != exitUsage || rep.Error == nil || rep.Error.Code != "usage" || !strings.Contains(rep.Error.Hint, "milestone add") {
			t.Fatalf("milestone %s: exit %d %+v", w, exit, rep.Error)
		}
	}
	if n := count("milestones"); n != 0 {
		t.Fatalf("a word named a milestone: %d", n)
	}
	if exit, rep := run(t, a, "milestone", "add", "list", "-m", "said so"); exit != 0 {
		t.Fatalf("milestone add list: %+v", rep.Error)
	}
	if exit, rep := run(t, a, "milestone", "add", "Beta"); exit != 0 {
		t.Fatalf("milestone add Beta: %+v", rep.Error)
	}
	if n := count("milestone"); n != 2 {
		t.Fatalf("after add: %d", n)
	}
	if exit, rep := run(t, a, "milestone", "remove", "list"); exit != 0 || count("milestone", "list") != 1 {
		t.Fatalf("remove list: %+v", rep.Error)
	}
}

// Milestones and the file commands are in every build since 0.1.81
// (Stable too): r3v milestone, tag, diff of a project's files, merge-files.
func TestFileCommandsInEveryBuild(t *testing.T) {
	for _, c := range []string{"milestone", "milestones", "tag", "tags", "merge-files"} {
		if _, ok := extraCommands[c]; !ok {
			t.Errorf("r3v %s missing", c)
		}
	}
	if !fileCommands || !jsonCommands["diff"] || !jsonCommands["info"] {
		t.Error("the file commands missing")
	}
}
