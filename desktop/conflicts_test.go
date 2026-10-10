package desktop

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nonlabhq/r3v/internal/manifest"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

func TestConflictsName(t *testing.T) {
	got := toConflicts([]project.ConflictItem{
		{Key: "Song.als#track:14", File: "Song.als", Unit: `AudioTrack "Bass"`, Description: "modified on both sides", CanKeepBoth: true},
		{Key: "Song.als#track:9", File: "Song.als", Unit: `MidiTrack "Keys"`, Description: "deleted in ours, modified in theirs", CanKeepBoth: true},
		{Key: "Song.als#order", File: "Song.als", Unit: "track order", Description: "reordered on both sides"},
		{Key: "file:Samples/kick.wav", File: "Samples/kick.wav", Unit: "Samples/kick.wav", Description: "changed by you, deleted by others"},
	})
	want := []struct{ kind, track, name, ours, theirs string }{
		{"track", "14", "Bass", "changed", "changed"},
		{"track", "9", "Keys", "deleted", "changed"},
		{"set", "", "track order", "changed", "changed"},
		{"file", "", "kick.wav", "changed", "deleted"},
	}
	for i, w := range want {
		c := got[i]
		if c.Kind != w.kind || c.Track != w.track || c.Name != w.name || c.Ours != w.ours || c.Theirs != w.theirs {
			t.Errorf("%s: %+v, want %+v", c.Key, c, w)
		}
	}
}

func TestCombinedOf(t *testing.T) {
	got := combinedOf([]string{
		"took theirs: Samples/snare.wav",
		"deleted (theirs): Samples/old.wav",
		"Notes.txt: changed on both sides -> merged",
		`Song.als: AudioTrack "Choir": added from theirs`,
		`Song.als: MidiTrack "Lead": took theirs`,
		`Song.als: AudioTrack "Pads": kept ours`,
		`Song.als: ReturnTrack "Verb": removed (deleted in theirs)`,
		"Song.als: track order: took theirs",
		"Samples/a.wav: moved by you to Samples/b.wav; the other side's changes follow it",
		"took theirs: Samples/snare.wav",
	})
	want := []Combined{
		{File: "Samples/snare.wav", What: "changed"},
		{File: "Samples/old.wav", What: "removed"},
		{File: "Notes.txt", What: "changed"},
		{File: "Song.als", Name: "Choir", What: "added"},
		{File: "Song.als", Name: "Lead", What: "changed"},
		{File: "Song.als", Name: "Verb", What: "removed"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("combined:\n%+v\nwant\n%+v", got, want)
	}
}

// Merging a branch that changed the same file: the conflict comes with
// both sides (who, which version) and what came in on its own.
func TestMergeConflictSides(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	fake := s3test.New("one")
	defer fake.Close()
	t.Cleanup(waitTidy)
	a := NewApp()
	one, _ := a.CreateStorageTeam(remote.Storage{Endpoint: fake.URL, Bucket: "one", AccessKey: "k", SecretKey: "s"}, "One")
	root := newSong(t)
	if _, err := a.AddProjectToTeam(one.ID, root); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Save(root, "first", true, nil, true, nil); err != nil {
		t.Fatal(err)
	}
	key, err := a.CreateBranch(root, "idea", "", true)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "Samples", "kick.wav"), []byte("RIFF-kick-idea"), 0o644)
	os.WriteFile(filepath.Join(root, "Samples", "snare.wav"), []byte("RIFF-snare"), 0o644)
	if _, err := a.Save(root, "on the idea", true, nil, true, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SwitchBranch(root, "main", true); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "Samples", "kick.wav"), []byte("RIFF-kick-main"), 0o644)
	if _, err := a.Save(root, "on main", true, nil, true, nil); err != nil {
		t.Fatal(err)
	}

	res, err := a.MergeBranch(root, key, "", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != "conflicts" || len(res.Conflicts) != 1 || res.Conflicts[0].Kind != "file" || !res.Conflicts[0].CanKeepBoth {
		t.Fatalf("result: %+v", res)
	}
	if res.Ours == nil || res.Ours.Message != "on main" || res.Theirs == nil || res.Theirs.Message != "on the idea" ||
		res.Theirs.Author == "" || res.Theirs.Short == "" {
		t.Fatalf("sides: ours %+v theirs %+v", res.Ours, res.Theirs)
	}
	if want := []Combined{{File: "Samples/snare.wav", What: "changed"}}; !reflect.DeepEqual(res.Combined, want) {
		t.Fatalf("combined: %+v", res.Combined)
	}

	// Decided: merged as before.
	res, err = a.MergeBranch(root, key, "", map[string]string{res.Conflicts[0].Key: "both"}, true)
	if err != nil || res.Action == "conflicts" {
		t.Fatalf("merge: %+v %v", res, err)
	}
}

// A text file's hunk is a decision of its own, with its lines on each side.
func TestConflictsHunk(t *testing.T) {
	got := toConflicts([]project.ConflictItem{
		{Key: "text:src/app.ts#2", File: "src/app.ts", Unit: "lines 4–5", Description: "changed on both sides", CanKeepBoth: true,
			Hunk: 2, Base: &project.HunkSide{Start: 4, Count: 1, Text: "a\n"}, Ours: &project.HunkSide{Start: 4, Count: 2, Text: "b\nc\n"},
			Theirs: &project.HunkSide{Start: 4, Count: 1, Text: "d\n"}},
		{Key: "text:src/app.ts#3", File: "src/app.ts", Unit: "after line 9", Description: "deleted by you, changed by others", CanKeepBoth: true,
			Hunk: 3, Base: &project.HunkSide{Start: 9, Count: 1, Text: "x\n"}, Ours: &project.HunkSide{Start: 10}, Theirs: &project.HunkSide{Start: 9, Count: 1, Text: "y\n"}},
	})
	c := got[0]
	if c.Kind != "hunk" || c.Hunk != 2 || c.Name != "app.ts" || c.Ours != "changed" || c.Theirs != "changed" || c.Text == nil ||
		c.Text.Ours != (HunkSide{Start: 4, Count: 2, Text: "b\nc\n"}) || c.Text.Base.Text != "a\n" || c.Text.Theirs.Text != "d\n" {
		t.Errorf("hunk: %+v %+v", c, c.Text)
	}
	if d := got[1]; d.Kind != "hunk" || d.Ours != "deleted" || d.Theirs != "changed" || d.Text.Ours.Count != 0 {
		t.Errorf("deleted hunk: %+v", d)
	}
}

// A version says the agent that made it; a branch, that it's an agent's.
func TestVersionActorAndAgentBranch(t *testing.T) {
	m := &project.Manifest{ID: "0123456789abcdef", Message: "Fix the renderer\n\nLights were too bright."}
	if v := toVersion(m, nil); v.Actor != nil {
		t.Errorf("a person's: %+v", v.Actor)
	}
	m.Actor = &manifest.Actor{Kind: manifest.ActorAgent, Tool: "claude-code", Model: "opus", Session: "s1"}
	if v := toVersion(m, nil); v.Actor == nil || *v.Actor != (Actor{Tool: "claude-code", Model: "opus"}) {
		t.Errorf("an agent's: %+v", v.Actor)
	}
	b := toBranch(project.BranchInfo{Name: "b-1", Current: true, Latest: m},
		remote.BranchRecord{Name: "renderer", Kind: remote.BranchAgent, Owner: "m1"})
	if b.Kind != "agent" || b.Owner != "m1" || b.Label != "renderer" || b.Latest == nil || b.Latest.Actor == nil {
		t.Errorf("agent branch: %+v", b)
	}
}
