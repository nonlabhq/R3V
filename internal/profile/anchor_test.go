package profile

import "testing"

// As profiles.md says: a pattern with a "/" inside starts at the project
// folder, one without matches at any depth.
func TestPatternAnchoring(t *testing.T) {
	for _, c := range []struct {
		pattern, rel string
		want         bool
	}{
		{"target/", "spikes/app/src-tauri/target/debug/x.exe", true},
		{"src-tauri/gen/", "src-tauri/gen/schema.json", true},
		{"src-tauri/gen/", "spikes/app/src-tauri/gen/schema.json", false},
		{"**/src-tauri/gen/", "spikes/app/src-tauri/gen/schema.json", true},
		{"**/src-tauri/gen/", "src-tauri/gen/schema.json", true},
		{"apps/web/dist/", "apps/web/dist/index.js", true},
		{"apps/web/dist/", "other/apps/web/dist/index.js", false},
		{"/Notes.txt", "sub/Notes.txt", false},
	} {
		if got := matchPath(c.pattern, c.rel, false); got != c.want {
			t.Errorf("%s on %s: %v", c.pattern, c.rel, got)
		}
	}
}
