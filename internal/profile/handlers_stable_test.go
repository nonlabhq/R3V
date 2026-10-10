//go:build !nightly

package profile

import "testing"

// Stable doesn't take a project's own handlers yet.
func TestProjectHandlersNightlyOnly(t *testing.T) {
	if ProjectHandlers {
		t.Error("project handlers on in Stable")
	}
}
