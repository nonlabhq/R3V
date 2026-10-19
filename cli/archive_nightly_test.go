//go:build nightly

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote/cloudtest"
)

// r3v project archive / unarchive / list: refused with work not shared
// unless --force, marked in the list and in status, brought back.
func TestProjectArchiveCommands(t *testing.T) {
	a := teamProject(t)
	os.WriteFile(filepath.Join(a, "src", "main.ts"), []byte("changed"), 0o644)
	if exit, rep := run(t, a, "project", "archive"); exit == 0 || rep.Error.Code != "unsaved_changes" {
		t.Fatalf("archive with changes: %d %+v", exit, rep.Error)
	}
	if exit, rep := run(t, a, "commit", "--local", "-m", "only here"); exit != 0 {
		t.Fatalf("commit --local: %+v", rep.Error)
	}
	if exit, rep := run(t, a, "project", "archive"); exit == 0 || rep.Error.Code != "unshared_versions" {
		t.Fatalf("archive with a version not shared: %d %+v", exit, rep.Error)
	}
	exit, rep := run(t, a, "project", "archive", "--force")
	if exit != 0 {
		t.Fatalf("archive --force: %+v", rep.Error)
	}
	var got archiveJSON
	if json.Unmarshal(rep.Result, &got) != nil || !got.Archived || !got.Changed || got.ID == "" {
		t.Fatalf("archive: %s", rep.Result)
	}
	if _, rep := run(t, a, "project", "archive", "--force"); json.Unmarshal(rep.Result, &got) != nil || got.Changed {
		t.Errorf("archived twice: %s", rep.Result)
	}

	_, rep = run(t, a, "project", "list")
	var list struct{ Projects []projectListJSON }
	if json.Unmarshal(rep.Result, &list) != nil || len(list.Projects) != 1 || !list.Projects[0].Archived ||
		!list.Projects[0].Here || list.Projects[0].ArchivedAt == "" {
		t.Fatalf("list: %s", rep.Result)
	}
	_, rep = run(t, a, "status")
	var st struct{ Team struct{ Archived bool } }
	if json.Unmarshal(rep.Result, &st) != nil || !st.Team.Archived {
		t.Errorf("status: %s", rep.Result)
	}

	if exit, rep := run(t, a, "project", "unarchive"); exit != 0 || json.Unmarshal(rep.Result, &got) != nil || got.Archived || !got.Changed {
		t.Fatalf("unarchive: %d %+v %s", exit, rep.Error, rep.Result)
	}
	_, rep = run(t, a, "project", "list")
	if json.Unmarshal(rep.Result, &list) != nil || list.Projects[0].Archived {
		t.Errorf("list after: %s", rep.Result)
	}
	if exit, rep := run(t, a, "project", "unarchive", "--force"); exit != exitUsage || rep.Error.Code != "usage" {
		t.Errorf("unarchive --force: %d %+v", exit, rep.Error)
	}
}

// On an R3V Cloud team a member is refused (not_admin); its owner archives.
func TestProjectArchiveCloudAdminsOnly(t *testing.T) {
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
	t.Setenv("R3V_CLOUD_TOKEN", "kai")
	if exit, rep := run(t, a, "project", "archive"); exit != exitError || rep.Error.Code != "not_admin" {
		t.Fatalf("a member archiving: %d %+v", exit, rep.Error)
	}
	t.Setenv("R3V_CLOUD_TOKEN", "token")
	if exit, rep := run(t, a, "project", "archive"); exit != 0 {
		t.Fatalf("the owner archiving: %+v", rep.Error)
	}
}
