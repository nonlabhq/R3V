package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// systemLookup looks in the registry (App Paths, the uninstall entries),
// asks vswhere for Visual Studio, and checks a few known paths.
func systemLookup() lookup {
	return lookup{
		appPath:   regAppPath,
		uninstall: regUninstall,
		vswhere:   vswherePath,
		glob: func(p string) []string {
			m, _ := filepath.Glob(p)
			return m
		},
		exists: func(p string) bool {
			fi, err := os.Stat(p)
			return err == nil && !fi.IsDir()
		},
		getenv: os.Getenv,
		lookPath: func(f string) string {
			p, _ := exec.LookPath(f)
			return p
		},
	}
}

var regRoots = []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE}

func regAppPath(exe string) string {
	for _, root := range regRoots {
		k, err := registry.OpenKey(root, `Software\Microsoft\Windows\CurrentVersion\App Paths\`+exe, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		v, _, err := k.GetStringValue("")
		k.Close()
		if err == nil && v != "" {
			return strings.Trim(v, `"`)
		}
	}
	return ""
}

func regUninstall() []installed {
	var out []installed
	for _, root := range regRoots {
		for _, path := range []string{`Software\Microsoft\Windows\CurrentVersion\Uninstall`,
			`Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`} {
			k, err := registry.OpenKey(root, path, registry.ENUMERATE_SUB_KEYS)
			if err != nil {
				continue
			}
			names, _ := k.ReadSubKeyNames(-1)
			k.Close()
			for _, n := range names {
				sub, err := registry.OpenKey(root, path+`\`+n, registry.QUERY_VALUE)
				if err != nil {
					continue
				}
				var in installed
				in.name, _, _ = sub.GetStringValue("DisplayName")
				in.location, _, _ = sub.GetStringValue("InstallLocation")
				in.icon, _, _ = sub.GetStringValue("DisplayIcon")
				sub.Close()
				if in.name != "" {
					out = append(out, in)
				}
			}
		}
	}
	return out
}

// vswherePath asks Visual Studio's installer where the latest devenv.exe is.
func vswherePath() string {
	vswhere := filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft Visual Studio", "Installer", "vswhere.exe")
	if _, err := os.Stat(vswhere); err != nil {
		return ""
	}
	cmd := exec.Command(vswhere, "-latest", "-prerelease", "-property", "productPath", "-format", "value", "-utf8")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ownConsole gives a console program (PowerShell) a window of its own: R3V
// has no console to share.
func ownConsole(cmd *exec.Cmd, console bool) {
	if console {
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE}
	}
}
