package profile

import (
	"slices"
	"testing"
)

func TestIgnoredFolderNames(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".gitignore", "/target\n*.log\n!keep/\nnode_modules/\n")
	write(t, root, FileName, "gitignore: true\nrules:\n  - ignore: \"**/Build/\"\n  - ignore: \"*.tmp\"\n"+
		"  - ignore: \"cache-*/\"\n  - ignore: \"x/\"\n  - track: \"Library/\"\n  - ignore: \"notes.txt\"\n")
	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	got := p.IgnoredFolderNames()
	// Folders only (from rules), literal, long enough; .gitignore lines may
	// name a folder without the slash.
	if want := []string{"build", "node_modules", "target"}; !slices.Equal(got, want) {
		t.Errorf("names %v, want %v", got, want)
	}
}
