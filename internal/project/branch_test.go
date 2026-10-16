package project

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestBranchWorkflow(t *testing.T) {
	a, b := team(t)

	// Yi experiments on a personal branch; main is untouched.
	if err := a.CreateBranch("yi-ideas"); err != nil {
		t.Fatal(err)
	}
	if err := a.CreateBranch("yi-ideas"); err == nil {
		t.Error("creating an existing branch should fail")
	}
	copyFile(t, filepath.Join(fixtureProject, "Split-A.als"), filepath.Join(a.Root, "Song.als"))
	if _, res, err := a.Save("group audio", Strategy("fail")); err != nil || res.Action != "published" {
		t.Fatalf("save on branch: %v %+v", err, res)
	}
	if up, err := b.Update(Strategy("fail")); err != nil || up.Action != "up-to-date" {
		t.Fatalf("main should not have moved: %v %+v", err, up)
	}

	// Alex keeps working on main.
	copyFile(t, filepath.Join(fixtureProject, "Split-B.als"), filepath.Join(b.Root, "Song.als"))
	if _, _, err := b.Save("drums", Strategy("fail")); err != nil {
		t.Fatal(err)
	}

	branches, err := b.Branches()
	if err != nil || len(branches) != 2 || branches[0].Name != "main" || !branches[0].Current ||
		branches[1].Latest == nil || branches[1].Latest.Message != "group audio" {
		t.Fatalf("branches: %v %+v", err, branches)
	}

	// Alex previews Yi's branch: what comes in and what will conflict.
	p, err := b.PreviewMerge("yi-ideas")
	if err != nil {
		t.Fatal(err)
	}
	if p.Action != "merge" || len(p.Versions) != 1 || p.Versions[0].Message != "group audio" {
		t.Fatalf("preview: %+v", p)
	}
	if len(p.Changes) != 1 || p.Changes[0].SetDiff == nil ||
		!strings.Contains(p.Changes[0].SetDiff.Render(), `GroupTrack "Audios"`) {
		t.Fatalf("preview changes: %+v", p.Changes)
	}
	if len(p.Conflicts) != 1 || !strings.Contains(p.Conflicts[0].String(), "Bounce + Reverb") {
		t.Fatalf("preview conflicts: %v", p.Conflicts)
	}
	assertClean(t, b) // preview changes nothing

	res, err := b.MergeBranch("yi-ideas", "", Strategy("both"))
	if err != nil || res.Action != "merged" {
		t.Fatalf("merge branch: %v %+v", err, res)
	}
	if setTracks(t, b)["Audios"].Elem == nil {
		t.Error("merge did not bring the group")
	}
	m, _ := b.Load(b.Head())
	if m.Message != "Merge yi-ideas: 1 version\n- group audio" {
		t.Errorf("merge message = %q", m.Message)
	}

	// Yi goes back to main and gets everything.
	sw, err := a.SwitchBranch("main", false)
	if err != nil || sw.To != b.Head() || a.BranchName() != "main" {
		t.Fatalf("switch: %v %+v", err, sw)
	}
	if setTracks(t, a)["Drum"].Elem == nil {
		t.Error("switch did not bring main's drum track")
	}
	assertClean(t, a)
}

