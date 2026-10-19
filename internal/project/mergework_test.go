package project

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// laneToMerge: a code project on a team, a branch lane that changed
// src/render.ts and added src/fx.ts, the project back on main with
// changes not committed that lane doesn't touch (src/main.ts changed,
// notes.md added, assets/wall.png deleted). It returns the files before
// the merge and after it.
func laneToMerge(t *testing.T, code string) (r *Repo, before, after map[string]string) {
	t.Helper()
	r = codeProject(t, code)
	if _, err := r.NewBranch("lane", "", true); err != nil {
		t.Fatal(err)
	}
	write(t, r.Root, "src/render.ts", "render(fast)\n")
	write(t, r.Root, "src/fx.ts", "bloom()\n")
	if _, _, err := r.Save("faster", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	if _, err := r.SwitchBranch("main", false); err != nil {
		t.Fatal(err)
	}
	write(t, r.Root, "src/main.ts", "export const a = 2\n")
	write(t, r.Root, "notes.md", "todo\n")
	if err := os.Remove(filepath.Join(r.Root, "assets", "wall.png")); err != nil {
		t.Fatal(err)
	}
	before = files(t, r.Root)
	after = maps.Clone(before)
	after["src/render.ts"], after["src/fx.ts"] = "render(fast)\n", "bloom()\n"
	return r, before, after
}

// Merging another branch with changes not committed that it doesn't
// touch: the merge goes ahead and is shared without them, and they stay
// as they were, uncommitted.
func TestMergeKeepsUnrelatedChanges(t *testing.T) {
	_, code := cutTeam(t)
	r, _, after := laneToMerge(t, code)
	head := r.Head()
	res, err := r.MergeBranch("lane", "", Strategy("fail"))
	if err != nil || res.Action != "merged" || !res.KeptWork {
		t.Fatalf("merge: %v %+v", err, res)
	}
	if got := files(t, r.Root); !maps.Equal(got, after) {
		t.Fatalf("files after the merge:\n%v", fileNames(got))
	}
	m, err := r.Load(r.Head())
	if err != nil || len(m.Parents) != 2 || m.Parents[0] != head {
		t.Fatalf("the merge version: %v %+v", err, m)
	}
	fm := m.FileMap()
	if _, ok := fm["notes.md"]; ok {
		t.Error("the merge version has the file added, not committed")
	}
	if _, ok := fm["assets/wall.png"]; !ok {
		t.Error("the merge version lost the file deleted, not committed")
	}
	if _, ok := fm["src/fx.ts"]; !ok {
		t.Error("the merge version lacks what came in")
	}
	if was, _ := r.Load(head); fm["src/main.ts"].Hash != was.FileMap()["src/main.ts"].Hash {
		t.Error("the merge version has the change not committed")
	}
	changes, err := r.Status()
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, c := range changes {
		paths = append(paths, c.Path+" "+c.Status)
	}
	slices.Sort(paths)
	if want := []string{"assets/wall.png deleted", "notes.md added", "src/main.ts modified"}; !slices.Equal(paths, want) {
		t.Fatalf("still uncommitted: %v, want %v", paths, want)
	}
	c, _ := r.Client()
	if heads, _ := c.Branches(r.Config.ProjectID); heads["main"] != r.Head() {
		t.Fatal("the merge isn't shared")
	}
	// Committed later, they make a version of their own after the merge.
	if v, _, err := r.Save("my changes", Strategy("fail")); err != nil || v.Parents[0] != m.ID {
		t.Fatalf("commit after: %v", err)
	}
	if ch, _ := r.Status(); len(ch) != 0 {
		t.Fatalf("left after the commit: %+v", ch)
	}
}

// A change not committed to a file the merge writes or deletes refuses it,
// naming the files; nothing changes. A file moved away from a place the
// merge writes counts, and a change of the rules coming in touches every
// change.
func TestMergeRefusesTouchedChanges(t *testing.T) {
	for _, tc := range []struct {
		name  string
		edit  func(t *testing.T, r *Repo)
		paths []string
	}{
		{"changed", func(t *testing.T, r *Repo) { write(t, r.Root, "src/render.ts", "render(mine)\n") }, []string{"src/render.ts"}},
		{"added where it adds", func(t *testing.T, r *Repo) { write(t, r.Root, "src/fx.ts", "mine\n") }, []string{"src/fx.ts"}},
		{"moved", func(t *testing.T, r *Repo) {
			if err := os.Rename(filepath.Join(r.Root, "src", "render.ts"), filepath.Join(r.Root, "src", "draw.ts")); err != nil {
				t.Fatal(err)
			}
		}, []string{"src/render.ts"}}, // (where it was: what the merge writes)
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, code := cutTeam(t)
			r, _, _ := laneToMerge(t, code)
			tc.edit(t, r)
			before, head := files(t, r.Root), r.Head()
			_, err := r.MergeBranch("lane", "", Strategy("fail"))
			var touched *TouchedChangesError
			if !errors.As(err, &touched) || !errors.Is(err, ErrDirty) {
				t.Fatalf("merge: %v", err)
			}
			for _, p := range tc.paths {
				if !slices.Contains(touched.Paths, p) {
					t.Errorf("%s not named: %v", p, touched.Paths)
				}
			}
			if slices.Contains(touched.Paths, "notes.md") {
				t.Errorf("an unrelated change named: %v", touched.Paths)
			}
			if got := files(t, r.Root); !maps.Equal(got, before) || r.Head() != head || r.UnfinishedSwitch() != "" {
				t.Fatal("the refused merge changed something")
			}
		})
	}
	t.Run("rules", func(t *testing.T) {
		_, code := cutTeam(t)
		r, _, _ := laneToMerge(t, code)
		// lane changes the rules too.
		if _, err := r.SwitchBranch("lane", true); err != nil {
			t.Fatal(err)
		}
		write(t, r.Root, ".r3v.yaml", "presets:\n  ./: none\nrules:\n  - ignore: \"node_modules/\"\n  - ignore: \"target/\"\n  - ignore: \"*.log\"\n")
		if _, _, err := r.Save("rules", Strategy("fail")); err != nil {
			t.Fatal(err)
		}
		if _, err := r.SwitchBranch("main", false); err != nil {
			t.Fatal(err)
		}
		write(t, r.Root, "notes.md", "todo\n")
		var touched *TouchedChangesError
		if _, err := r.MergeBranch("lane", "", Strategy("fail")); !errors.As(err, &touched) || !slices.Contains(touched.Paths, "notes.md") {
			t.Fatalf("merge bringing new rules: %v", err)
		}
	})
}

