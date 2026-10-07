package project

import (
	"errors"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// A version given a name for the team: renamed, its note changed, taken
// away; the version itself untouched.
func TestMilestones(t *testing.T) {
	withBranchRecords(t)
	fake := s3test.New("team")
	defer fake.Close()
	code := remote.EncodeConnectionCode(remote.Config{URL: "s3+" + fake.URL + "/team/r3v",
		AccessKey: "key", SecretKey: "secret"})
	a, _ := Init(newProject(t), "yi")
	a.SetRemote(code)
	write(t, a.Root, "Notes/lyrics.txt", lyrics)
	if _, _, err := a.Save("mix for the label", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	v := a.Head()
	id, err := a.AddMilestone(v, "  Sent to the label, v1 ", "with the long intro")
	if err != nil {
		t.Fatal(err)
	}
	all, err := a.Milestones()
	if m := all[id]; err != nil || m.Version != v || m.Name != "Sent to the label, v1" || m.Note != "with the long intro" || m.Time.IsZero() {
		t.Fatalf("milestones: %+v %v", all, err)
	}
	if err := a.EditMilestone(id, "Mastered — 最終版", ""); err != nil {
		t.Fatal(err)
	}
	if all, _ := a.Milestones(); all[id].Name != "Mastered — 最終版" || all[id].Note != "" || all[id].Version != v {
		t.Errorf("after editing: %+v", all[id])
	}
	for _, bad := range []struct{ version, name, note string }{
		{v, "", ""}, {v, "a\nb", ""}, {v, "ok", strings.Repeat("x", 1001)},
		{strings.Repeat("0", 64), "not a version of the team", ""},
	} {
		if _, err := a.AddMilestone(bad.version, bad.name, bad.note); err == nil {
			t.Errorf("taken: %q %q", bad.name, bad.version[:8])
		}
	}
	if err := a.RemoveMilestone(id); err != nil {
		t.Fatal(err)
	}
	if all, _ := a.Milestones(); len(all) != 0 {
		t.Errorf("left: %+v", all)
	}
	if !a.HasSnapshot(v) {
		t.Error("the version went with its milestone")
	}

	remote.BranchRecords = false
	if _, err := a.AddMilestone(v, "Stable", ""); !errors.Is(err, ErrNoMilestones) {
		t.Errorf("Stable: %v", err)
	}
	if all, err := a.Milestones(); err != nil || len(all) != 0 {
		t.Errorf("Stable lists: %v %v", all, err)
	}
}
