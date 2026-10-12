//go:build nightly

package desktop

import (
	"testing"
)

// The main copy, where no workspace set Claude Code up: the app says its
// steps aren't made, and sets it up.
func TestAppSetUpSteps(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	a := NewApp()
	root := teamSong(t, a)
	info, err := a.Steps(root, "")
	if err != nil || !info.Available || info.Hooked {
		t.Fatalf("before: %+v %v", info, err)
	}
	if err := a.SetUpSteps(root); err != nil {
		t.Fatal(err)
	}
	if info, err := a.Steps(root, ""); err != nil || !info.Hooked {
		t.Fatalf("after: %+v %v", info, err)
	}
}
