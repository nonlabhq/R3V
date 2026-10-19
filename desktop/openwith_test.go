package desktop

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/teams"
)

// fakeLookup is a computer with files (and registry values) as given.
func fakeLookup(files ...string) lookup {
	has := map[string]bool{}
	for _, f := range files {
		has[strings.ToLower(f)] = true
	}
	env := map[string]string{"LOCALAPPDATA": `C:\Users\u\AppData\Local`, "ProgramFiles": `C:\Program Files`, "SystemRoot": `C:\Windows`}
	return lookup{
		exists: func(p string) bool { return has[strings.ToLower(p)] },
		getenv: func(k string) string { return env[k] },
		glob: func(pattern string) []string {
			var out []string
			for _, f := range files {
				if ok, _ := filepath.Match(strings.ToLower(pattern), strings.ToLower(f)); ok {
					out = append(out, f)
				}
			}
			slices.Sort(out)
			return out
		},
		lookPath: func(string) string { return "" },
	}
}

func ids(ps []Program) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.ID+"="+p.Path)
	}
	return out
}

func TestDetectFromKnownPaths(t *testing.T) {
	if filepath.Separator != '\\' {
		t.Skip("Windows paths")
	}
	l := fakeLookup(
		`C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`,
		`C:\Users\u\AppData\Local\Programs\Microsoft VS Code\Code.exe`,
		`C:\Users\u\AppData\Local\Programs\cursor\Cursor.exe`,
		`C:\Users\u\AppData\Local\JetBrains\Toolbox\apps\Rider\ch-0\241.1\bin\rider64.exe`,
		`C:\Users\u\AppData\Local\JetBrains\Toolbox\apps\Rider\ch-0\243.2\bin\rider64.exe`,
	)
	got := ids(detect(l))
	want := []string{"explorer=", `terminal=C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`,
		`vscode=C:\Users\u\AppData\Local\Programs\Microsoft VS Code\Code.exe`,
		`cursor=C:\Users\u\AppData\Local\Programs\cursor\Cursor.exe`,
		`rider=C:\Users\u\AppData\Local\JetBrains\Toolbox\apps\Rider\ch-0\243.2\bin\rider64.exe`}
	if !slices.Equal(got, want) {
		t.Fatalf("detected %q, want %q", got, want)
	}
}

func TestDetectFromRegistryAndVswhere(t *testing.T) {
	if filepath.Separator != '\\' {
		t.Skip("Windows paths")
	}
	l := fakeLookup(`C:\Users\u\AppData\Local\Microsoft\WindowsApps\wt.exe`, `D:\Tools\VSCode\Code.exe`,
		`C:\Program Files\Microsoft Visual Studio\2022\Community\Common7\IDE\devenv.exe`,
		`D:\JB\Rider\bin\rider64.exe`, `E:\Cursor\Cursor.exe`)
	l.appPath = func(exe string) string {
		if exe == "Code.exe" {
			return `D:\Tools\VSCode\Code.exe`
		}
		return ""
	}
	l.vswhere = func() string { return `C:\Program Files\Microsoft Visual Studio\2022\Community\Common7\IDE\devenv.exe` }
	l.uninstall = func() []installed {
		return []installed{
			{name: "JetBrains Rider 2024.3", location: `D:\JB\Rider`},
			{name: "Cursor (User)", icon: `"E:\Cursor\Cursor.exe",0`},
			{name: "Something Else", location: `D:\JB\Rider`},
		}
	}
	got := ids(detect(l))
	want := []string{"explorer=", `terminal=C:\Users\u\AppData\Local\Microsoft\WindowsApps\wt.exe`,
		`vscode=D:\Tools\VSCode\Code.exe`, `cursor=E:\Cursor\Cursor.exe`,
		`visualstudio=C:\Program Files\Microsoft Visual Studio\2022\Community\Common7\IDE\devenv.exe`,
		`rider=D:\JB\Rider\bin\rider64.exe`}
	if !slices.Equal(got, want) {
		t.Fatalf("detected %q, want %q", got, want)
	}
}

func TestDetectNothingInstalled(t *testing.T) {
	got := ids(detect(fakeLookup()))
	if !slices.Equal(got, []string{"explorer="}) {
		t.Fatalf("detected %q", got)
	}
}

