package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// A folder of documents only: refused as a kind R3V doesn't know, then
// added as the person says ("none": every file as it is), its choice in
// .r3v.yaml.
func TestAddProjectAs(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	fake := s3test.New("band")
	defer fake.Close()
	t.Cleanup(waitTidy)
	a := NewApp()
	team, err := a.CreateStorageTeam(remote.Storage{Endpoint: fake.URL, Bucket: "band", AccessKey: "k", SecretKey: "s"}, "Band")
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "Plans")
	os.MkdirAll(filepath.Join(root, "notes"), 0o755)
	os.WriteFile(filepath.Join(root, "notes", "plan.md"), []byte("# Plan\n"), 0o644)
	if _, err := a.AddProjectToTeam(team.ID, root); !errors.Is(err, project.ErrUnknownKind) {
		t.Fatalf("documents only: %v", err)
	}
	if _, err := a.AddProjectAs(team.ID, root, "nonsense"); err == nil {
		t.Fatal("a kind R3V doesn't know was taken")
	}
	if _, err := a.AddProjectAs(team.ID, root, "none"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, ".r3v.yaml"))
	if !strings.Contains(string(data), "./: none") {
		t.Fatalf(".r3v.yaml: %s", data)
	}
	// The guide for agents: none, then the project's own, then R3V's part
	// at its end (the rest kept).
	if g := a.AgentGuide(root); g != "" {
		t.Fatalf("guide: %q", g)
	}
	const ours = "# Our rules\nBe kind.\n"
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(ours), 0o644)
	if g := a.AgentGuide(root); g != "theirs" {
		t.Fatalf("their guide: %q", g)
	}
	if _, err := a.AddAgentGuide(root); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(root, "AGENTS.md")); !strings.HasPrefix(string(data), ours) || a.AgentGuide(root) != "has" {
		t.Fatalf("AGENTS.md: %s", data)
	}
	st, err := a.State(root)
	if err != nil || len(st.Changes) < 2 {
		t.Fatalf("its files to commit: %v %+v", err, st)
	}
}
