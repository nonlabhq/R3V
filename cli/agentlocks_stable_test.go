//go:build !nightly

package cli

import (
	"testing"

	"github.com/nonlabhq/r3v/internal/project"
)

// Stable has no agent locks: no commands, nothing checked at commit, and
// the hook (a project set up by Nightly) allows everything.
func TestAgentLocksNotInStable(t *testing.T) {
	if project.AgentLocks {
		t.Fatal("agent locks on in Stable")
	}
	dir, _ := agentFolder(t)
	for _, c := range []string{"lock", "unlock", "locks", "agent"} {
		if exit, rep := run(t, dir, c); exit != 2 || rep.Error.Code != "usage" {
			t.Fatalf("%s in Stable: %+v", c, rep.Error)
		}
	}
	if _, raw := hook(t, hookCall("Edit", dir+"/tex/rock.png", dir)); raw != "" {
		t.Fatalf("hook: %s", raw)
	}
	if code := Run([]string{"hook", "pre-tool"}); code != 0 {
		t.Fatalf("hook exit %d", code)
	}
}