func TestSwitchRefusesUnsharedVersions(t *testing.T) {
	a, _ := team(t)
	a.CreateBranch("draft")
	copyFile(t, filepath.Join(fixtureProject, "Split-A.als"), filepath.Join(a.Root, "Song.als"))
	if _, err := a.Snapshot("local only"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SwitchBranch("main", false); !errors.Is(err, ErrUnshared) {
		t.Fatalf("expected ErrUnshared, got %v", err)
	}
}

func TestPreviewUpdate(t *testing.T) {
	a, b := team(t)
	copyFile(t, filepath.Join(fixtureProject, "Split-A.als"), filepath.Join(a.Root, "Song.als"))
	a.Save("group audio", Strategy("fail"))
	p, err := b.PreviewUpdate()
	if err != nil || p.Action != "fast-forward" || len(p.Changes) != 1 || len(p.Conflicts) != 0 {
		t.Fatalf("%v %+v", err, p)
	}
}

// Any version of another branch can be merged, not only its latest.
func TestMergeVersionFromTheMiddleOfABranch(t *testing.T) {
	a, b := team(t)
	a.CreateBranch("yi-ideas")
	copyFile(t, filepath.Join(fixtureProject, "Split-A.als"), filepath.Join(a.Root, "Song.als"))
	i1, _, err := a.Save("group audio", Strategy("fail"))
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(a.Root, "Samples", "later.wav"), []byte("RIFF-later"), 0o644)
	if _, _, err := a.Save("later idea", Strategy("fail")); err != nil {
		t.Fatal(err)
	}

	copyFile(t, filepath.Join(fixtureProject, "Split-B.als"), filepath.Join(b.Root, "Song.als"))
	before, _, _ := b.Save("drums", Strategy("fail"))
	b.Branches() // fetches the other branch's versions, as the app's history does

	p, err := b.PreviewVersion(i1.ID)
	if err != nil || p.Action != "merge" || len(p.Versions) != 1 || p.Versions[0].ID != i1.ID {
		t.Fatalf("preview: %v %+v", err, p)
	}
	if want := "Merge version " + i1.ID[:10] + ": 1 version\n- group audio"; p.Message != want {
		t.Fatalf("suggested message: %q", p.Message)
	}
	if msg, err := b.MergeMessage(i1.ID); err != nil || !strings.HasPrefix(msg, "Merge version ") {
		t.Fatalf("default merge message: %q %v", msg, err)
	}
	res, err := b.MergeVersion(i1.ID[:10], "Bring in the first idea", Strategy("both"))
	if err != nil || res.Action != "merged" {
		t.Fatalf("merge version: %v %+v", err, res)
	}
	m, _ := b.Load(b.Head())
	if len(m.Parents) != 2 || m.Parents[0] != before.ID || m.Parents[1] != i1.ID ||
		m.Message != "Bring in the first idea" {
		t.Fatalf("merge version: %+v", m)
	}
	// What the merge brought into this branch: the set, compared with "drums".
	changes, err := b.VersionChanges(m.ID)
	if err != nil || len(changes) == 0 || changes[0].Path != "Song.als" || changes[0].SetDiff == nil {
		t.Fatalf("merge changes: %v %+v", err, changes)
	}
	if setTracks(t, b)["Audios"].Elem == nil {
		t.Error("the merged version's group is missing")
	}
	if _, err := os.Stat(filepath.Join(b.Root, "Samples", "later.wav")); err == nil {
		t.Error("a later version of the branch came in too")
	}
	if heads, _ := b.Branches(); len(heads) != 2 {
		t.Fatalf("branches: %+v", heads)
	}
	assertClean(t, b)
}

