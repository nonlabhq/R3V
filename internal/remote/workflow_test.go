package remote_test

import (
	"slices"
	"testing"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/membucket"
)

// Workflow settings (docs/design/team-settings.md): the team's defaults,
// a project's own over them, kept beside what older R3Vs don't know, and a
// team feature once any is written.

func TestWorkflowValues(t *testing.T) {
	on, off := true, false
	for _, c := range []struct {
		team, project *bool
		want          bool
	}{{nil, nil, true}, {&off, nil, false}, {&off, &on, true}, {nil, &off, false}} {
		if got := remote.Value(c.team, c.project, true); got != c.want {
			t.Errorf("Value(%v, %v) = %v", c.team, c.project, got)
		}
	}
	// File locking: the team's default (with its service on), the project's own over it.
	for _, c := range []struct {
		s    remote.LockSettings
		p    *remote.ProjectLocks
		want bool
	}{
		{remote.LockSettings{}, nil, false},
		{remote.LockSettings{On: true}, nil, true},
		{remote.LockSettings{On: true, Default: &off}, nil, false},
		{remote.LockSettings{On: true, Default: &off}, &remote.ProjectLocks{On: &on}, true},
		{remote.LockSettings{On: true}, &remote.ProjectLocks{On: &off}, false},
		{remote.LockSettings{On: true}, &remote.ProjectLocks{Add: []string{"Maps/"}}, true},
	} {
		if got := remote.ProjectLocksOn(c.s, c.p); got != c.want {
			t.Errorf("ProjectLocksOn(%+v, %+v) = %v", c.s, c.p, got)
		}
	}
}

const (
	p1 = "11111111111111111111111111111111"
	p2 = "22222222222222222222222222222222"
)

func TestProjectWorkflowKeptAndCleared(t *testing.T) {
	b := remote.NewBucketBackend(membucket.New())
	for _, p := range []remote.Project{{ID: p1, Name: "Game"}, {ID: p2, Name: "Trailer"}} {
		if err := b.PutProject(p); err != nil {
			t.Fatal(err)
		}
	}
	off, on := false, true
	w1 := &remote.Workflow{Steps: &off, Locks: &remote.ProjectLocks{On: &on, Add: []string{"Content/Maps/"}}}
	if !remote.FileLocks { // (Stable: no file locks to set)
		if err := remote.SetProjectWorkflow(b, p1, w1); err == nil {
			t.Fatal("file locks set in a build without them")
		}
		w1.Locks = nil
	}
	if err := remote.SetProjectWorkflow(b, p1, w1); err != nil {
		t.Fatal(err)
	}
	if err := remote.SetProjectWorkflow(b, p2, &remote.Workflow{Prompts: &off}); err != nil {
		t.Fatal(err)
	}
	info, _ := b.Info()
	if !slices.Contains(info.Features, remote.FeatureWorkflow) {
		t.Errorf("features %v: no workflow", info.Features)
	}
	// A project turning locking on: the service keeps locks, the other projects stay off.
	if s := remote.LocksOf(info); remote.FileLocks && (!s.On || remote.LocksDefault(s) || !slices.Contains(info.Features, remote.FeatureLocks)) {
		t.Errorf("team locks %+v, features %v", s, info.Features)
	}
	ps, _ := b.Projects()
	got := map[string]*remote.Workflow{}
	for _, p := range ps {
		got[p.ID] = p.Workflow
	}
	if w := got[p1]; w == nil || w.Steps == nil || *w.Steps || (remote.FileLocks && (w.Locks == nil || w.Locks.Add[0] != "Content/Maps/")) || got[p2].Prompts == nil {
		t.Fatalf("kept %+v / %+v", got[p1], got[p2])
	}

	// Back to the team's for agents: p2 has nothing left, p1 its locks.
	n, err := remote.ClearProjectWorkflow(b, "agents")
	if err != nil || n != 2 {
		t.Fatalf("cleared %d, %v", n, err)
	}
	ps, _ = b.Projects()
	for _, p := range ps {
		switch {
		case p.ID == p2 && p.Workflow != nil:
			t.Errorf("p2 still %+v", p.Workflow)
		case p.ID == p1 && remote.FileLocks && (p.Workflow == nil || p.Workflow.Locks == nil || p.Workflow.Steps != nil):
			t.Errorf("p1 %+v", p.Workflow)
		}
	}
}

func TestTeamWorkflowDefaults(t *testing.T) {
	b := remote.NewBucketBackend(membucket.New())
	off := false
	if err := remote.SetTeamWorkflow(b, remote.Workflow{Prompts: &off}); err != nil {
		t.Fatal(err)
	}
	info, _ := b.Info()
	if info.Workflow == nil || info.Workflow.Prompts == nil || *info.Workflow.Prompts || !slices.Contains(info.Features, remote.FeatureWorkflow) {
		t.Fatalf("%+v", info)
	}
	if len(remote.Missing(info)) != 0 {
		t.Errorf("this build doesn't know %v", remote.Missing(info))
	}
}
