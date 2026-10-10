package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/profile"
)

// A preset merging .ts as text, as the code preset (Nightly) does.
func init() {
	if err := profile.RegisterPreset([]byte("name: zz-hunks\npriority: -9\nhandlers:\n  - files: [\"*.ts\"]\n    merge: text\n")); err != nil {
		panic(err)
	}
}

const appTS = "src/app.ts"

// tenLines is a file of ten lines, those named replaced.
func tenLines(change map[int]string) string {
	var b strings.Builder
	for i := 1; i <= 10; i++ {
		if s, ok := change[i]; ok {
			b.WriteString(s + "\n")
		} else {
			fmt.Fprintf(&b, "line %d\n", i)
		}
	}
	return b.String()
}

// codeTeam is a code project shared by a and b, src/app.ts in it.
func codeTeam(t *testing.T) (a, b *Repo) {
	t.Helper()
	code, _ := newStorage(t)
	dir := filepath.Join(t.TempDir(), "app")
	write(t, dir, ".r3v.yaml", "presets:\n  ./: zz-hunks\n")
	write(t, dir, appTS, tenLines(nil))
	a, err := Init(dir, "yi")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SetRemote(code); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Save("first", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	b, _, err = Clone(code, a.Config.Name, filepath.Join(t.TempDir(), "b", "app"), "alex")
	if err != nil {
		t.Fatal(err)
	}
	return a, b
}

func readText(t *testing.T, r *Repo, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(r.Root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// sameLines: a commits lines 2 and 8, b changes them differently (and
// line 5, which merges on its own) and commits.
func sameLines(t *testing.T) (a, b *Repo) {
	a, b = codeTeam(t)
	write(t, a.Root, appTS, tenLines(map[int]string{2: "a2", 8: "a8"}))
	if _, _, err := a.Save("a", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	write(t, b.Root, appTS, tenLines(map[int]string{2: "b2", 5: "b5", 8: "b8"}))
	return a, b
}

func TestHunksDifferentLinesMerge(t *testing.T) {
	a, b := codeTeam(t)
	write(t, a.Root, appTS, tenLines(map[int]string{2: "a2"}))
	if _, _, err := a.Save("a", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	write(t, b.Root, appTS, tenLines(map[int]string{8: "b8"}))
	if _, _, err := b.Save("b", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	if got, want := readText(t, b, appTS), tenLines(map[int]string{2: "a2", 8: "b8"}); got != want {
		t.Errorf("merged:\n%s\nwant:\n%s", got, want)
	}
}

func TestHunkConflictsDecidedOneByOne(t *testing.T) {
	_, b := sameLines(t)
	head := b.Head()
	_, _, err := b.Save("b", MergeOptions{})
	var mc *MergeConflictError
	if !errors.As(err, &mc) || len(mc.Conflicts) != 2 {
		t.Fatalf("want two hunk conflicts, got %v", err)
	}
	c := mc.Conflicts[0]
	if c.Key != "text:src/app.ts#1" || c.File != appTS || c.Unit != "line 2" || c.Hunk != 1 || !c.CanKeepBoth ||
		c.Description != "changed on both sides" || c.Base.Text != "line 2\n" || c.Ours.Text != "b2\n" ||
		c.Theirs.Text != "a2\n" || c.Ours.Start != 2 || c.Ours.Count != 1 {
		t.Errorf("first hunk: %+v %+v %+v %+v", c, c.Base, c.Ours, c.Theirs)
	}
	if c := mc.Conflicts[1]; c.Key != "text:src/app.ts#2" || c.Unit != "line 8" || c.Hunk != 2 {
		t.Errorf("second hunk: %+v", c)
	}
	// Nothing changed.
	if b.Head() != head || readText(t, b, appTS) != tenLines(map[int]string{2: "b2", 5: "b5", 8: "b8"}) {
		t.Fatal("a conflict changed the project")
	}

	// One decided: the other is still asked for, by the same key.
	_, _, err = b.Save("b", MergeOptions{Resolutions: map[string]string{"text:src/app.ts#1": "theirs"}})
	if !errors.As(err, &mc) || len(mc.Conflicts) != 1 || mc.Conflicts[0].Key != "text:src/app.ts#2" {
		t.Fatalf("want the second hunk left, got %v", err)
	}
	// The rest by the strategy.
	_, res, err := b.Save("b", MergeOptions{Strategy: "ours", Resolutions: map[string]string{"text:src/app.ts#1": "theirs"}})
	if err != nil || res.Action != "published" {
		t.Fatalf("save: %v %+v", err, res)
	}
	if got, want := readText(t, b, appTS), tenLines(map[int]string{2: "a2", 5: "b5", 8: "b8"}); got != want {
		t.Errorf("merged:\n%s\nwant:\n%s", got, want)
	}
}

func TestHunksBothAndWholeFile(t *testing.T) {
	_, b := sameLines(t)
	if _, _, err := b.Save("b", Strategy("both")); err != nil {
		t.Fatal(err)
	}
	if got, want := readText(t, b, appTS), tenLines(map[int]string{2: "b2\na2", 5: "b5", 8: "b8\na8"}); got != want {
		t.Errorf("both:\n%s\nwant:\n%s", got, want)
	}

	// A decision for the file settles it whole, as before hunks.
	_, b = sameLines(t)
	if _, _, err := b.Save("b", MergeOptions{Resolutions: map[string]string{"file:src/app.ts": "theirs"}}); err != nil {
		t.Fatal(err)
	}
	if got, want := readText(t, b, appTS), tenLines(map[int]string{2: "a2", 8: "a8"}); got != want {
		t.Errorf("whole file:\n%s\nwant:\n%s", got, want)
	}
}

func TestHunksResolvedContent(t *testing.T) {
	_, b := sameLines(t)
	given := "merged by hand\n"
	_, res, err := b.Save("b", MergeOptions{Resolved: map[string][]byte{appTS: []byte(given)}})
	if err != nil || res.Action != "published" {
		t.Fatalf("save: %v %+v", err, res)
	}
	if got := readText(t, b, appTS); got != given {
		t.Errorf("given: %q", got)
	}
}
