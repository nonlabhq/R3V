package project

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// Downloads cut off (the connection lost, R3V closed), uploads resumed, and
// what a stopped step leaves behind: see crash_test.go for shares cut off.

// cutTeam is storage of its own and its connection code.
func cutTeam(t *testing.T) (*s3test.Server, string) {
	t.Helper()
	fake := s3test.New("team")
	t.Cleanup(fake.Close)
	return fake, remote.EncodeConnectionCode(remote.Config{URL: "s3+" + fake.URL + "/team/r3v",
		AccessKey: "key", SecretKey: "secret"})
}

// behindTeammate: A shared a first version, B cloned it and has work of
// its own (uncommitted), then A shared a second version. It returns B and
// B's files as they are and as they are after the update.
func behindTeammate(t *testing.T, code string) (b *Repo, before, after map[string]string) {
	t.Helper()
	a, _, _ := sharedProject(t, code, "Song")
	b, _, err := Clone(code, a.Config.Name, filepath.Join(t.TempDir(), "B", "Song Project"), "alex")
	if err != nil {
		t.Fatal(err)
	}
	write(t, b.Root, "Samples/mine.wav", string(randomBytes(9, 200<<10)))
	before = files(t, b.Root)
	if _, _, err := a.Save("second", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	after = files(t, a.Root)
	after["Samples/mine.wav"] = before["Samples/mine.wav"]
	return b, before, after
}

// An update cut off at any download — half-way through a file, then every
// request failing — leaves the files as they were (your own work
// included), the project on the version it was on, and updating again
// finishes it. Every download is tried, one cut at a time.
func TestUpdateCutAtEveryRead(t *testing.T) {
	if testing.Short() {
		t.Skip("cuts an update at every read")
	}
	fake, code := cutTeam(t)
	b, _, after := behindTeammate(t, code)
	start := fake.Reads
	if _, err := b.Update(Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(files(t, b.Root), after) {
		t.Fatal("an uncut update doesn't give the new version")
	}
	steps := fake.Reads - start
	if steps < 3 {
		t.Fatalf("an update made only %d reads", steps)
	}
	for cut := 0; cut < steps; cut++ {
		fake, code := cutTeam(t)
		b, before, after := behindTeammate(t, code)
		head := b.Head()
		fake.CutReadsAfter = fake.Reads + cut
		_, err := b.Update(Strategy("fail"))
		fake.CutReadsAfter = 0
		got := files(t, b.Root)
		if err == nil {
			if !maps.Equal(got, after) {
				t.Fatalf("cut after %d of %d reads: said updated, but the files are:\n%v", cut, steps, fileNames(got))
			}
		} else if !maps.Equal(got, before) || b.Head() != head || b.UnfinishedSwitch() != "" {
			t.Fatalf("cut after %d of %d reads (%v): the files or the version changed:\n%v", cut, steps, err, fileNames(got))
		}
		if _, err := b.Update(Strategy("fail")); err != nil {
			t.Fatalf("cut after %d of %d reads: updating again: %v", cut, steps, err)
		}
		if got := files(t, b.Root); !maps.Equal(got, after) {
			t.Fatalf("cut after %d of %d reads: after updating again:\n%v", cut, steps, fileNames(got))
		}
	}
}

// A clone cut off at any download leaves a project with no version yet (not
// half a version's files) that can't be committed by mistake, and cloning
// again into the folder finishes it.
func TestCloneCutAtEveryRead(t *testing.T) {
	if testing.Short() {
		t.Skip("cuts a clone at every read")
	}
	fake, code := cutTeam(t)
	a, _, _ := sharedProject(t, code, "Song")
	start := fake.Reads
	want := cloneFiles(t, code, a.Config.Name)
	steps := fake.Reads - start
	for cut := 0; cut < steps; cut++ {
		dir := filepath.Join(t.TempDir(), "B", "Song Project")
		fake.CutReadsAfter = fake.Reads + cut
		b, _, err := Clone(code, a.Config.Name, dir, "alex")
		fake.CutReadsAfter = 0
		if err == nil {
			continue // (the last reads may be ones it can do without)
		}
		if b != nil {
			if got := files(t, b.Root); b.Head() != "" || len(got) > 1 { // (the rules' file)
				t.Fatalf("cut after %d of %d reads: head %q, files %v", cut, steps, b.Head(), fileNames(got))
			}
			if _, _, err := b.Save("oops", Strategy("fail")); !errors.Is(err, ErrNotDownloaded) {
				t.Fatalf("cut after %d of %d reads: a commit before the download finished: %v", cut, steps, err)
			}
		}
		c, _, err := Clone(code, a.Config.Name, dir, "alex")
		if err != nil {
			t.Fatalf("cut after %d of %d reads: cloning again: %v", cut, steps, err)
		}
		if got := files(t, c.Root); !maps.Equal(got, want) {
			t.Fatalf("cut after %d of %d reads: after cloning again:\n%v", cut, steps, fileNames(got))
		}
	}
}

// Cloning into a folder that isn't an unfinished download of that project
// leaves it alone.
func TestCloneLeavesOtherFolders(t *testing.T) {
	_, code := cutTeam(t)
	a, _, _ := sharedProject(t, code, "Song")
	other, _, _ := sharedProject(t, code, "Other")
	for _, dir := range []string{a.Root, other.Root} {
		before := files(t, dir)
		if _, _, err := Clone(code, a.Config.Name, dir, "alex"); err == nil || !strings.Contains(err.Error(), "not empty") {
			t.Errorf("%s: %v", filepath.Base(dir), err)
		}
		if !maps.Equal(files(t, dir), before) {
			t.Errorf("%s: its files changed", filepath.Base(dir))
		}
	}
}

// A big file going up early, cut off (the connection lost, or R3V closed
// and its copy left behind): the copies left are cleaned up when R3V starts,
// it goes up again sending only the pieces that didn't, and the commit then
// sends next to nothing.
func TestPreuploadResumes(t *testing.T) {
	defer func(m int64, s time.Duration) { PreuploadMin, PreuploadStable = m, s }(PreuploadMin, PreuploadStable)
	PreuploadMin, PreuploadStable = 1000, 0
	fake, code := cutTeam(t)
	r, _ := Init(newProject(t), "yi")
	if err := r.SetRemote(code); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.Save("v1", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	big := randomBytes(5, 20<<20) // in pieces
	write(t, r.Root, "Video/take.mov", string(big))
	cands, _ := r.PreuploadCandidates(time.Now())
	if len(cands) != 1 {
		t.Fatalf("candidates: %+v", cands)
	}
	c, _ := r.Client()
	fake.CutAfter = fake.Writes + 6
	first := fake.PutBytes
	if err := r.Preupload(c, cands[0], nil, nil); err == nil {
		t.Fatal("not cut off")
	}
	fake.CutAfter = 0
	first = fake.PutBytes - first

	// R3V closed mid-way leaves its copy; starting again cleans it up.
	dir := filepath.Join(r.Dir, preuploadDir)
	write(t, dir, "tmp-123", "half a copy")
	write(t, dir, cands[0].Hash, string(big))
	r.CleanPreuploads()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("the copies left are still there")
	}

	if again, _ := r.PreuploadCandidates(time.Now()); len(again) != 1 {
		t.Fatalf("not to go up again: %+v", again)
	}
	before := fake.PutBytes
	if err := r.Preupload(c, cands[0], nil, nil); err != nil {
		t.Fatal(err)
	}
	r.NotePreuploaded(cands[0].Hash)
	if sent := fake.PutBytes - before; first == 0 || sent+first > int64(len(big))+64<<10 {
		t.Errorf("sent %d KB before the cut and %d KB after, for %d KB", first>>10, sent>>10, len(big)>>10)
	}
	before = fake.PutBytes
	if _, _, err := r.Save("the take", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	if sent := fake.PutBytes - before; sent > 256<<10 {
		t.Errorf("the commit sent %d KB", sent>>10)
	}
}

// What a step that stopped mid-way left in objects/tmp (R3V closed while
// copying a file into the history, or putting pieces together) goes at the
// next cleanup; what a step under way has there stays.
func TestCleanupRemovesLeftovers(t *testing.T) {
	r, _ := Init(newProject(t), "yi")
	tmp := filepath.Join(r.Dir, "objects", "tmp")
	old := time.Now().Add(-2 * leftoverAge)
	write(t, tmp, "put-123", strings.Repeat("x", 1000))
	write(t, tmp, "pieces-456/a", strings.Repeat("y", 500))
	for _, p := range []string{"put-123", "pieces-456/a", "pieces-456"} {
		os.Chtimes(filepath.Join(tmp, p), old, old)
	}
	write(t, tmp, "put-789", "under way")
	freed, err := r.GC()
	if err != nil {
		t.Fatal(err)
	}
	left, _ := os.ReadDir(tmp)
	if len(left) != 1 || left[0].Name() != "put-789" || freed < 1500 {
		t.Errorf("left %v, freed %d", left, freed)
	}
}