// A branch made without switching to it: the project stays where it was.
func TestNewBranchStaying(t *testing.T) {
	a, _ := team(t)
	key, err := a.NewBranch("later", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if a.BranchName() != "main" {
		t.Fatalf("on %q, want main", a.BranchName())
	}
	bs, err := a.Branches()
	if err != nil || !slices.ContainsFunc(bs, func(b BranchInfo) bool { return b.Name == key }) {
		t.Fatalf("branches: %v %+v", err, bs)
	}
}

// A version keeps the branch it was made on; merging a branch into another
// always makes a version there, even when it could fast-forward.
func TestVersionsKeepTheirBranch(t *testing.T) {
	a, _ := team(t)
	if m, _ := a.Load(a.Head()); m.Branch != "main" {
		t.Fatalf("first version on %q", m.Branch)
	}
	if err := a.CreateBranch("idea"); err != nil {
		t.Fatal(err)
	}
	write(t, a.Root, "idea.txt", "idea")
	if _, _, err := a.Save("on idea", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	idea := a.Head()
	if m, _ := a.Load(idea); m.Branch != "idea" {
		t.Fatalf("version on idea says %q", m.Branch)
	}
	if _, err := a.SwitchBranch("main", false); err != nil {
		t.Fatal(err)
	}
	res, err := a.MergeBranch("idea", "", Strategy("fail"))
	if err != nil {
		t.Fatal(err)
	}
	m, _ := a.Load(a.Head())
	if res.Action != "merged" || a.Head() == idea || len(m.Parents) != 2 || m.Branch != "main" {
		t.Fatalf("merge: %+v, version %+v", res, m)
	}
}

// Merging a lane main hasn't moved past: the preview says merge (what
// happens), with the lane head's files; the merge version has them, and
// the results recorded for the lane head show on it, carried over. Own
// results win; a merge that changed the files gets none.
func TestMergeBehindKeepsLaneResults(t *testing.T) {
	a, _ := team(t)
	if err := a.CreateBranch("vfx"); err != nil {
		t.Fatal(err)
	}
	write(t, a.Root, "fx.txt", "fx")
	if _, _, err := a.Save("fx", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	lane := a.Head()
	if _, _, err := a.SetResult("", "tests", "pass", "all passed"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SwitchBranch("main", false); err != nil {
		t.Fatal(err)
	}
	main := a.Head()
	p, err := a.PreviewMerge("vfx")
	if err != nil || p.Action != "merge" || p.SameFilesAs != lane || p.Target != lane || p.Base != main ||
		len(p.Versions) != 1 || len(p.Conflicts) != 0 {
		t.Fatalf("preview: %v %+v", err, p)
	}
	if v, err := a.PreviewVersion(lane); err != nil || v.Action != "merge" || v.SameFilesAs != lane {
		t.Fatalf("preview version: %v %+v", err, v)
	}
	res, err := a.MergeBranch("vfx", "", Strategy("fail"))
	if err != nil || res.Action != "merged" || res.SameFilesAs != lane {
		t.Fatalf("merge: %v %+v", err, res)
	}
	merged := a.Head()
	if m, _ := a.Load(merged); len(m.Parents) != 2 || m.Parents[0] != main || m.Parents[1] != lane {
		t.Fatalf("merge version: %+v", m)
	}
	got := a.Results(merged)
	if got["tests"].State != "pass" || got["tests"].From != lane {
		t.Fatalf("carried results: %+v", got)
	}
	if a.Results(lane)["tests"].From != "" {
		t.Fatal("a version's own results marked carried")
	}
	if c := a.Checked(merged); c[merged] != "pass" {
		t.Fatalf("checked: %v", c)
	}
	if c := a.Checked(); c[merged] != "" {
		t.Fatalf("checked without the version asked: %v", c)
	}
	// Own results win, and recording one doesn't copy the carried ones.
	if _, _, err := a.SetResult(merged, "build", "fail", ""); err != nil {
		t.Fatal(err)
	}
	if own := a.Results(merged); len(own) != 1 || own["build"].State != "fail" || own["build"].From != "" {
		t.Fatalf("own results: %+v", own)
	}
	if c := a.Checked(merged); c[merged] != "fail" {
		t.Fatalf("checked with own: %v", c)
	}

	// Main and the lane both move on: the merge changes the files.
	write(t, a.Root, "mix.txt", "mix")
	if _, _, err := a.Save("mix", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SwitchBranch("vfx", false); err != nil {
		t.Fatal(err)
	}
	write(t, a.Root, "fx2.txt", "more fx")
	if _, _, err := a.Save("more fx", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	a.SetResult("", "tests", "pass", "")
	if _, err := a.SwitchBranch("main", false); err != nil {
		t.Fatal(err)
	}
	p, err = a.PreviewMerge("vfx")
	if err != nil || p.Action != "merge" || p.SameFilesAs != "" {
		t.Fatalf("preview of a merge both changed: %v %+v", err, p)
	}
	res, err = a.MergeBranch("vfx", "", Strategy("fail"))
	if err != nil || res.Action != "merged" || res.SameFilesAs != "" {
		t.Fatalf("merge both changed: %v %+v", err, res)
	}
	if got := a.Results(a.Head()); len(got) != 0 {
		t.Fatalf("results carried to a merge that changed the files: %+v", got)
	}
}

// Updating the branch you are on still fast-forwards, and says so.
func TestPreviewUpdateStillFastForwards(t *testing.T) {
	a, b := team(t)
	write(t, a.Root, "notes.txt", "new")
	a.Save("notes", Strategy("fail"))
	p, err := b.PreviewUpdate()
	if err != nil || p.Action != "fast-forward" || p.SameFilesAs != "" {
		t.Fatalf("%v %+v", err, p)
	}
	if p, err := b.PreviewMerge("main"); err != nil || p.Action != "fast-forward" {
		t.Fatalf("merging the branch you are on: %v %+v", err, p)
	}
}

// Each copy tells the team the work it has: changes, versions not shared,
// changes parked; written when it changes, and at once on another branch.
func TestWorkspaceRecordsWork(t *testing.T) {
	parking(t, true)
	a, b := team(t)
	write(t, a.Root, "notes.txt", "mine")
	a.Snapshot("not shared")
	write(t, a.Root, "notes.txt", "more")
	was := noteEvery
	noteEvery = 0 // (the share just now wrote it)
	a.NoteWork(1)
	noteEvery = was
	defer func() { noteEvery = was }()
	ws, err := b.Workspaces()
	if err != nil || len(ws) != 1 || ws[0].Branch != "main" || ws[0].Changes != 1 || ws[0].Unshared != 1 {
		t.Fatalf("workspaces: %v %+v", err, ws)
	}
	a.NoteWork(2) // (soon after, on the same branch: not yet)
	if ws, _ := b.Workspaces(); ws[0].Changes != 1 {
		t.Fatalf("written again at once: %+v", ws)
	}
	if err := a.CreateBranch("idea"); err != nil {
		t.Fatal(err)
	}
	a.NoteWork(2)
	if ws, _ := b.Workspaces(); ws[0].Branch != "idea" || ws[0].Changes != 2 {
		t.Fatalf("on another branch: %+v", ws)
	}
}
