package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A new folder named like one the rules leave out is shown with its rule;
// adding the rule takes it out of the changes.
func TestStateBuiltFolders(t *testing.T) {
	root := localSong(t)
	os.WriteFile(filepath.Join(root, ".r3v.yaml"), []byte("rules:\n  - ignore: \"**/Renders/\"\n"), 0o644)
	os.MkdirAll(filepath.Join(root, "Renders-old"), 0o755)
	os.WriteFile(filepath.Join(root, "Renders-old", "take.wav"), []byte("RIFF"), 0o644)
	a := NewApp()
	st, err := a.State(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Built) != 1 || st.Built[0].Folder != "Renders-old" || st.Built[0].Rule != "Renders-*/" || st.Built[0].Files != 1 {
		t.Fatalf("built: %+v", st.Built)
	}
	if err := a.AddIgnoreRule(root, st.Built[0].Rule); err != nil {
		t.Fatal(err)
	}
	st, err = a.State(root)
	if err != nil {
		t.Fatal(err)
	}
	if st.Built == nil || len(st.Built) != 0 {
		t.Errorf("built after the rule: %+v", st.Built)
	}
	for _, c := range st.Changes {
		if strings.HasPrefix(c.Path, "Renders-old/") {
			t.Errorf("still a change: %s", c.Path)
		}
	}
}
