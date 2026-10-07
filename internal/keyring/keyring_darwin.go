package keyring

import (
	"errors"
	"os/exec"
	"strings"
)

// macOS: the login Keychain, through the security tool. The secret goes on
// its standard input (interactive mode), never on a command line others
// could see.

// notFound is security's exit code for an item that isn't there.
const notFound = 44

func security(stdin string, args ...string) (string, error) {
	cmd := exec.Command("/usr/bin/security", args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.Output()
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == notFound {
		return "", ErrNotFound
	}
	return string(out), err
}

// quote makes s one argument of security's interactive mode.
func quote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

func set(target, user string, secret string) error {
	_, err := security("add-generic-password -U -s "+quote(target)+" -a "+quote(user)+" -w "+quote(secret)+"\n", "-i")
	if err == nil {
		fileDel(target) // (one kept in the file before)
	}
	return err
}

func get(target string) (string, error) {
	out, err := security("", "find-generic-password", "-s", target, "-w")
	if errors.Is(err, ErrNotFound) {
		return fileGet(target) // kept by an earlier R3V
	}
	return strings.TrimRight(out, "\n"), err
}

func del(target string) error {
	if _, err := security("", "delete-generic-password", "-s", target); err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	return fileDel(target)
}
