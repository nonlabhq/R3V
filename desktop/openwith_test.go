package desktop

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
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

func TestChosenProgramsStillThere(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "Editor.exe")
	os.WriteFile(exe, nil, 0o755)
	os.WriteFile(filepath.Join(dir, "notes.txt"), nil, 0o644)
	detected.Lock()
	detected.list = []Program{{ID: "explorer", Name: "Explorer"}} // not this computer's
	detected.Unlock()
	t.Cleanup(func() { detected.Lock(); detected.list = nil; detected.Unlock() })
	a := &App{}
	got := a.OpenPrograms([]string{exe, filepath.Join(dir, "Gone.exe"), filepath.Join(dir, "notes.txt"), "Editor.exe"})
	var chosen []Program
	for _, p := range got {
		if p.Chosen {
			chosen = append(chosen, p)
		}
	}
	if len(chosen) != 1 || chosen[0].ID != exe || chosen[0].Name != "Editor" {
		t.Fatalf("chosen: %+v", chosen)
	}
	if got[0].ID != "explorer" {
		t.Fatalf("Explorer first: %+v", got)
	}
}

func TestChooseProgram(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "Editor.exe")
	os.WriteFile(exe, nil, 0o755)
	a := &App{pickFile: func(string) (string, error) { return exe, nil }}
	if p, err := a.ChooseProgram("Choose"); err != nil || p != exe {
		t.Fatalf("chose %q, %v", p, err)
	}
	a.pickFile = func(string) (string, error) { return "", nil }
	if p, err := a.ChooseProgram("Choose"); err != nil || p != "" {
		t.Fatalf("cancelled: %q, %v", p, err)
	}
	a.pickFile = func(string) (string, error) { return filepath.Join(dir, "a.txt"), nil }
	if _, err := a.ChooseProgram("Choose"); err == nil {
		t.Fatal("a .txt taken for a program")
	}
}