func TestLaunchFor(t *testing.T) {
	root := t.TempDir()
	vs := Program{ID: "visualstudio", Path: `C:\VS\devenv.exe`}
	cases := []struct {
		p    Program
		want launch
	}{
		{Program{ID: "explorer"}, launch{Dir: root}},
		{Program{ID: "terminal", Path: `C:\x\wt.exe`}, launch{Exe: `C:\x\wt.exe`, Args: []string{"-d", root}, Dir: root}},
		{Program{ID: "terminal", Path: `C:\x\powershell.exe`}, launch{Exe: `C:\x\powershell.exe`, Args: []string{"-NoExit"}, Dir: root, Console: true}},
		{Program{ID: "vscode", Path: `C:\x\Code.exe`}, launch{Exe: `C:\x\Code.exe`, Args: []string{root}, Dir: root}},
		{vs, launch{Exe: vs.Path, Args: []string{root}, Dir: root}}, // no solution: the folder
		{Program{ID: `C:\x\My Editor.exe`, Path: `C:\x\My Editor.exe`, Chosen: true}, launch{Exe: `C:\x\My Editor.exe`, Args: []string{root}, Dir: root}},
	}
	for _, c := range cases {
		if got := launchFor(c.p, root); !launchEqual(got, c.want) {
			t.Errorf("%s: %+v, want %+v", c.p.ID, got, c.want)
		}
	}
	// One solution in the root: Visual Studio and Rider open it; with two, the folder.
	os.WriteFile(filepath.Join(root, "Game.sln"), nil, 0o644)
	sln := filepath.Join(root, "Game.sln")
	for _, p := range []Program{vs, {ID: "rider", Path: `C:\x\rider64.exe`}} {
		if got := launchFor(p, root); !slices.Equal(got.Args, []string{sln}) {
			t.Errorf("%s with a solution: %q", p.ID, got.Args)
		}
	}
	if got := launchFor(Program{ID: "cursor", Path: `C:\x\Cursor.exe`}, root); !slices.Equal(got.Args, []string{root}) {
		t.Errorf("cursor opens the folder: %q", got.Args)
	}
	os.WriteFile(filepath.Join(root, "Tools.sln"), nil, 0o644)
	if got := launchFor(vs, root); !slices.Equal(got.Args, []string{root}) {
		t.Errorf("two solutions: %q", got.Args)
	}
}

func launchEqual(a, b launch) bool {
	return a.Exe == b.Exe && slices.Equal(a.Args, b.Args) && a.Dir == b.Dir && a.Console == b.Console
}

// fakePrograms: this computer detects only Explorer and a fake VS Code, and
// nothing is started (what would be is recorded).
func fakePrograms(t *testing.T) *[]launch {
	t.Helper()
	detected.Lock()
	detected.list = []Program{{ID: "explorer", Name: "Explorer"}, {ID: "vscode", Name: "VS Code", Path: `C:\x\Code.exe`}}
	detected.Unlock()
	var started []launch
	was := startProgram
	startProgram = func(l launch) error { started = append(started, l); return nil }
	t.Cleanup(func() {
		detected.Lock()
		detected.list = nil
		detected.Unlock()
		startProgram = was
	})
	return &started
}

func chosenIDs(ps []Program) []string {
	var out []string
	for _, p := range ps {
		if p.Chosen {
			out = append(out, p.ID)
		}
	}
	return out
}

