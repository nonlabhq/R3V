package desktop

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/nonlabhq/r3v/internal/teams"
	"github.com/nonlabhq/r3v/internal/version"

	"golang.org/x/sys/windows/registry"
)

const (
	runKey = `Software\Microsoft\Windows\CurrentVersion\Run`
	// backgroundFlag starts R3V in the tray without opening the window
	// (used when Windows starts it at sign-in).
	backgroundFlag = "--background"
)

func autostartCommand() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return `"` + exe + `" ` + backgroundFlag, nil
}

// Autostart reports whether R3V starts when the user signs in to Windows.
func (a *App) Autostart() bool {
	if f := autostartFile(); f != "" {
		_, err := os.Stat(f)
		return err == nil
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(runValue())
	return err == nil
}

// SetAutostart turns starting R3V at sign-in on or off.
func (a *App) SetAutostart(on bool) error {
	if f := autostartFile(); f != "" {
		if !on {
			if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			return nil
		}
		return os.WriteFile(f, nil, 0o644)
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !on {
		if err := k.DeleteValue(runValue()); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return err
		}
		return nil
	}
	cmd, err := autostartCommand()
	if err != nil {
		return err
	}
	return k.SetStringValue(runValue(), cmd)
}

// runValue names the sign-in entry: the app's name, so a build with
// extensions ("R3V Pro") has its own next to the public app's.
func runValue() string { return version.Name() }

// autostartFile keeps the choice in the settings folder when R3V_CONFIG_DIR
// is set (tests, the demo): such a run neither shows this user's own choice
// nor has Windows start a test build at sign-in. "" otherwise.
func autostartFile() string {
	if os.Getenv("R3V_CONFIG_DIR") == "" {
		return ""
	}
	return filepath.Join(teams.Dir(), "autostart")
}
