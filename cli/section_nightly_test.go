//go:build nightly

package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote/cloudtest"
)

// r3v section: new, add this project, list, rename, remove, delete; names
// taken and sections not there have their codes.
func TestSectionCommands(t *testing.T) {
	a := teamProject(t)
	var got sectionChangeJSON
	exit, rep := run(t, a, "section", "new", "Engine")
	if exit != 0 || json.Unmarshal(rep.Result, &got) != nil || got.Section == nil || got.Section.Name != "Engine" || !got.Changed {
		t.Fatalf("new: %d %+v %s", exit, rep.Error, rep.Result)
	}
	if exit, rep := run(t, a, "section", "new", "engine"); exit == 0 || rep.Error.Code != "section_exists" {
		t.Errorf("a second Engine: %d %+v", exit, rep.Error)
	}
	run(t, a, "section", "new", "Tools")
	exit, rep = run(t, a, "section", "add", "tools")
	if exit != 0 || json.Unmarshal(rep.Result, &got) != nil || !got.Changed || got.ID == "" || got.Section.Name != "Tools" ||
		len(got.Section.Projects) != 1 || !got.Section.Projects[0].Here {
		t.Fatalf("add: %d %+v %s", exit, rep.Error, rep.Result)
	}
	_, rep = run(t, a, "status")
	var st struct{ Team struct{ Section string } }
	if json.Unmarshal(rep.Result, &st) != nil || st.Team.Section != "Tools" {
		t.Errorf("status: %s", rep.Result)
	}
	if exit, rep := run(t, a, "section", "add", "Game"); exit == 0 || rep.Error.Code != "no_section" {
		t.Errorf("add to no section: %d %+v", exit, rep.Error)
	}

	var list struct {
		Sections  []sectionJSON
		Ungrouped []sectionProjectJSON
		Here      string
	}
	_, rep = run(t, a, "section", "list")
	if json.Unmarshal(rep.Result, &list) != nil || len(list.Sections) != 2 || list.Sections[0].Name != "Engine" ||
		len(list.Sections[1].Projects) != 1 || len(list.Ungrouped) != 0 || list.Here != list.Sections[1].ID {
		t.Fatalf("list: %s", rep.Result)
	}
	if exit, rep := run(t, a, "section", "rename", "Tools", "Game"); exit != 0 || json.Unmarshal(rep.Result, &got) != nil ||
		got.Section.Name != "Game" || len(got.Section.Projects) != 1 {
		t.Fatalf("rename: %d %+v %s", exit, rep.Error, rep.Result)
	}
	if exit, rep := run(t, a, "section", "remove"); exit != 0 || json.Unmarshal(rep.Result, &got) != nil || !got.Changed || strings.Contains(string(rep.Result), `"section"`) {
		t.Fatalf("remove: %d %+v %s", exit, rep.Error, rep.Result)
	}
	if _, rep := run(t, a, "section", "remove"); json.Unmarshal(rep.Result, &got) != nil || got.Changed {
		t.Errorf("removed twice: %s", rep.Result)
	}
	if exit, rep := run(t, a, "section", "delete", "Engine"); exit != 0 || json.Unmarshal(rep.Result, &got) != nil || !got.Changed {
		t.Fatalf("delete: %d %+v %s", exit, rep.Error, rep.Result)
	}
	_, rep = run(t, a, "section")
	if json.Unmarshal(rep.Result, &list) != nil || len(list.Sections) != 1 || len(list.Ungrouped) != 1 || list.Here != "" {
		t.Errorf("list after: %s", rep.Result)
	}
	if exit, rep := run(t, a, "section", "rename", "Game"); exit != exitUsage || rep.Error.Code != "usage" {
		t.Errorf("rename without a new name: %d %+v", exit, rep.Error)
	}
}

// On an R3V Cloud team a member is refused (not_admin) and still lists.
func TestSectionCloudAdminsOnly(t *testing.T) {
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
	if exit, rep := run(t, a, "section", "new", "Engine"); exit != 0 {
		t.Fatalf("the owner: %+v", rep.Error)
	}
	t.Setenv("R3V_CLOUD_TOKEN", "kai")
	if exit, rep := run(t, a, "section", "add", "Engine"); exit != exitError || rep.Error.Code != "not_admin" {
		t.Fatalf("a member: %d %+v", exit, rep.Error)
	}
	if exit, rep := run(t, a, "section", "list"); exit != 0 {
		t.Errorf("a member listing: %+v", rep.Error)
	}
}