// anotherSong is a second project R3V lists, beside localSong's.
func anotherSong(t *testing.T) string {
	t.Helper()
	root := newSong(t)
	if _, err := project.Init(root, "yi"); err != nil {
		t.Fatal(err)
	}
	store, err := teams.Load()
	if err != nil {
		t.Fatal(err)
	}
	store.AddLocal(root)
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestChooseProgramKeepsItForTheProject(t *testing.T) {
	root := localSong(t)
	fakePrograms(t)
	dir := t.TempDir()
	exe := filepath.Join(dir, "Editor.exe")
	os.WriteFile(exe, nil, 0o755)
	a := &App{pickFile: func(string) (string, error) { return exe, nil }}
	if p, err := a.ChooseProgram(root, "Choose"); err != nil || p == nil || p.ID != exe || p.Name != "Editor" || !p.Chosen {
		t.Fatalf("chose %+v, %v", p, err)
	}
	a.ChooseProgram(root, "Choose") // twice: kept once
	got := a.OpenPrograms(root)
	if !slices.Equal(chosenIDs(got), []string{exe}) || got[0].ID != "explorer" {
		t.Fatalf("programs %+v", got)
	}
	// on this computer, for this project only
	if !strings.Contains(read(t, openWithFile()), "Editor.exe") {
		t.Fatal("not kept in the app's settings")
	}
	other := anotherSong(t)
	if got := chosenIDs(a.OpenPrograms(other)); len(got) != 0 {
		t.Fatalf("other project: %q", got)
	}

	a.pickFile = func(string) (string, error) { return "", nil }
	if p, err := a.ChooseProgram(root, "Choose"); err != nil || p != nil {
		t.Fatalf("cancelled: %+v, %v", p, err)
	}
	a.pickFile = func(string) (string, error) { return filepath.Join(dir, "a.txt"), nil }
	if _, err := a.ChooseProgram(root, "Choose"); err == nil {
		t.Fatal("a .txt taken for a program")
	}
	a.pickFile = func(string) (string, error) { return exe, nil }
	if _, err := a.ChooseProgram(filepath.Join(dir, "Unknown"), "Choose"); err == nil {
		t.Fatal("chosen for a folder R3V doesn't know")
	}
}

func TestOpenFolderWithOnlyKnownPrograms(t *testing.T) {
	root := localSong(t)
	started := fakePrograms(t)
	dir := t.TempDir()
	exe := filepath.Join(dir, "Editor.exe")
	os.WriteFile(exe, nil, 0o755)
	a := &App{pickFile: func(string) (string, error) { return exe, nil }}

	// an .exe that is there but was never chosen for the project: refused
	if err := a.OpenFolderWith(root, exe); err == nil {
		t.Fatal("ran a program never chosen")
	}
	if err := a.OpenFolderWith(root, "cursor"); err == nil {
		t.Fatal("ran a program not detected")
	}
	if len(*started) != 0 || a.OpenedWith(root) != nil {
		t.Fatalf("started %+v, last %+v", *started, a.OpenedWith(root))
	}

	if err := a.OpenFolderWith(root, "vscode"); err != nil {
		t.Fatal(err)
	}
	if l := a.OpenedWith(root); l == nil || l.ID != "vscode" || l.Name != "VS Code" {
		t.Fatalf("last: %+v", l)
	}
	a.ChooseProgram(root, "Choose")
	if err := a.OpenFolderWith(root, exe); err != nil {
		t.Fatal(err)
	}
	if len(*started) != 2 || (*started)[1].Exe != exe || !slices.Equal((*started)[1].Args, []string{root}) {
		t.Fatalf("started %+v", *started)
	}
	if l := a.OpenedWith(root); l == nil || l.ID != exe || l.Name != "Editor" {
		t.Fatalf("last: %+v", l)
	}
	if err := a.OpenFolderWith(anotherSong(t), exe); err == nil {
		t.Fatal("ran another project's program")
	}

	// gone: forgotten, the last picked with it
	os.Remove(exe)
	if err := a.OpenFolderWith(root, exe); err == nil {
		t.Fatal("ran a program gone")
	}
	if got := chosenIDs(a.OpenPrograms(root)); len(got) != 0 || a.OpenedWith(root) != nil {
		t.Fatalf("chosen %q, last %+v", got, a.OpenedWith(root))
	}
	if err := a.OpenFolderWith(filepath.Join(dir, "Unknown"), "explorer"); err == nil {
		t.Fatal("opened a folder R3V doesn't know")
	}
}

func TestImportOpenWith(t *testing.T) {
	root := localSong(t)
	fakePrograms(t)
	other := anotherSong(t)
	dir := t.TempDir()
	exe := filepath.Join(dir, "Editor.exe")
	os.WriteFile(exe, nil, 0o755)
	gone := filepath.Join(dir, "Gone.exe")
	notes := filepath.Join(dir, "notes.txt")
	os.WriteFile(notes, nil, 0o644)
	a := &App{}
	if err := a.ImportOpenWith(map[string]KeptOpenWith{
		root:                          {Last: &LastProgram{ID: exe, Name: "Editor"}, Chosen: []string{exe, gone, notes, "Editor.exe", exe}},
		other:                         {Last: &LastProgram{ID: gone, Name: "Gone"}, Chosen: []string{gone}},
		filepath.Join(dir, "Unknown"): {Last: &LastProgram{ID: "vscode", Name: "VS Code"}},
	}); err != nil {
		t.Fatal(err)
	}
	if got := chosenIDs(a.OpenPrograms(root)); !slices.Equal(got, []string{exe}) {
		t.Fatalf("chosen %q", got)
	}
	if l := a.OpenedWith(root); l == nil || l.ID != exe {
		t.Fatalf("last: %+v", l)
	}
	if got := chosenIDs(a.OpenPrograms(other)); len(got) != 0 || a.OpenedWith(other) != nil {
		t.Fatalf("other: %q, %+v", got, a.OpenedWith(other))
	}
	if strings.Contains(read(t, openWithFile()), "Unknown") {
		t.Fatal("kept for a folder R3V doesn't know")
	}
	// a detected kind is kept as the last; what is kept here already wins
	if err := a.ImportOpenWith(map[string]KeptOpenWith{
		other: {Last: &LastProgram{ID: "vscode", Name: "VS Code"}},
		root:  {Last: &LastProgram{ID: "explorer", Name: "Explorer"}},
	}); err != nil {
		t.Fatal(err)
	}
	if l := a.OpenedWith(other); l == nil || l.ID != "vscode" {
		t.Fatalf("other's last: %+v", l)
	}
	if l := a.OpenedWith(root); l == nil || l.ID != exe {
		t.Fatalf("root's last replaced: %+v", l)
	}
}
