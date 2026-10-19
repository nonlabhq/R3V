package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote/cloudtest"
)

// A copy of a project its R3V Cloud team deleted (moved away from another
// copy): a member's commit stays here (project_deleted); the owner's
// brings it back (restored) and shares.
func TestCommitProjectDeletedFromCloudTeam(t *testing.T) {
	f := cloudtest.NewFake(t)
	f.AddMember("kai", "0000000000000000000000000000000b", false)
	a := newFolder(t, map[string]string{".r3v.yaml": "presets:\n  ./: none\n", "notes.txt": "hi"})
	ra, err := project.Init(a, "yi")
	if err != nil {
		t.Fatal(err)
	}
	if err := ra.SetRemote(f.Address); err != nil {
		t.Fatal(err)
	}
	if exit, rep := run(t, a, "commit", "-m", "first"); exit != 0 {
		t.Fatalf("commit: %+v", rep.Error)
	}
	c, err := ra.Client()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteProject(ra.Config.ProjectID); err != nil {
		t.Fatal(err)
	}

	os.WriteFile(filepath.Join(a, "notes.txt"), []byte("hi again"), 0o644)
	t.Setenv("R3V_CLOUD_TOKEN", "kai")
	if exit, rep := run(t, a, "commit", "-m", "second"); exit == 0 || rep.Error.Code != "project_deleted" {
		t.Fatalf("a member's commit: %d %+v", exit, rep.Error)
	}
	t.Setenv("R3V_CLOUD_TOKEN", "token")
	os.WriteFile(filepath.Join(a, "notes.txt"), []byte("hi once more"), 0o644)
	exit, rep := run(t, a, "commit", "-m", "third")
	if exit != 0 {
		t.Fatalf("the owner's commit: %+v", rep.Error)
	}
	var got syncJSON
	if json.Unmarshal(rep.Result, &got) != nil || got.Action != "published" || !got.Restored {
		t.Fatalf("result: %s", rep.Result)
	}
	if f.Deleted(ra.Config.ProjectID) {
		t.Error("still deleted")
	}
}
