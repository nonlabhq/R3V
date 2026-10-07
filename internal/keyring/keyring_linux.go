package keyring

import (
	"errors"
	"os"
	"os/exec"
	"strings"
)

// Linux: the Secret Service (GNOME Keyring, KWallet) through secret-tool,
// the secret on its standard input. Without it (no desktop session, the
// tool not installed) the file is used.

const attr = "r3v-target"

func secretTool() bool {
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		return false
	}
	_, err := exec.LookPath("secret-tool")
	return err == nil
}

func set(target, user string, secret string) error {
	if secretTool() {
		cmd := exec.Command("secret-tool", "store", "--label=R3V "+user, attr, target)
		cmd.Stdin = strings.NewReader(secret)
		if cmd.Run() == nil {
			fileDel(target) // (one kept in the file before)
			return nil
		}
		// (the service can't be used now: the file, as without it)
	}
	return fileSet(target, secret)
}

func get(target string) (string, error) {
	if secretTool() {
		// lookup fails, without output, when there is no such secret
		if out, err := exec.Command("secret-tool", "lookup", attr, target).Output(); err == nil && len(out) > 0 {
			return strings.TrimRight(string(out), "\n"), nil
		}
	}
	return fileGet(target)
}

func del(target string) error {
	if secretTool() {
		var ee *exec.ExitError
		if err := exec.Command("secret-tool", "clear", attr, target).Run(); err != nil && !errors.As(err, &ee) {
			return err
		}
	}
	return fileDel(target)
}
