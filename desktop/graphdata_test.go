package desktop

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nonlabhq/r3v/internal/manifest"
	"github.com/nonlabhq/r3v/internal/project"
)

// The history says what the graph's segments use: how many files each
// version changed, its checks' result, the agent's session it was made
// in. Read while another program holds the project, the counts aren't
// kept; read free, they are.
func TestStateGraphData(t *testing.T) {
	root := localSong(t)
	r, _ := project.Open(root)
	r.Actor = &manifest.Actor{Kind: manifest.ActorAgent, Tool: "claude-code", Session: "s1"}
	snap := func(name, content, msg string) string {
		os.WriteFile(filepath.Join(root, name), []byte(content), 0o644)
		m, err := r.Snapshot(msg)
		if err != nil {
			t.Fatal(err)
		}
		return m.ID
	}
	first := snap("a.txt", "one", "first")
	second := snap("b.txt", "two", "second")
	r.Actor = &manifest.Actor{Kind: manifest.ActorAgent, Tool: "claude-code", Session: "s2"}
	os.WriteFile(filepath.Join(root, "c.txt"), []byte("three"), 0o644)
	third := snap("a.txt", "one, changed", "third")
	r.SetResult(second, "tests", "fail", "")
	r.SetResult(third, "tests", "pass", "")

	other, _ := project.Open(root)
	release, err := other.Lock(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	a := NewApp()
	st, err := a.State(root)
	release()
	if err != nil || !st.Busy {
		t.Fatalf("held: %v", err)
	}
	kept := filepath.Join(root, ".r3v", "changed.json")
	if _, err := os.Stat(kept); !os.IsNotExist(err) {
		t.Fatalf("counts kept while another program held the project: %v", err)
	}
	st, err = a.State(root)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]Version{}
	for _, v := range st.History {
		by[v.ID] = v
	}
	want := map[string]Version{second: {Files: 1, Check: "fail", Step: "s1"}, third: {Files: 2, Check: "pass", Step: "s2"}}
	for id, w := range want {
		if v := by[id]; v.Files != w.Files || v.Check != w.Check || v.Step != w.Step {
			t.Errorf("%s: %d %q %q, want %d %q %q", v.Message, v.Files, v.Check, v.Step, w.Files, w.Check, w.Step)
		}
	}
	if v := by[first]; v.Files < 1 || v.Check != "" || v.Step != "s1" {
		t.Errorf("first: %+v", v)
	}
	if _, err := os.Stat(kept); err != nil {
		t.Fatalf("counts not kept: %v", err)
	}
}
