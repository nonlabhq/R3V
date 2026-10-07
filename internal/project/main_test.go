package project

import (
	"os"
	"testing"
)

// Tests must not touch the user's real team store.
func TestMain(m *testing.M) {
	if op := os.Getenv("R3V_KILL_OP"); op != "" {
		os.Exit(killedChild(op)) // a test's child process, to be killed (killed_test.go)
	}
	dir, err := os.MkdirTemp("", "r3v-teams-")
	if err != nil {
		panic(err)
	}
	os.Setenv("R3V_CONFIG_DIR", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
