package desktop

import (
	"testing"

	"golang.org/x/sys/windows/registry"
)

// With its own settings folder (tests, the demo) the choice stays there:
// this user's sign-in entry is neither read nor written.
func TestAutostartFollowsTheSettingsFolder(t *testing.T) {
	real := func() (string, bool) {
		k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
		if err != nil {
			return "", false
		}
		defer k.Close()
		v, _, err := k.GetStringValue(runValue())
		return v, err == nil
	}
	before, had := real()

	a := &App{}
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	if a.Autostart() {
		t.Error("a fresh settings folder starts at sign-in")
	}
	if err := a.SetAutostart(true); err != nil {
		t.Fatal(err)
	}
	if !a.Autostart() {
		t.Error("turned on, not kept")
	}
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	if a.Autostart() {
		t.Error("another settings folder sees it")
	}
	if after, has := real(); after != before || has != had {
		t.Error("the user's own sign-in entry changed")
	}
}

// A run with its own settings folder is its own app, not a second launch
// of the user's.
func TestInstanceFollowsTheSettingsFolder(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", "")
	own := instanceID()
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	one := instanceID()
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	if one == own || instanceID() == own || instanceID() == one {
		t.Errorf("instances %q, %q, %q", own, one, instanceID())
	}
}
