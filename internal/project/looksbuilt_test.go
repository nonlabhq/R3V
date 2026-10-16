package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/profile"
)

func entries(paths ...string) []FileEntry {
	var out []FileEntry
	for _, p := range paths {
		out = append(out, FileEntry{Path: p, Size: 10})
	}
	return out
}

func many(dir, ext string, n int) []string {
	var out []string
	for i := range n {
		out = append(out, fmt.Sprintf("%s/f%03d%s", dir, i, ext))
	}
	return out
}

func TestLooksBuilt(t *testing.T) {
	names := []string{"build", "library", "target"}
	head := map[string]FileEntry{"src/main.rs": {}, "app/target-old/keep.txt": {}, "Assets/a.png": {}}
	for name, c := range map[string]struct {
		added []FileEntry
		want  []BuiltFolder
	}{
		"named like an ignored folder, topmost only": {
			added: entries("game/target-dx12/release/a.txt", "game/target-dx12/release/deps/b.rlib", "Build_Win/x.bin", "Library.bak/c"),
			want: []BuiltFolder{
				{Folder: "Build_Win", Files: 1, Bytes: 10, Rule: "Build_*/"},
				{Folder: "Library.bak", Files: 1, Bytes: 10, Rule: "Library.*/"},
				{Folder: "game/target-dx12", Files: 2, Bytes: 20, Rule: "target-*/"},
			},
		},
		"not just a name that begins so":  {added: entries("targets/a", "build/b", "buildings/c", "target-/d")},
		"a folder already in the version": {added: entries("app/target-old/new.txt")},
		"many built files": {
			added: entries(append(many("tools/out/obj", ".obj", 30), many("tools/out/bin", ".pdb", 20)...)...),
			want:  []BuiltFolder{{Folder: "tools", Files: 50, Bytes: 500, Rule: "/tools/"}},
		},
		"too few built files": {added: entries(many("tools/out", ".obj", 49)...)},
		"inside a folder already tracked": {
			added: entries(many("src/gen", ".o", 60)...),
			want:  []BuiltFolder{{Folder: "src/gen", Files: 60, Bytes: 600, Rule: "src/gen/"}},
		},
		"source files don't count": {added: entries(many("src/new", ".rs", 80)...)},
		"named, then built inside the same new folder": {
			// The named folder's files don't count again for the folder above.
			added: entries(append(many("plugin/target-x", ".obj", 60), "plugin/src/lib.rs")...),
			want:  []BuiltFolder{{Folder: "plugin/target-x", Files: 60, Bytes: 600, Rule: "target-*/"}},
		},
	} {
		got := looksBuilt(c.added, head, names)
		if len(got) != len(c.want) {
			t.Errorf("%s: %+v, want %+v", name, got, c.want)
			continue
		}
		for i, w := range c.want {
			g := got[i]
			if g.Folder != w.Folder || g.Files != w.Files || g.Bytes != w.Bytes || g.Rule != w.Rule || g.Reason == "" {
				t.Errorf("%s: %+v, want %+v", name, g, w)
			}
		}
	}
	// Size: whatever the files are.
	big := []FileEntry{{Path: "Renders/take1/a.wav", Size: 150 << 20}, {Path: "Renders/take2/b.wav", Size: 60 << 20}, {Path: "Renders/c.txt", Size: 1}}
	got := looksBuilt(big, head, names)
	if len(got) != 1 || got[0].Folder != "Renders" || got[0].Files != 3 || got[0].Rule != "/Renders/" || got[0].Reason != "holds 210 MB of new files" {
		t.Errorf("size: %+v", got)
	}
	if got := looksBuilt(big[:1], head, names); len(got) != 0 {
		t.Errorf("150 MB: %+v", got)
	}
}

// What status warns of, from the project's own rules; the commit says the
// same of what it took; a folder once committed isn't new any more.
func TestStatusCheckedWarnsOfNewBuildFolders(t *testing.T) {
	root := t.TempDir()
	write := func(rel, s string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(profile.FileName, "presets:\n  ./: none\nrules:\n  - ignore: \"**/target/\"\n")
	write("src/main.rs", "fn main() {}")
	r, err := Init(root, "Yi")
	if err != nil {
		t.Fatal(err)
	}
	mustSnapshot(t, r, "first")
	write("target/debug/app.exe", "built")      // left out
	write("target-dx12/debug/app.exe", "built") // not
	write("target-dx12/debug/deps/a.rlib", "x") // (one warning for both)
	write("src/lib.rs", "pub fn f() {}")

	ro, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	ro.ReadOnly = true // (as status while another R3V holds the project)
	before := treeState(t, root)
	changes, warned, err := ro.StatusChecked()
	if err != nil {
		t.Fatal(err)
	}
	if after := treeState(t, root); after != before {
		t.Errorf("a read-only status wrote to the project:\n%s\n%s", before, after)
	}
	if len(changes) != 3 {
		t.Errorf("changes %+v", changes)
	}
	if len(warned) != 1 || warned[0].Folder != "target-dx12" || warned[0].Files != 2 || warned[0].Rule != "target-*/" {
		t.Fatalf("warnings %+v", warned)
	}

	m := mustSnapshot(t, r, "with a build")
	if got := r.BuiltFoldersIn(m); len(got) != 1 || got[0].Folder != "target-dx12" {
		t.Errorf("the commit's warnings %+v", got)
	}
	write("target-dx12/debug/b.exe", "more")
	if _, warned, _ := r.StatusChecked(); len(warned) != 0 {
		t.Errorf("a committed folder warned of again: %+v", warned)
	}
}

// treeState: every file under root, with its size and time.
func treeState(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if fi, err := d.Info(); err == nil {
				fmt.Fprintf(&b, "%s %d %d\n", p, fi.Size(), fi.ModTime().UnixNano())
			}
		}
		return nil
	})
	return b.String()
}
