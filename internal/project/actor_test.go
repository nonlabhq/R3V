package project

import (
	"os"
	"path/filepath"
	"testing"
)

// The versions a process makes carry its actor; replayed ones keep whose
// they were, a person's included.
func TestVersionsCarryTheActor(t *testing.T) {
	a, b := team(t)
	a.Actor = &Actor{Kind: "agent", Tool: "claude-code", Model: "opus"}
	copyFile(t, filepath.Join(fixtureProject, "Split-A.als"), filepath.Join(a.Root, "Song.als"))
	m, _, err := a.Save("group audio", Strategy("fail"))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := a.Load(m.ID); got.Actor == nil || got.Actor.Tool != "claude-code" || got.Actor.Model != "opus" {
		t.Fatalf("actor: %+v", got.Actor)
	}

	// B: one version by the person, one by an agent, then shared by another.
	os.WriteFile(filepath.Join(b.Root, "Samples", "one.wav"), []byte("RIFF one"), 0o644)
	if _, err := b.Snapshot("one"); err != nil {
		t.Fatal(err)
	}
	b.Actor = &Actor{Kind: "agent", Tool: "codex"}
	os.WriteFile(filepath.Join(b.Root, "Samples", "two.wav"), []byte("RIFF two"), 0o644)
	if _, err := b.Snapshot("two"); err != nil {
		t.Fatal(err)
	}
	b.Actor = &Actor{Kind: "agent", Tool: "other"}
	if res, err := b.Share(Strategy("fail")); err != nil || res.Action != "published" {
		t.Fatalf("share: %v %+v", err, res)
	}
	log, _ := b.Log()
	tools := map[string]string{}
	for _, m := range log {
		tool := "(person)"
		if m.Actor != nil {
			tool = m.Actor.Tool
		}
		tools[m.Message] = tool
	}
	if tools["one"] != "(person)" || tools["two"] != "codex" || tools["group audio"] != "claude-code" {
		t.Fatalf("actors after replay: %v", tools)
	}
}
