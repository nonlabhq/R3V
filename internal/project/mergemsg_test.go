package project

import (
	"fmt"
	"strings"
	"testing"
)

// chain makes versions one after the other (all in the same second, as
// an agent's quick commits are), the first on top of parent.
func chain(parent string, titles ...string) []*Manifest {
	var out []*Manifest
	for i, title := range titles {
		m := &Manifest{ID: fmt.Sprintf("v%02d", i), Time: "2026-10-16T10:00:00Z", Message: title + "\n\nsome details"}
		if parent != "" {
			m.Parents = []string{parent}
		}
		parent = m.ID
		out = append(out, m)
	}
	return out
}

func TestMergeSummary(t *testing.T) {
	vs := chain("base", "Add bloom pass", "WIP shadows", "Tune shadow cascades", "draft: new sky",
		"Add bloom pass", "Do not merge: debug view", "Wipe old caches", "not for merge")
	// newest first, as a preview lists them: the summary still goes oldest first
	for i, j := 0, len(vs)-1; i < j; i, j = i+1, j-1 {
		vs[i], vs[j] = vs[j], vs[i]
	}
	got := mergeSummary("lane/vfx", "Merge branch lane/vfx", vs)
	want := "Merge lane/vfx: 8 versions\n- Add bloom pass\n- Tune shadow cascades\n- Wipe old caches"
	if got != want {
		t.Fatalf("summary:\n%s\nwant:\n%s", got, want)
	}

	// Nothing to list (only unfinished ones, or merges): the plain message.
	merge := &Manifest{ID: "m", Parents: []string{"a", "b"}, Message: "Merge branch main"}
	if got := mergeSummary("lane/vfx", "Merge branch lane/vfx", append(chain("", "WIP"), merge)); got != "Merge branch lane/vfx" {
		t.Fatalf("no titles: %q", got)
	}
	if got := mergeSummary("lane/vfx", "plain", nil); got != "plain" {
		t.Fatalf("nothing: %q", got)
	}

	// At most ten titles, then how many more.
	var titles []string
	for i := range 14 {
		titles = append(titles, fmt.Sprintf("Step %d", i+1))
	}
	got = mergeSummary("lane/vfx", "", chain("", titles...))
	lines := strings.Split(got, "\n")
	if len(lines) != 12 || lines[0] != "Merge lane/vfx: 14 versions" || lines[1] != "- Step 1" ||
		lines[10] != "- Step 10" || lines[11] != "… and 4 more" {
		t.Fatalf("capped:\n%s", got)
	}
	if got := mergeSummary("lane/vfx", "", chain("", "One")); got != "Merge lane/vfx: 1 version\n- One" {
		t.Fatalf("one: %q", got)
	}
}

func TestLooksWIP(t *testing.T) {
	for title, want := range map[string]bool{
		"WIP": true, "wip: bloom": true, "[Draft] sky": true, "Not for merge": true, "DO NOT MERGE yet": true,
		"Wipe caches": false, "Drafting table model": false, "Add bloom": false, "Fix\nWIP in details": false,
	} {
		if LooksWIP(title) != want {
			t.Errorf("LooksWIP(%q) = %v", title, !want)
		}
	}
}

// Merging a branch without a description writes the summary; the preview
// offers the same text; one given is used as it is.
func TestMergeBranchMessage(t *testing.T) {
	a, b := team(t)
	if err := a.CreateBranch("vfx"); err != nil {
		t.Fatal(err)
	}
	for i, title := range []string{"Add bloom pass", "WIP glow", "Tune shadow cascades"} {
		write(t, a.Root, "vfx.txt", fmt.Sprint(i))
		if _, _, err := a.Save(title, Strategy("fail")); err != nil {
			t.Fatal(err)
		}
	}
	write(t, b.Root, "main.txt", "on main")
	if _, _, err := b.Save("main moves on", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	want := "Merge vfx: 3 versions\n- Add bloom pass\n- Tune shadow cascades"
	p, err := b.PreviewMerge("vfx")
	if err != nil || p.Message != want {
		t.Fatalf("preview: %v %q", err, p.Message)
	}
	if _, err := b.MergeBranch("vfx", "", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	if m, _ := b.Load(b.Head()); m.Message != want {
		t.Fatalf("merge message = %q", m.Message)
	}

	// Given: exactly that.
	write(t, a.Root, "vfx.txt", "more")
	if _, _, err := a.Save("Add lens flare", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	if _, err := b.MergeBranch("vfx", "Bring in the flare", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	if m, _ := b.Load(b.Head()); m.Message != "Bring in the flare" {
		t.Fatalf("given message = %q", m.Message)
	}
}
