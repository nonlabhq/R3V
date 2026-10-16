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
				{Folder: "Build_Win", Kind: "name", Like: "Build/", Files: 1, Bytes: 10, Rule: "Build_*/"},
				{Folder: "Library.bak", Kind: "name", Like: "Library/", Files: 1, Bytes: 10, Rule: "Library.*/"},
				{Folder: "game/target-dx12", Kind: "name", Like: "target/", Files: 2, Bytes: 20, Rule: "target-*/"},
			},
		},
		"not just a name that begins so":  {added: entries("targets/a", "build/b", "buildings/c", "target-/d")},
		"a folder already in the version": {added: entries("app/target-old/new.txt")},
		"many built files": {
			added: entries(append(many("tools/out/obj", ".obj", 30), many("tools/out/bin", ".pdb", 20)...)...),
			want:  []BuiltFolder{{Folder: "tools/out", Kind: "built", Files: 50, Bytes: 500, Rule: "tools/out/"}},
		},
		"a new sub-project: its build's output, not its sources": {
			added: entries(append(append(many("tools/src", ".cpp", 40), "tools/CMakeLists.txt"), many("tools/out/x64/Release", ".obj", 60)...)...),
			want:  []BuiltFolder{{Folder: "tools/out/x64/Release", Kind: "built", Files: 60, Bytes: 600, Rule: "tools/out/x64/Release/"}},
		},
		"built files spread out: the folder holding them all": {
			added: entries(append(many("tools/a/out", ".obj", 30), many("tools/b/out", ".obj", 30)...)...),
			want:  []BuiltFolder{{Folder: "tools", Kind: "built", Files: 60, Bytes: 600, Rule: "/tools/"}},
		},
		"too few built files": {added: entries(many("tools/out", ".obj", 49)...)},
		"inside a folder already tracked": {
			added: entries(many("src/gen", ".o", 60)...),
			want:  []BuiltFolder{{Folder: "src/gen", Kind: "built", Files: 60, Bytes: 600, Rule: "src/gen/"}},
		},
		"source files don't count": {added: entries(many("src/new", ".rs", 80)...)},
		"named, then built inside the same new folder": {
			// The named folder's files don't count again for the folder above.
			added: entries(append(many("plugin/target-x", ".obj", 60), "plugin/src/lib.rs")...),
			want:  []BuiltFolder{{Folder: "plugin/target-x", Kind: "name", Like: "target/", Files: 60, Bytes: 600, Rule: "target-*/"}},
		},
	} {
		got := looksBuilt(c.added, head, names)
		if len(got) != len(c.want) {
			t.Errorf("%s: %+v, want %+v", name, got, c.want)
			continue
		}
		for i, w := range c.want {
			g := got[i]
			if g.Folder != w.Folder || g.Kind != w.Kind || g.Like != w.Like || g.Files != w.Files || g.Bytes != w.Bytes || g.Rule != w.Rule || g.Reason == "" {
				t.Errorf("%s: %+v, want %+v", name, g, w)
			}
		}
	}
	// Size: whatever the files are.
	big := []FileEntry{{Path: "Renders/take1/a.wav", Size: 150 << 20}, {Path: "Renders/take2/b.wav", Size: 60 << 20}, {Path: "Renders/c.txt", Size: 1}}
	got := looksBuilt(big, head, names)
	if len(got) != 1 || got[0].Folder != "Renders" || got[0].Kind != "size" || got[0].Files != 3 || got[0].Rule != "/Renders/" || got[0].Reason != "holds 210 MB of new files" {
		t.Errorf("size: %+v", got)
	}
	// Most of the bytes in one folder: that one.
	deep := append([]FileEntry{{Path: "Renders/take1/stems/a.wav", Size: 240 << 20}}, big[1:]...)
	if got := looksBuilt(deep, head, names); len(got) != 1 || got[0].Folder != "Renders/take1/stems" || got[0].Kind != "size" || got[0].Files != 1 {
		t.Errorf("size, in one folder: %+v", got)
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

// A folder's rule leaves it out at once; adding it again changes nothing;
// a project without a .r3v.yaml gets one.
func TestAddIgnoreRuleLooksBuilt(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, profile.FileName), []byte("presets:\n  ./: none\n"), 0o644)
	r, err := Init(root, "Yi")
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(root, "tools", "out"), 0o755)
	for i := range 60 {
		os.WriteFile(filepath.Join(root, "tools", "out", fmt.Sprintf("f%d.obj", i)), []byte("o"), 0o644)
	}
	os.WriteFile(filepath.Join(root, "tools", "main.cpp"), []byte("int main() {}"), 0o644)
	_, warned, err := r.StatusChecked()
	if err != nil || len(warned) != 1 || warned[0].Folder != "tools/out" || warned[0].Kind != BuiltByFiles {
		t.Fatalf("warnings %+v %v", warned, err)
	}
	if added, err := r.AddIgnoreRule(warned[0].Rule); err != nil || !added {
		t.Fatalf("added %v, %v", added, err)
	}
	if added, err := r.AddIgnoreRule(warned[0].Rule); err != nil || added {
		t.Errorf("added again %v, %v", added, err)
	}
	changes, warned, _ := r.StatusChecked()
	if len(warned) != 0 {
		t.Errorf("warnings after %+v", warned)
	}
	for _, c := range changes {
		if strings.HasPrefix(c.Path, "tools/out/") {
			t.Errorf("still a change: %s", c.Path)
		}
	}
	if _, err := AddIgnoreRule(root, ""); err == nil {
		t.Error("an empty rule added")
	}

	set := t.TempDir() // (a Live project, no .r3v.yaml yet)
	os.MkdirAll(filepath.Join(set, "Ableton Project Info"), 0o755)
	if added, err := AddIgnoreRule(set, "Backup-*/"); err != nil || !added {
		t.Fatalf("no .r3v.yaml: added %v, %v", added, err)
	}
	if data, _ := os.ReadFile(filepath.Join(set, profile.FileName)); !strings.Contains(string(data), `- ignore: "Backup-*/"`) {
		t.Errorf("written:\n%s", data)
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
