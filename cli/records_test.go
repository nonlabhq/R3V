package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/actor"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// A commit by an agent says so; -m twice is a title and details.
func TestCommitActorAndDetails(t *testing.T) {
	a := newFolder(t, map[string]string{"notes.txt": "first"})
	if _, err := project.Init(a, "yi"); err != nil {
		t.Fatal(err)
	}
	t.Setenv(actor.EnvTool, "claude-code")
	t.Setenv(actor.EnvModel, "opus")
	exit, rep := run(t, a, "commit", "-m", "Fix the renderer", "-m", "Lights were too bright.")
	var sv syncJSON
	json.Unmarshal(rep.Result, &sv)
	if exit != 0 || sv.Saved == nil || sv.Saved.Message != "Fix the renderer\n\nLights were too bright." {
		t.Fatalf("commit: %s %+v", rep.Result, rep.Error)
	}
	if a := sv.Saved.Actor; a == nil || a.Kind != "agent" || a.Tool != "claude-code" || a.Model != "opus" {
		t.Fatalf("actor: %s", rep.Result)
	}

	// A person's commit has none.
	t.Setenv(actor.EnvTool, actor.None)
	os.WriteFile(filepath.Join(a, "notes.txt"), []byte("second"), 0o644)
	if exit, rep := run(t, a, "commit", "-m", "by hand"); exit != 0 || strings.Contains(string(rep.Result), `"actor"`) {
		t.Fatalf("person's commit: %s", rep.Result)
	}
	exit, rep = run(t, a, "log")
	var lg struct{ Versions []versionJSON }
	json.Unmarshal(rep.Result, &lg)
	if exit != 0 || len(lg.Versions) != 2 || lg.Versions[0].Actor != nil || lg.Versions[1].Actor == nil ||
		lg.Versions[1].Actor.Tool != "claude-code" {
		t.Fatalf("log: %s", rep.Result)
	}
	var raw struct{ Versions []map[string]any }
	json.Unmarshal(rep.Result, &raw)
	if got, _ := json.Marshal(raw.Versions[1]["actor"]); string(got) != `{"kind":"agent","model":"opus","tool":"claude-code"}` {
		t.Fatalf("actor's JSON: %s", got)
	}

	if exit, rep := run(t, a, "commit", "-m", " ", "-m", ""); exit != 2 || rep.Error.Code != "usage" {
		t.Fatalf("no title: %d %+v", exit, rep.Error)
	}
}

func TestByLine(t *testing.T) {
	m := &project.Manifest{}
	if got := byLine("Mia", m); got != "Mia" {
		t.Fatal(got)
	}
	m.Actor = &project.Actor{Kind: "agent", Tool: "claude-code"}
	if got := byLine("Mia", m); got != "Mia (claude-code)" {
		t.Fatal(got)
	}
}

// `branch new NAME --agent` makes an agent's branch, the name several
// words; without branch records (Stable) it is refused.
func TestBranchNewAgent(t *testing.T) {
	for _, records := range []bool{true, false} {
		was := remote.BranchRecords
		remote.BranchRecords = records
		fake := s3test.New("team")
		code := remote.EncodeConnectionCode(remote.Config{URL: "s3+" + fake.URL + "/team/r3v",
			AccessKey: "key", SecretKey: "secret"})
		a := newFolder(t, map[string]string{"notes.txt": "first"})
		r, err := project.Init(a, "yi")
		if err != nil {
			t.Fatal(err)
		}
		if err := r.SetRemote(code); err != nil {
			t.Fatal(err)
		}
		if exit, rep := run(t, a, "commit", "-m", "first"); exit != 0 {
			t.Fatalf("commit: %+v", rep.Error)
		}
		exit := Run([]string{"branch", "new", "Renderer", "work", "--agent"})
		r, _ = project.Open(a)
		recs, _ := r.BranchRecords()
		switch {
		case records && (exit != 0 || r.BranchName() != "renderer-work" || recs["renderer-work"].Kind != remote.BranchAgent):
			t.Errorf("with records: exit %d, on %q, %+v", exit, r.BranchName(), recs)
		case !records && (exit == 0 || r.BranchName() != "main"):
			t.Errorf("without records: exit %d, on %q", exit, r.BranchName())
		}
		fake.Close()
		remote.BranchRecords = was
	}
}
