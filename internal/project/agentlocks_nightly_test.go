//go:build nightly

package project

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/nonlabhq/r3v/presets/code"
)

// Code never locks, even where the project's rules ask for a folder: it
// merges hunk by hunk. (The code preset is the Nightly channel's.)
func TestCodeNeverLocks(t *testing.T) {
	dir := t.TempDir()
	for p, c := range map[string]string{"package.json": "{}", "src/main.ts": "x", "src/logo.png": "png",
		".r3v.yaml": "presets:\n  ./: code\nfile_locks:\n  auto_lock:\n    add: [\"src/\"]\n"} {
		os.MkdirAll(filepath.Dir(filepath.Join(dir, p)), 0o755)
		os.WriteFile(filepath.Join(dir, p), []byte(c), 0o644)
	}
	r, err := Init(dir, "yi")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Profile(); err != nil {
		t.Fatal(err)
	}
	if r.AgentLocked("src/main.ts") || r.AgentLocked("package.json") || !r.AgentLocked("src/logo.png") {
		t.Fatal("code locked, or a texture not")
	}
}
