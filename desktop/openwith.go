package desktop

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// Program is one the project folder can be opened with ("Open in" menu):
// Explorer, a terminal, an editor found on this computer, or a program the
// user chose for the project.
type Program struct {
	ID   string `json:"id"` // "explorer", "terminal", a codeEditors id, or Path for one chosen
	Name string `json:"name"`
	Path string `json:"path"` // its .exe ("" for Explorer)
	// Chosen: picked with "Choose a program…" (the page remembers it for the
	// project).
	Chosen bool `json:"chosen"`
}

// editor is a program "Open in" looks for: where it is usually installed,
// and whether it opens a solution rather than the folder when there is one.
type editor struct {
	id, name  string
	exe       string   // its file name (App Paths, an uninstall entry's icon)
	uninstall string   // an uninstall entry's DisplayName starts with it
	sub       string   // the exe under that entry's InstallLocation
	paths     []string // well-known places (%VAR% expanded; may be a glob)
	vswhere   bool     // Visual Studio's own installer knows where it is
	solution  bool     // opens a .sln in the project's root when there is exactly one
}

// codeEditors are the programs detected for now; a new one is a line here.
var codeEditors = []editor{
	{id: "vscode", name: "VS Code", exe: "Code.exe", uninstall: "Microsoft Visual Studio Code", sub: "Code.exe",
		paths: []string{`%LOCALAPPDATA%\Programs\Microsoft VS Code\Code.exe`, `%ProgramFiles%\Microsoft VS Code\Code.exe`}},
	{id: "cursor", name: "Cursor", exe: "Cursor.exe", uninstall: "Cursor", sub: "Cursor.exe",
		paths: []string{`%LOCALAPPDATA%\Programs\cursor\Cursor.exe`}},
	{id: "visualstudio", name: "Visual Studio", exe: "devenv.exe", uninstall: "Visual Studio", sub: `Common7\IDE\devenv.exe`,
		vswhere: true, solution: true},
	{id: "rider", name: "Rider", exe: "rider64.exe", uninstall: "JetBrains Rider", sub: `bin\rider64.exe`, solution: true,
		paths: []string{`%LOCALAPPDATA%\Programs\Rider\bin\rider64.exe`,
			`%LOCALAPPDATA%\JetBrains\Toolbox\apps\Rider\*\*\bin\rider64.exe`,
			`%ProgramFiles%\JetBrains\JetBrains Rider*\bin\rider64.exe`}},
}

// installed is an uninstall entry of the registry.
type installed struct{ name, location, icon string }

// lookup is where detection looks: the registry and a few known paths on
// Windows; faked in tests. Nothing in it walks the disk.
type lookup struct {
	appPath   func(exe string) string // App Paths' default value
	uninstall func() []installed
	vswhere   func() string // devenv.exe of the latest Visual Studio
	glob      func(pattern string) []string
	exists    func(path string) bool
	getenv    func(key string) string
	lookPath  func(file string) string
}

// expand replaces %VAR% in p; "" when one isn't set (no path relative to
// wherever R3V runs).
func (l lookup) expand(p string) string {
	parts := strings.Split(p, "%")
	if len(parts)%2 == 0 {
		return p
	}
	for i := 1; i < len(parts); i += 2 {
		if parts[i] = l.getenv(parts[i]); parts[i] == "" {
			return ""
		}
	}
	return strings.Join(parts, "")
}

// find is the first existing path of patterns (of a glob, the last by name:
// usually the newest version).
func (l lookup) find(patterns ...string) string {
	for _, p := range patterns {
		if p = l.expand(p); p == "" {
			continue
		}
		if strings.ContainsAny(p, "*?[") {
			if m := l.glob(p); len(m) > 0 {
				return m[len(m)-1]
			}
			continue
		}
		if l.exists(p) {
			return p
		}
	}
	return ""
}

// locate finds e on this computer ("" when it isn't).
func (l lookup) locate(e editor, entries []installed) string {
	if e.vswhere && l.vswhere != nil {
		if p := l.vswhere(); p != "" && l.exists(p) {
			return p
		}
	}
	if l.appPath != nil {
		if p := l.appPath(e.exe); p != "" && l.exists(p) {
			return p
		}
	}
	for _, in := range entries {
		if !strings.HasPrefix(in.name, e.uninstall) {
			continue
		}
		if in.location != "" {
			if p := filepath.Join(in.location, e.sub); l.exists(p) {
				return p
			}
		}
		// DisplayIcon is `"C:\…\Code.exe",0` or the like.
		icon := strings.Trim(in.icon, `"`)
		if i := strings.LastIndex(icon, ","); i > 0 && !strings.HasSuffix(strings.ToLower(icon), ".exe") {
			icon = strings.Trim(icon[:i], `"`)
		}
		if strings.EqualFold(filepath.Base(icon), e.exe) && l.exists(icon) {
			return icon
		}
	}
	return l.find(e.paths...)
}

