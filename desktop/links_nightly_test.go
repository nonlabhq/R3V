//go:build nightly

package desktop

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/teams"
)

// Links handed to R3V lead to this computer's team and project: a hosted
// team by R3V Cloud's id, a project by the team that has it here; what this
// computer hasn't is said, not guessed.
func TestLinks(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	cloudID := strings.Repeat("1", 32)
	pid := strings.Repeat("2", 32)
	var hosted, storage string
	teams.Update(func(s *teams.Store) error {
		hosted = s.Upsert(remote.Config{URL: "r3v-cloud+https://api.r3v.so/v1/teams/" + cloudID}, "Band").ID
		storage = s.Upsert(remote.Config{URL: "s3+https://storage.example/bucket/r3v"}, "Duo").ID
		s.SetProjectRoot(storage, pid, `C:\Song`)
		return nil
	})
	a := NewApp()
	var emitted []string
	a.emit = func(name string, _ any) { emitted = append(emitted, name) }
	if a.TakeLink() != nil {
		t.Fatal("a link before any was handed over")
	}
	if !a.handLinks([]string{"--background", "r3v://team/" + cloudID, "r3v://project/" + pid + "/version/" + strings.Repeat("3", 64),
		"r3v://team/" + strings.Repeat("9", 32), "r3v://project/" + strings.Repeat("8", 32)}) || len(emitted) != 1 || emitted[0] != "open-link" {
		t.Fatalf("handed over: %v", emitted)
	}
	want := []LinkTarget{
		{Team: hosted},
		{Team: storage, Project: pid, Version: strings.Repeat("3", 64)},
		{Missing: "team"},
		{Project: strings.Repeat("8", 32), Missing: "project"},
	}
	for i, w := range want {
		got := a.TakeLink()
		if got == nil || *got != w {
			t.Errorf("link %d: %+v, want %+v", i, got, w)
		}
	}
	if a.TakeLink() != nil || a.handLinks([]string{"--background"}) {
		t.Error("more links than handed over")
	}
	// Links the page never takes: the newest few are kept.
	var many []string
	for i := range maxPendingLinks + 3 {
		many = append(many, fmt.Sprintf("r3v://project/%032x", i))
	}
	a.handLinks(many)
	for i := 3; i < maxPendingLinks+3; i++ {
		if got := a.TakeLink(); got == nil || got.Project != fmt.Sprintf("%032x", i) {
			t.Fatalf("kept link %d: %+v", i, got)
		}
	}
	if a.TakeLink() != nil {
		t.Error("more than the newest links kept")
	}
}