// The merge killed while it writes the files: the switch is known to have
// stopped, the changes kept don't count as changed since, putting the
// files back gives them as they were (the version too), and merging again
// finishes it with the changes kept.
func TestMergeWithWorkKilled(t *testing.T) {
	if testing.Short() {
		t.Skip("kills merges")
	}
	_, code := cutTeam(t)
	r, _, _ := laneToMerge(t, code)
	steps := writes(func() {
		if _, err := r.MergeBranch("lane", "", Strategy("fail")); err != nil {
			t.Fatal(err)
		}
	})
	if steps < 2 {
		t.Fatalf("the merge wrote %d files", steps)
	}
	for n := 1; n <= steps; n++ {
		what := fmt.Sprintf("killed after %d of %d files", n, steps)
		_, code := cutTeam(t)
		b, before, after := laneToMerge(t, code)
		head := b.Head()
		if !runKilled(t, nil, "merge", "", 0, "R3V_KILL_DIR="+b.Root, "R3V_KILL_TO=lane", fmt.Sprintf("R3V_KILL_WRITES=%d", n)) {
			t.Fatalf("%s: it finished", what)
		}
		r, err := Open(b.Root)
		if err != nil {
			t.Fatal(err)
		}
		eitherOf(t, what, files(t, r.Root), before, after)
		if r.UnfinishedSwitch() == "" {
			t.Fatalf("%s: the merge isn't known to have stopped", what)
		}
		if changed, err := r.ChangedSinceSwitch(); err != nil || len(changed) != 0 {
			t.Fatalf("%s: changed since: %v %v", what, changed, err)
		}
		if _, err := r.RecoverSwitch(); err != nil {
			t.Fatalf("%s: putting the files back: %v", what, err)
		}
		if got := files(t, r.Root); !maps.Equal(got, before) || r.Head() != head {
			t.Fatalf("%s: after putting the files back (the changes as they were):\n%v", what, fileNames(got))
		}
		if res, err := r.MergeBranch("lane", "", Strategy("fail")); err != nil || !res.KeptWork {
			t.Fatalf("%s: merging again: %v %+v", what, err, res)
		}
		if got := files(t, r.Root); !maps.Equal(got, after) {
			t.Fatalf("%s: after merging again:\n%v", what, fileNames(got))
		}
		if ch, _ := r.Status(); len(ch) != 3 {
			t.Fatalf("%s: uncommitted after: %+v", what, ch)
		}
	}
}

// An update whose team's versions and the ones committed here must be
// merged keeps changes not committed the same way: the merge version
// (here, to be shared by the next commit) is without them.
func TestUpdateMergeKeepsUnrelatedChanges(t *testing.T) {
	_, code := cutTeam(t)
	b, _, _ := behindTeammate(t, code)
	if _, err := b.Snapshot("mine, here"); err != nil {
		t.Fatal(err)
	}
	other := string(randomBytes(31, 50<<10))
	write(t, b.Root, "Samples/other.wav", other)
	res, err := b.Update(Strategy("fail"))
	if err != nil || res.Action != "merged" || !res.KeptWork {
		t.Fatalf("update: %v %+v", err, res)
	}
	m, _ := b.Load(b.Head())
	if _, ok := m.FileMap()["Samples/other.wav"]; ok || len(m.Parents) != 2 {
		t.Fatalf("the merge version: %+v", m.Parents)
	}
	if got := fileOf(t, filepath.Join(b.Root, "Samples", "other.wav")); got != other {
		t.Fatal("the change not committed is gone")
	}
	if ch, _ := b.Status(); len(ch) != 1 || ch[0].Path != "Samples/other.wav" {
		t.Fatalf("uncommitted: %+v", ch)
	}
}