// terminal is Windows Terminal when installed, else PowerShell.
func (l lookup) terminal() Program {
	wt := ""
	if l.appPath != nil {
		wt = l.appPath("wt.exe")
	}
	if wt == "" || !l.exists(wt) {
		wt = l.find(`%LOCALAPPDATA%\Microsoft\WindowsApps\wt.exe`)
	}
	if wt == "" {
		wt = l.lookPath("wt.exe")
	}
	if wt != "" {
		return Program{ID: "terminal", Name: "Terminal", Path: wt}
	}
	ps := l.find(`%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe`)
	if ps == "" {
		ps = l.lookPath("powershell.exe")
	}
	if ps == "" {
		return Program{}
	}
	return Program{ID: "terminal", Name: "PowerShell", Path: ps}
}

// detect lists Explorer, the terminal and the editors found.
func detect(l lookup) []Program {
	out := []Program{{ID: "explorer", Name: "Explorer"}}
	if t := l.terminal(); t.ID != "" {
		out = append(out, t)
	}
	var entries []installed
	if l.uninstall != nil {
		entries = l.uninstall()
	}
	for _, e := range codeEditors {
		if p := l.locate(e, entries); p != "" {
			out = append(out, Program{ID: e.id, Name: e.name, Path: p})
		}
	}
	return out
}

// detected is detect's answer for the app's run: programs are rarely
// installed while it runs, and the registry is read once.
var detected struct {
	sync.Mutex
	list []Program
}

func detectedPrograms() []Program {
	detected.Lock()
	defer detected.Unlock()
	if detected.list == nil {
		detected.list = detect(systemLookup())
	}
	return detected.list
}

// chosenProgram is an .exe the user picked, when it is still there.
func chosenProgram(path string) (Program, bool) {
	if !filepath.IsAbs(path) || !strings.EqualFold(filepath.Ext(path), ".exe") {
		return Program{}, false
	}
	if fi, err := os.Stat(path); err != nil || fi.IsDir() {
		return Program{}, false
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return Program{ID: path, Name: name, Path: path, Chosen: true}, true
}

// OpenPrograms lists what the project folder can be opened with: Explorer,
// a terminal, the editors on this computer, then the programs chosen for
// the project (chosen: their paths, as the page keeps them) that still
// exist. The editors are looked for the first time only (the menu opened).
func (a *App) OpenPrograms(chosen []string) []Program {
	out := append([]Program(nil), detectedPrograms()...)
	for _, c := range chosen {
		if p, ok := chosenProgram(c); ok {
			out = append(out, p)
		}
	}
	return out
}

// ChooseProgram asks for a program (.exe) to open projects with ("" when
// cancelled).
func (a *App) ChooseProgram(title string) (string, error) {
	if a.pickFile == nil {
		return "", errors.New("file picker not available")
	}
	path, err := a.pickFile(title)
	if err != nil || path == "" {
		return "", err
	}
	if _, ok := chosenProgram(path); !ok {
		return "", errors.New("that isn't a program (.exe)")
	}
	return path, nil
}

// launch is how a program is started on a project: Exe with Args (never a
// shell's command line), in Dir. No Exe: the folder opens in Explorer.
type launch struct {
	Exe     string
	Args    []string
	Dir     string
	Console bool // a program with a console window of its own (PowerShell)
}

// solution is the project root's one .sln ("" with none or several).
func solution(root string) string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	found := ""
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".sln") {
			if found != "" {
				return ""
			}
			found = filepath.Join(root, e.Name())
		}
	}
	return found
}

// launchFor is how p opens the project in root.
func launchFor(p Program, root string) launch {
	switch {
	case p.ID == "explorer":
		return launch{Dir: root}
	case p.ID == "terminal" && strings.EqualFold(filepath.Base(p.Path), "wt.exe"):
		return launch{Exe: p.Path, Args: []string{"-d", root}, Dir: root}
	case p.ID == "terminal":
		return launch{Exe: p.Path, Args: []string{"-NoExit"}, Dir: root, Console: true}
	}
	target := root
	for _, e := range codeEditors {
		if e.id == p.ID && e.solution {
			if sln := solution(root); sln != "" {
				target = sln
			}
		}
	}
	return launch{Exe: p.Path, Args: []string{target}, Dir: root}
}

// OpenFolderWith opens the project folder with a program of OpenPrograms
// (its id; a chosen one by its path).
func (a *App) OpenFolderWith(root, id string) error {
	if !knownProject(root) {
		return errors.New("unknown project")
	}
	var p Program
	if c, ok := chosenProgram(id); ok {
		p = c
	} else {
		for _, d := range detectedPrograms() {
			if d.ID == id {
				p = d
			}
		}
	}
	if p.ID == "" {
		return errors.New("that program isn't on this computer any more")
	}
	l := launchFor(p, root)
	if l.Exe == "" {
		return shellOpen(l.Dir)
	}
	cmd := exec.Command(l.Exe, l.Args...)
	cmd.Dir = l.Dir
	ownConsole(cmd, l.Console)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
