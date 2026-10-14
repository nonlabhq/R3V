package handlers

import "testing"

// OpenIn runs the checks the project names, in order: the first that has
// something open says it; names without a check, or none open, say nothing.
func TestOpenIn(t *testing.T) {
	RegisterRunning("test-closed", func(string) string { return "" })
	RegisterRunning("test-open", func(root string) string { return root + "/Main.scene" })
	if c, w := OpenIn("P", []string{"nothing-registered", "test-closed", "test-open"}); c != "test-open" || w != "P/Main.scene" {
		t.Errorf("open: %q %q", c, w)
	}
	if c, w := OpenIn("P", []string{"test-closed"}); c != "" || w != "" {
		t.Errorf("none open: %q %q", c, w)
	}
}
