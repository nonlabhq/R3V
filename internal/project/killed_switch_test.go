package project

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// R3V killed while a switch rewrites the project's files (going to another
// version, or getting the team's versions into uncommitted work): every file
// is whole (as it was, or as it was to be), the project knows the switch
// didn't finish, putting the files back gives exactly what was there (the
// work included), nothing of R3V's is left in the folder, and switching
// again works. The files are rewritten after everything is downloaded, so
// this is the step that can leave a project half one version, half another.

// writes counts the files a step puts in place, run here.
func writes(step func()) int {
	n := 0
	fileWritten = func(string) { n++ }
	defer func() { fileWritten = nil }()
	step()
	return n
}

// eitherOf checks each file is whole: as in before or as in after.
func eitherOf(t *testing.T, what string, got, before, after map[string]string) {
	t.Helper()
	for p, c := range got {
		if strings.HasPrefix(filepath.Base(p), ".r3v-") {
			continue // (R3V's own: see leftovers)
		}
		if b, ok := before[p]; ok && c == b {
			continue
		}
		if a, ok := after[p]; ok && c == a {
			continue
		}
		t.Errorf("%s: %s is neither as it was nor as it was to be (%d bytes)", what, p, len(c))
	}
}

// leftovers lists what R3V's own writes left in the project folder.
func leftovers(root string) []string {
	var out []string
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && d.Name() == ".r3v" {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasPrefix(d.Name(), ".r3v-") {
			out = append(out, p)
		}
		return nil
	})
	return out
}

// twoVersions: a project at version a, and version b with a file changed,
// added, removed and moved.
func twoVersions(t *testing.T) (r *Repo, a, b *Manifest, before, after map[string]string) {
	t.Helper()
	r, _ = Init(newProject(t), "yi")
	write(t, r.Root, "notes.txt", "a")
	write(t, r.Root, "Samples/one.wav", string(randomBytes(11, 200<<10)))
	write(t, r.Root, "Samples/two.wav", string(randomBytes(12, 200<<10)))
	write(t, r.Root, "Docs/old.txt", "old")
	a = mustSnapshot(t, r, "a")
	write(t, r.Root, "notes.txt", "b")
	write(t, r.Root, "Samples/one.wav", string(randomBytes(13, 200<<10)))
	write(t, r.Root, "Samples/three.wav", string(randomBytes(14, 200<<10)))
	write(t, r.Root, "Mix/bounce.wav", string(randomBytes(15, 300<<10)))
	os.Remove(filepath.Join(r.Root, "Docs", "old.txt"))
	os.Remove(filepath.Join(r.Root, "Docs"))
	os.MkdirAll(filepath.Join(r.Root, "Samples", "Kept"), 0o755)
	os.Rename(filepath.Join(r.Root, "Samples", "two.wav"), filepath.Join(r.Root, "Samples", "Kept", "two.wav"))
	b = mustSnapshot(t, r, "b")
	// (as a switch leaves them: sets relinked for this computer)
	if _, _, err := r.Checkout(b.ID, false); err != nil {
		t.Fatal(err)
	}
	after = files(t, r.Root)
	if _, _, err := r.Checkout(a.ID, false); err != nil {
		t.Fatal(err)
	}
	before = files(t, r.Root)
	return r, a, b, before, after
}

func TestSwitchKilled(t *testing.T) {
	if testing.Short() {
		t.Skip("kills switches")
	}
	r, _, b, _, _ := twoVersions(t)
	steps := writes(func() {
		if _, _, err := r.Checkout(b.ID, false); err != nil {
			t.Fatal(err)
		}
	})
	if steps < 3 {
		t.Fatalf("the switch wrote %d files", steps)
	}
	for n := 1; n <= steps; n++ {
		what := fmt.Sprintf("killed after %d of %d files", n, steps)
		r, a, b, before, after := twoVersions(t)
		if !runKilled(t, nil, "checkout", "", 0, "R3V_KILL_DIR="+r.Root, "R3V_KILL_TO="+b.ID,
			fmt.Sprintf("R3V_KILL_WRITES=%d", n)) {
			t.Fatalf("%s: it finished", what)
		}
		// (a file cut off mid-copy leaves R3V's temporary file beside it)
		write(t, r.Root, "Samples/.r3v-123456", "half a copy")
		r, err := Open(r.Root)
		if err != nil {
			t.Fatal(err)
		}
		eitherOf(t, what, files(t, r.Root), before, after)
		if got := r.UnfinishedSwitch(); got != b.ID {
			t.Fatalf("%s: unfinished %q, want b", what, got)
		}
		if _, err := r.RecoverSwitch(); err != nil {
			t.Fatalf("%s: putting the files back: %v", what, err)
		}
		if got := files(t, r.Root); !maps.Equal(got, before) || r.Head() != a.ID || r.UnfinishedSwitch() != "" {
			t.Fatalf("%s: after putting the files back:\n%v", what, fileNames(got))
		}
		if left := leftovers(r.Root); len(left) != 0 {
			t.Errorf("%s: left in the folder: %v", what, left)
		}
		assertClean(t, r)
		if _, _, err := r.Checkout(b.ID, false); err != nil {
			t.Fatalf("%s: switching again: %v", what, err)
		}
		if got := files(t, r.Root); !maps.Equal(got, after) {
			t.Fatalf("%s: after switching again:\n%v", what, fileNames(got))
		}
	}
}

// Getting the team's versions into uncommitted work, killed while the
// files are rewritten: putting them back gives the work as it was.
func TestUpdateWithWorkKilled(t *testing.T) {
	if testing.Short() {
		t.Skip("kills updates")
	}
	_, code := cutTeam(t)
	b, _, _ := behindTeammate(t, code)
	steps := writes(func() {
		if _, err := b.Update(Strategy("fail")); err != nil {
			t.Fatal(err)
		}
	})
	if steps < 2 {
		t.Fatalf("the update wrote %d files", steps)
	}
	for n := 1; n <= steps; n++ {
		what := fmt.Sprintf("killed after %d of %d files", n, steps)
		_, code := cutTeam(t)
		b, before, after := behindTeammate(t, code)
		head := b.Head()
		if !runKilled(t, nil, "update", "", 0, "R3V_KILL_DIR="+b.Root, fmt.Sprintf("R3V_KILL_WRITES=%d", n)) {
			t.Fatalf("%s: it finished", what)
		}
		r, err := Open(b.Root)
		if err != nil {
			t.Fatal(err)
		}
		eitherOf(t, what, files(t, r.Root), before, after)
		if r.UnfinishedSwitch() == "" {
			t.Fatalf("%s: the update isn't known to have stopped", what)
		}
		if _, err := r.RecoverSwitch(); err != nil {
			t.Fatalf("%s: putting the files back: %v", what, err)
		}
		if got := files(t, r.Root); !maps.Equal(got, before) || r.Head() != head {
			t.Fatalf("%s: after putting the files back (the work as it was):\n%v", what, fileNames(got))
		}
		if _, err := r.Update(Strategy("fail")); err != nil {
			t.Fatalf("%s: updating again: %v", what, err)
		}
		if got := files(t, r.Root); !maps.Equal(got, after) {
			t.Fatalf("%s: after updating again:\n%v", what, fileNames(got))
		}
	}
}
